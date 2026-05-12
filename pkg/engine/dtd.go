package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DTDDecl represents a single declaration found in a DTD file.
type DTDDecl struct {
	Kind    string `json:"kind"`    // "element", "attlist", "general-entity", "parameter-entity", "notation"
	Name    string `json:"name"`
	Value   string `json:"value"`
	Type    string `json:"type"`    // for elements: content model; for attrs: type; for entities: SYSTEM/PUBLIC
	Default string `json:"default"` // for attributes: #REQUIRED, #IMPLIED, #FIXED, default value
}

// DTDFinding represents a single finding from DTD audit.
type DTDFinding struct {
	Severity   string `json:"severity"`   // "error", "warning", "info"
	Category   string `json:"category"`   // "entity-expansion", "external-entity", "parameter-complexity", "missing-decl"
	Name       string `json:"name"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
}

// ParseDTD reads and parses a DTD file, extracting all declarations.
// This uses a simple regex-based parser since DTDs are not well-formed XML.
func ParseDTD(path string) ([]DTDDecl, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading DTD: %w", err)
	}

	content := string(data)
	var decls []DTDDecl

	// Remove comments
	commentRe := regexp.MustCompile(`<!--.*?-->`)
	content = commentRe.ReplaceAllString(content, "")

	// Parse ENTITY declarations
	// General entities: <!ENTITY name "value"> or <!ENTITY name SYSTEM "uri">
	entityRe := regexp.MustCompile(`(?s)<!ENTITY\s+(\S+)\s+(.*?)>`)
	for _, match := range entityRe.FindAllStringSubmatch(content, -1) {
		name := match[1]
		value := strings.TrimSpace(match[2])
		value = strings.ReplaceAll(value, "\n", " ")
		value = regexp.MustCompile(`\s+`).ReplaceAllString(value, " ")

		kind := "general-entity"
		if strings.HasPrefix(name, "%") {
			kind = "parameter-entity"
			name = strings.TrimPrefix(name, "%")
		}

		entityType := "internal"
		if strings.HasPrefix(value, "SYSTEM") || strings.HasPrefix(value, "PUBLIC") {
			entityType = "external"
		}

		decls = append(decls, DTDDecl{
			Kind:  kind,
			Name:  name,
			Value: value,
			Type:  entityType,
		})
	}

	// Parse ELEMENT declarations
	elemRe := regexp.MustCompile(`<!ELEMENT\s+(\S+)\s+(.*?)>`)
	for _, match := range elemRe.FindAllStringSubmatch(content, -1) {
		name := match[1]
		contentModel := strings.TrimSpace(match[2])
		decls = append(decls, DTDDecl{
			Kind:  "element",
			Name:  name,
			Value: contentModel,
		})
	}

	// Parse ATTLIST declarations
	attlistRe := regexp.MustCompile(`(?s)<!ATTLIST\s+(\S+)\s+(.*?)>`)
	for _, match := range attlistRe.FindAllStringSubmatch(content, -1) {
		elemName := match[1]
		attrs := strings.TrimSpace(match[2])
		// Normalize newlines within attribute list
		attrs = strings.ReplaceAll(attrs, "\n", " ")
		attrs = regexp.MustCompile(`\s+`).ReplaceAllString(attrs, " ")
		decls = append(decls, DTDDecl{
			Kind:    "attlist",
			Name:    elemName,
			Value:   attrs,
			Default: "",
		})
	}

	// Parse NOTATION declarations
	notationRe := regexp.MustCompile(`<!NOTATION\s+(\S+)\s+(.*?)>`)
	for _, match := range notationRe.FindAllStringSubmatch(content, -1) {
		name := match[1]
		value := strings.TrimSpace(match[2])
		decls = append(decls, DTDDecl{
			Kind:  "notation",
			Name:  name,
			Value: value,
		})
	}

	return decls, nil
}

// FlattenDTD reads a DTD file and resolves all external parameter entity
// references (%pe;) into a single monolithic output.
func FlattenDTD(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading DTD: %w", err)
	}

	content := string(data)
	dir := filepath.Dir(path)

	// Remove comments
	commentRe := regexp.MustCompile(`<!--.*?-->`)
	content = commentRe.ReplaceAllString(content, "")

	// Resolve external parameter entity references
	// Pattern: <!ENTITY % name SYSTEM "file"> or <!ENTITY % name PUBLIC "..." "file">
	peRe := regexp.MustCompile(`<!ENTITY\s+%\s+(\S+)\s+(?:SYSTEM|PUBLIC\s+[^"]*")\s+"([^"]+)"\s*>`)
	for _, match := range peRe.FindAllStringSubmatch(content, -1) {
		peName := match[1]
		peFile := match[2]

		// Read the referenced file
		resolvedPath := filepath.Join(dir, peFile)
		incData, err := os.ReadFile(resolvedPath)
		if err != nil {
			// Skip files we can't read
			continue
		}

		// Replace %peName; references with the included content
		refPattern := regexp.MustCompile(`%` + regexp.QuoteMeta(peName) + `;`)
		content = refPattern.ReplaceAllString(content, string(incData))
	}

	return content, nil
}

// AuditDTD performs a comprehensive health check on a DTD file.
func AuditDTD(path string) ([]DTDFinding, error) {
	decls, err := ParseDTD(path)
	if err != nil {
		return nil, err
	}

	var findings []DTDFinding

	// Check for dangerous entity expansion (billion laughs)
	entityRefCount := map[string]int{}
	for _, decl := range decls {
		if decl.Kind == "general-entity" || decl.Kind == "parameter-entity" {
			refs := countEntityRefs(decl.Value)
			entityRefCount[decl.Name] = refs
		}
	}

	// Flag entities with many references
	for name, count := range entityRefCount {
		if count > 5 {
			findings = append(findings, DTDFinding{
				Severity:   "error",
				Category:   "entity-expansion",
				Name:       name,
				Message:    fmt.Sprintf("Entity %s contains %d entity references (potential billion laughs vector)", name, count),
				Suggestion: "Reduce entity nesting depth or disable external entity processing",
			})
		} else if count > 2 {
			findings = append(findings, DTDFinding{
				Severity:   "warning",
				Category:   "entity-expansion",
				Name:       name,
				Message:    fmt.Sprintf("Entity %s contains %d entity references", name, count),
				Suggestion: "Consider simplifying entity references",
			})
		}
	}

	// Check for external entities
	for _, decl := range decls {
		if (decl.Kind == "general-entity" || decl.Kind == "parameter-entity") && decl.Type == "external" {
			findings = append(findings, DTDFinding{
				Severity:   "warning",
				Category:   "external-entity",
				Name:       decl.Name,
				Message:    fmt.Sprintf("External %s: %s", decl.Kind, decl.Value),
				Suggestion: "Ensure external entity resolution is disabled or restricted in production",
			})
		}
	}

	// Check for elements without attribute declarations
	elemNames := map[string]bool{}
	attlistElems := map[string]bool{}
	for _, decl := range decls {
		if decl.Kind == "element" {
			elemNames[decl.Name] = true
		}
		if decl.Kind == "attlist" {
			attlistElems[decl.Name] = true
		}
	}
	for name := range elemNames {
		if !attlistElems[name] {
			findings = append(findings, DTDFinding{
				Severity:   "info",
				Category:   "missing-decl",
				Name:       name,
				Message:    fmt.Sprintf("Element %s has no ATTLIST declaration", name),
				Suggestion: "Consider whether this element needs attributes",
			})
		}
	}

	// Check for attribute declarations without matching elements
	for name := range attlistElems {
		if !elemNames[name] {
			findings = append(findings, DTDFinding{
				Severity:   "warning",
				Category:   "missing-decl",
				Name:       name,
				Message:    fmt.Sprintf("ATTLIST for undefined element %s", name),
				Suggestion: "Add an ELEMENT declaration for this name or remove the ATTLIST",
			})
		}
	}

	return findings, nil
}

func countEntityRefs(value string) int {
	re := regexp.MustCompile(`&[^;]+;|%[^;]+;`)
	return len(re.FindAllString(value, -1))
}
