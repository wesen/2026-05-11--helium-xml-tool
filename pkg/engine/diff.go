package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/xsd"
)

// SchemaDiff represents the result of comparing two XSD schemas.
type SchemaDiff struct {
	OldFile string        `json:"old-file"`
	NewFile string        `json:"new-file"`
	Changes []DiffChange  `json:"changes"`
	Summary DiffSummary   `json:"summary"`
}

// DiffChange represents a single difference between two schema versions.
type DiffChange struct {
	Category   string `json:"category"`   // "type-added", "type-removed", "type-changed", "element-added", "element-removed", "attr-added", "attr-removed", "attr-changed"
	Severity   string `json:"severity"`   // "breaking", "safe", "warning"
	Component  string `json:"component"`  // type/element name
	Detail     string `json:"detail"`
	OldValue   string `json:"old-value,omitempty"`
	NewValue   string `json:"new-value,omitempty"`
}

// DiffSummary counts changes by category and severity.
type DiffSummary struct {
	TotalChanges   int `json:"total-changes"`
	BreakingCount  int `json:"breaking-count"`
	SafeCount      int `json:"safe-count"`
	WarningCount   int `json:"warning-count"`
	TypesAdded     int `json:"types-added"`
	TypesRemoved   int `json:"types-removed"`
	TypesChanged   int `json:"types-changed"`
	ElemsAdded     int `json:"elements-added"`
	ElemsRemoved   int `json:"elements-removed"`
	AttrsChanged   int `json:"attrs-changed"`
}

// DiffSchemas performs a semantic comparison of two XSD schemas.
func DiffSchemas(ctx context.Context, oldPath, newPath string) (*SchemaDiff, error) {
	oldSchema, err := CompileSchema(ctx, oldPath)
	if err != nil {
		return nil, fmt.Errorf("compiling old schema: %w", err)
	}
	newSchema, err := CompileSchema(ctx, newPath)
	if err != nil {
		return nil, fmt.Errorf("compiling new schema: %w", err)
	}

	diff := &SchemaDiff{
		OldFile: oldPath,
		NewFile: newPath,
	}

	oldTypes := buildTypeMap(oldSchema)
	newTypes := buildTypeMap(newSchema)

	// Types removed (breaking)
	xsdNS := "http://www.w3.org/2001/XMLSchema"
	for name := range oldTypes {
		// Skip XSD built-in types in diff
		if strings.Contains(name, xsdNS) {
			continue
		}
		if _, ok := newTypes[name]; !ok {
			diff.Changes = append(diff.Changes, DiffChange{
				Category:  "type-removed",
				Severity:  "breaking",
				Component: name,
				Detail:    fmt.Sprintf("Type %s was removed", name),
			})
		}
	}

	// Types added (safe)
	for name := range newTypes {
		if strings.Contains(name, xsdNS) {
			continue
		}
		if _, ok := oldTypes[name]; !ok {
			diff.Changes = append(diff.Changes, DiffChange{
				Category:  "type-added",
				Severity:  "safe",
				Component: name,
				Detail:    fmt.Sprintf("Type %s was added", name),
			})
		}
	}

	// Types that exist in both — compare
	for name, oldTD := range oldTypes {
		if strings.Contains(name, xsdNS) {
			continue
		}
		newTD, ok := newTypes[name]
		if !ok {
			continue
		}
		_ = oldTD
		compareTypes(name, oldTD, newTD, diff)
	}

	// Build summary
	for _, c := range diff.Changes {
		diff.Summary.TotalChanges++
		switch c.Severity {
		case "breaking":
			diff.Summary.BreakingCount++
		case "safe":
			diff.Summary.SafeCount++
		case "warning":
			diff.Summary.WarningCount++
		}
		switch c.Category {
		case "type-added":
			diff.Summary.TypesAdded++
		case "type-removed":
			diff.Summary.TypesRemoved++
		case "type-changed":
			diff.Summary.TypesChanged++
		case "element-added":
			diff.Summary.ElemsAdded++
		case "element-removed":
			diff.Summary.ElemsRemoved++
		case "attr-changed", "attr-added", "attr-removed":
			diff.Summary.AttrsChanged++
		}
	}

	return diff, nil
}

func buildTypeMap(schema *xsd.Schema) map[string]*xsd.TypeDef {
	m := map[string]*xsd.TypeDef{}
	for _, qn := range schema.NamedTypes() {
		td, _ := schema.LookupType(qn.Local, qn.NS)
		if td != nil {
			m[qnameStr(qn)] = td
		}
	}
	return m
}

func compareTypes(name string, old, new *xsd.TypeDef, diff *SchemaDiff) {
	// Content type change
	if old.ContentType != new.ContentType {
		severity := "warning"
		detail := fmt.Sprintf("Content type changed from %s to %s",
			contentTypeStr(old.ContentType), contentTypeStr(new.ContentType))
		if old.ContentType == xsd.ContentTypeEmpty && new.ContentType != xsd.ContentTypeEmpty {
			severity = "safe" // widening
		}
		if new.ContentType == xsd.ContentTypeEmpty && old.ContentType != xsd.ContentTypeEmpty {
			severity = "breaking" // narrowing to empty
		}
		diff.Changes = append(diff.Changes, DiffChange{
			Category:  "type-changed",
			Severity:  severity,
			Component: name,
			Detail:    detail,
			OldValue:  contentTypeStr(old.ContentType),
			NewValue:  contentTypeStr(new.ContentType),
		})
	}

	// Base type change
	if old.BaseType != nil && new.BaseType != nil {
		if qnameStr(old.BaseType.Name) != qnameStr(new.BaseType.Name) {
			diff.Changes = append(diff.Changes, DiffChange{
				Category:  "type-changed",
				Severity:  "warning",
				Component: name,
				Detail:    fmt.Sprintf("Base type changed from %s to %s", qnameStr(old.BaseType.Name), qnameStr(new.BaseType.Name)),
				OldValue:  qnameStr(old.BaseType.Name),
				NewValue:  qnameStr(new.BaseType.Name),
			})
		}
	}

	// Derivation change
	if old.Derivation != new.Derivation {
		diff.Changes = append(diff.Changes, DiffChange{
			Category:  "type-changed",
			Severity:  "warning",
			Component: name,
			Detail:    fmt.Sprintf("Derivation changed from %s to %s", derivationStr(old.Derivation), derivationStr(new.Derivation)),
			OldValue:  derivationStr(old.Derivation),
			NewValue:  derivationStr(new.Derivation),
		})
	}

	// Attribute changes
	oldAttrs := buildAttrMap(old.Attributes)
	newAttrs := buildAttrMap(new.Attributes)

	// Build sets of attributes that come from base types (inherited)
	// to avoid false positives when comparing types that inherit attributes
	oldInherited := buildInheritedAttrs(old)
	newInherited := buildInheritedAttrs(new)

	for attrName := range oldAttrs {
		if oldInherited[attrName] && newInherited[attrName] {
			continue // both inherit it, not a change
		}
		if _, ok := newAttrs[attrName]; !ok {
			diff.Changes = append(diff.Changes, DiffChange{
				Category:  "attr-removed",
				Severity:  "breaking",
				Component: name,
				Detail:    fmt.Sprintf("Attribute %s removed from type %s", attrName, name),
			})
		}
	}
	for attrName := range newAttrs {
		if oldInherited[attrName] && newInherited[attrName] {
			continue
		}
		if _, ok := oldAttrs[attrName]; !ok {
			// New optional attribute = safe, new required attribute = breaking
			severity := "safe"
			if newAttrs[attrName].Required {
				severity = "breaking"
			}
			diff.Changes = append(diff.Changes, DiffChange{
				Category:  "attr-added",
				Severity:  severity,
				Component: name,
				Detail:    fmt.Sprintf("Attribute %s added to type %s (required=%v)", attrName, name, newAttrs[attrName].Required),
			})
		}
	}

	// Compare enumerations (if simple type with facets)
	if old.Facets != nil && new.Facets != nil {
		compareFacets(name, old.Facets, new.Facets, diff)
	}

	// Abstract change
	if old.Abstract != new.Abstract {
		severity := "warning"
		if old.Abstract && !new.Abstract {
			severity = "safe" // making concrete is widening
		}
		if !old.Abstract && new.Abstract {
			severity = "breaking" // making abstract is narrowing
		}
		diff.Changes = append(diff.Changes, DiffChange{
			Category:  "type-changed",
			Severity:  severity,
			Component: name,
			Detail:    fmt.Sprintf("Abstract flag changed from %v to %v", old.Abstract, new.Abstract),
			OldValue:  fmt.Sprintf("%v", old.Abstract),
			NewValue:  fmt.Sprintf("%v", new.Abstract),
		})
	}
}

func compareFacets(typeName string, old, new *xsd.FacetSet, diff *SchemaDiff) {
	// Enumeration values removed = breaking, added = safe
	oldEnums := map[string]bool{}
	for _, v := range old.Enumeration {
		oldEnums[v] = true
	}
	newEnums := map[string]bool{}
	for _, v := range new.Enumeration {
		newEnums[v] = true
	}
	for v := range oldEnums {
		if !newEnums[v] {
			diff.Changes = append(diff.Changes, DiffChange{
				Category:  "type-changed",
				Severity:  "breaking",
				Component: typeName,
				Detail:    fmt.Sprintf("Enumeration value %q removed", v),
				OldValue:  v,
			})
		}
	}
	for v := range newEnums {
		if !oldEnums[v] {
			diff.Changes = append(diff.Changes, DiffChange{
				Category:  "type-changed",
				Severity:  "safe",
				Component: typeName,
				Detail:    fmt.Sprintf("Enumeration value %q added", v),
				NewValue:  v,
			})
		}
	}

	// Pattern change
	oldPattern := ""
	newPattern := ""
	if old.Pattern != nil {
		oldPattern = *old.Pattern
	}
	if new.Pattern != nil {
		newPattern = *new.Pattern
	}
	if oldPattern != newPattern {
		diff.Changes = append(diff.Changes, DiffChange{
			Category:  "type-changed",
			Severity:  "warning",
			Component: typeName,
			Detail:    fmt.Sprintf("Pattern facet changed"),
			OldValue:  oldPattern,
			NewValue:  newPattern,
		})
	}

	// Length constraints tightened = breaking
	if new.MinLength != nil && (old.MinLength == nil || *new.MinLength > *old.MinLength) {
		diff.Changes = append(diff.Changes, DiffChange{
			Category:  "type-changed",
			Severity:  "breaking",
			Component: typeName,
			Detail:    "MinLength facet tightened",
		})
	}
	if new.MaxLength != nil && (old.MaxLength == nil || *new.MaxLength < *old.MaxLength) {
		diff.Changes = append(diff.Changes, DiffChange{
			Category:  "type-changed",
			Severity:  "breaking",
			Component: typeName,
			Detail:    "MaxLength facet tightened",
		})
	}
}

func buildAttrMap(attrs []*xsd.AttrUse) map[string]*xsd.AttrUse {
	m := map[string]*xsd.AttrUse{}
	for _, a := range attrs {
		m[qnameStr(a.Name)] = a
	}
	return m
}

// buildInheritedAttrs returns a set of attribute names that are inherited
// from base types (not declared directly on this type).
func buildInheritedAttrs(td *xsd.TypeDef) map[string]bool {
	inherited := map[string]bool{}
	if td.BaseType == nil {
		return inherited
	}
	// Walk up the base type chain
	for base := td.BaseType; base != nil; base = base.BaseType {
		for _, attr := range base.Attributes {
			inherited[qnameStr(attr.Name)] = true
		}
	}
	return inherited
}

func contentTypeStr(ct xsd.ContentTypeKind) string {
	switch ct {
	case xsd.ContentTypeEmpty:
		return "empty"
	case xsd.ContentTypeSimple:
		return "simple"
	case xsd.ContentTypeElementOnly:
		return "element-only"
	case xsd.ContentTypeMixed:
		return "mixed"
	default:
		return "unknown"
	}
}

func derivationStr(d xsd.DerivationKind) string {
	switch d {
	case xsd.DerivationExtension:
		return "extension"
	case xsd.DerivationRestriction:
		return "restriction"
	default:
		return "none"
	}
}

// BreakageResult holds the result of a breakage analysis.
type BreakageResult struct {
	OldSchema       string             `json:"old-schema"`
	NewSchema       string             `json:"new-schema"`
	CorpusFiles     int                `json:"corpus-files"`
	BreakingChanges []BreakingChange   `json:"breaking-changes"`
	SafeChanges     []SafeChange       `json:"safe-changes"`
	Summary         BreakageSummary    `json:"summary"`
}

// BreakingChange documents a schema change that breaks existing documents.
type BreakingChange struct {
	Change     DiffChange `json:"change"`
	AffectedFiles []string `json:"affected-files"`
	AffectedCount int      `json:"affected-count"`
}

// SafeChange documents a schema change that does not break existing documents.
type SafeChange struct {
	Change DiffChange `json:"change"`
}

// BreakageSummary counts breakage results.
type BreakageSummary struct {
	TotalBreaking     int `json:"total-breaking"`
	TotalSafe         int `json:"total-safe"`
	AffectedDocuments int `json:"affected-documents"`
}

// AnalyzeBreakage compares two schemas by validating a corpus of documents
// against the new schema and classifying changes.
func AnalyzeBreakage(ctx context.Context, oldPath, newPath string, corpusPaths []string, noNetwork bool) (*BreakageResult, error) {
	// First, compute the diff
	diff, err := DiffSchemas(ctx, oldPath, newPath)
	if err != nil {
		return nil, err
	}

	result := &BreakageResult{
		OldSchema:   oldPath,
		NewSchema:   newPath,
	}

	// Collect corpus files
	var files []string
	for _, path := range corpusPaths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".xml") {
					files = append(files, p)
				}
				return nil
			})
		} else {
			files = append(files, path)
		}
	}
	result.CorpusFiles = len(files)

	if len(files) == 0 {
		// No corpus: classify changes by severity from diff alone
		for _, c := range diff.Changes {
			if c.Severity == "breaking" {
				result.BreakingChanges = append(result.BreakingChanges, BreakingChange{
					Change:        c,
					AffectedCount: -1, // unknown without corpus
				})
			} else {
				result.SafeChanges = append(result.SafeChanges, SafeChange{Change: c})
			}
		}
	} else {
		// Validate corpus against new schema
		newSchema, err := CompileSchema(ctx, newPath)
		if err != nil {
			return nil, fmt.Errorf("compiling new schema: %w", err)
		}

		affectedFiles := map[string]bool{}
		for _, file := range files {
			buf, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			parser := helium.NewParser()
			if noNetwork {
				parser = parser.AllowNetwork(false)
			}
			doc, err := parser.Parse(ctx, buf)
			if err != nil {
				continue // skip malformed documents
			}
			if doc == nil {
				continue
			}

			collector := &resultCollector{
				file:       file,
				schemaType: "xsd",
				schemaFile: newPath,
			}
			_ = xsd.NewValidator(newSchema).ErrorHandler(collector).Validate(ctx, doc)
			if len(collector.results) > 0 {
				affectedFiles[file] = true
			}
		}

		// Classify changes
		affectedList := make([]string, 0, len(affectedFiles))
		for f := range affectedFiles {
			affectedList = append(affectedList, f)
		}
		sortStrings(affectedList)

		for _, c := range diff.Changes {
			if c.Severity == "breaking" {
				bc := BreakingChange{
					Change:         c,
					AffectedCount:  len(affectedFiles),
					AffectedFiles:  affectedList,
				}
				result.BreakingChanges = append(result.BreakingChanges, bc)
			} else {
				result.SafeChanges = append(result.SafeChanges, SafeChange{Change: c})
			}
		}
	}

	result.Summary.TotalBreaking = len(result.BreakingChanges)
	result.Summary.TotalSafe = len(result.SafeChanges)
	result.Summary.AffectedDocuments = len(files)

	return result, nil
}

func sortStrings(s []string) {
	sort := func(a, b string) int {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	}
	_ = sort
	// Simple insertion sort for small slices
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
