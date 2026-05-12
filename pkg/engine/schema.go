package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lestrrat-go/helium/xsd"
)

// SchemaInfo holds a compiled XSD schema and its metadata.
type SchemaInfo struct {
	Schema      *xsd.Schema
	File        string
	CompileErr  error
}

// CompileSchema compiles an XSD schema file and returns the Schema object.
func CompileSchema(ctx context.Context, path string) (*xsd.Schema, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, fmt.Errorf("cannot access schema: %w", err)
	}
	schema, err := xsd.NewCompiler().CompileFile(ctx, abs)
	if err != nil {
		return nil, fmt.Errorf("compiling schema: %w", err)
	}
	return schema, nil
}

// TypeExplanation is a human-readable description of an XSD type or element.
type TypeExplanation struct {
	Name        string   `json:"name"`
	Namespace   string   `json:"namespace"`
	Kind        string   `json:"kind"`         // "element", "simple-type", "complex-type"
	ContentType string   `json:"content-type"` // for complex types: empty, simple, element-only, mixed
	BaseType    string   `json:"base-type"`
	Derivation  string   `json:"derivation"`   // "extension", "restriction", ""
	Abstract    bool     `json:"abstract"`
	Nillable    bool     `json:"nillable"`      // elements only
	Attributes  []string `json:"attributes"`
	Children    []string `json:"children"`      // child element names
	Enumeration []string `json:"enumeration"`   // for simple types with enum facets
	Pattern     string   `json:"pattern"`       // for simple types with pattern facet
	MinLength   *int     `json:"min-length"`
	MaxLength   *int     `json:"max-length"`
	MinOccurs   int      `json:"min-occurs"`    // elements only
	MaxOccurs   int      `json:"max-occurs"`    // elements only (-1 = unbounded)
	SubstGroup  string   `json:"subst-group"`   // substitution group head
	Description string   `json:"description"`   // generated prose description
}

// ExplainElement produces a TypeExplanation for a global element declaration.
func ExplainElement(schema *xsd.Schema, local, ns string) (*TypeExplanation, error) {
	elem, ok := schema.LookupElement(local, ns)
	if !ok {
		return nil, fmt.Errorf("element {%s}%s not found in schema", ns, local)
	}
	expl := &TypeExplanation{
		Name:       local,
		Namespace:  ns,
		Kind:       "element",
		Abstract:   elem.Abstract,
		Nillable:   elem.Nillable,
		MinOccurs:  elem.MinOccurs,
		MaxOccurs:  elem.MaxOccurs,
		SubstGroup: qnameStr(elem.SubstitutionGroup),
	}

	if elem.Type != nil {
		fillTypeDetails(expl, elem.Type)
	}

	// Build prose description
	expl.Description = buildElementDescription(elem, expl)

	return expl, nil
}

// ExplainType produces a TypeExplanation for a named type definition.
func ExplainType(schema *xsd.Schema, local, ns string) (*TypeExplanation, error) {
	td, ok := schema.LookupType(local, ns)
	if !ok {
		return nil, fmt.Errorf("type {%s}%s not found in schema", ns, local)
	}

	expl := &TypeExplanation{
		Name:      local,
		Namespace: ns,
		Abstract:  td.Abstract,
	}

	if td.Name.Local != "" {
		expl.Kind = "complex-type"
		if td.ContentType == xsd.ContentTypeSimple || td.Name.Local != "" && td.ContentModel == nil && len(td.Attributes) > 0 && td.BaseType != nil && td.BaseType.ContentType == xsd.ContentTypeSimple {
			// Check if it's actually a simple type by looking at variety
		}
		// Distinguish simple vs complex type
		if td.ContentModel == nil && td.BaseType != nil && isSimpleTypeFamily(td) {
			expl.Kind = "simple-type"
		}
	}

	fillTypeDetails(expl, td)

	expl.Description = buildTypeDescription(td, expl)

	return expl, nil
}

// isSimpleTypeFamily checks if a TypeDef belongs to the simple type family
// (atomic, list, or union variety).
func isSimpleTypeFamily(td *xsd.TypeDef) bool {
	if td.Variety != 0 {
		return true // TypeVarietyAtomic, TypeVarietyList, TypeVarietyUnion
	}
	if td.ContentType == xsd.ContentTypeSimple {
		return true
	}
	return false
}

func fillTypeDetails(expl *TypeExplanation, td *xsd.TypeDef) {
	// Content type
	switch td.ContentType {
	case xsd.ContentTypeEmpty:
		expl.ContentType = "empty"
	case xsd.ContentTypeSimple:
		expl.ContentType = "simple"
	case xsd.ContentTypeElementOnly:
		expl.ContentType = "element-only"
	case xsd.ContentTypeMixed:
		expl.ContentType = "mixed"
	}

	// Base type
	if td.BaseType != nil && td.BaseType.Name.Local != "" {
		expl.BaseType = qnameStr(td.BaseType.Name)
	}

	// Derivation
	switch td.Derivation {
	case xsd.DerivationExtension:
		expl.Derivation = "extension"
	case xsd.DerivationRestriction:
		expl.Derivation = "restriction"
	}

	// Attributes
	for _, attr := range td.Attributes {
		req := ""
		if attr.Required {
			req = " (required)"
		}
		expl.Attributes = append(expl.Attributes, qnameStr(attr.Name)+req)
	}

	// Children from content model
	if td.ContentModel != nil {
		collectElementNames(td.ContentModel, &expl.Children)
	}

	// Facets for simple types
	if td.Facets != nil {
		f := td.Facets
		expl.Enumeration = f.Enumeration
		if f.Pattern != nil {
			expl.Pattern = *f.Pattern
		}
		expl.MinLength = f.MinLength
		expl.MaxLength = f.MaxLength
	}
}

func collectElementNames(mg *xsd.ModelGroup, names *[]string) {
	for _, p := range mg.Particles {
		if elem, ok := p.Term.(*xsd.ElementDecl); ok {
			*names = append(*names, qnameStr(elem.Name))
		}
		if sub, ok := p.Term.(*xsd.ModelGroup); ok {
			collectElementNames(sub, names)
		}
	}
}

func qnameStr(qn xsd.QName) string {
	if qn.NS == "" {
		return qn.Local
	}
	return fmt.Sprintf("{%s}%s", qn.NS, qn.Local)
}

func buildElementDescription(elem *xsd.ElementDecl, expl *TypeExplanation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Element {%s}%s", expl.Namespace, expl.Name)

	if expl.Abstract {
		b.WriteString(" is abstract")
	}
	if expl.Nillable {
		b.WriteString(" and nillable")
	}

	if elem.Type != nil {
		fmt.Fprintf(&b, ". Its type is %s", expl.BaseType)
		if expl.Derivation != "" {
			fmt.Fprintf(&b, " (derived by %s)", expl.Derivation)
		}
	}

	if len(expl.Children) > 0 {
		fmt.Fprintf(&b, ". Child elements: %s", strings.Join(expl.Children, ", "))
	}

	if len(expl.Attributes) > 0 {
		fmt.Fprintf(&b, ". Attributes: %s", strings.Join(expl.Attributes, ", "))
	}

	if len(expl.Enumeration) > 0 {
		fmt.Fprintf(&b, ". Allowed values: %s", strings.Join(expl.Enumeration, ", "))
	}

	if expl.Pattern != "" {
		fmt.Fprintf(&b, ". Must match pattern: %s", expl.Pattern)
	}

	return b.String()
}

func buildTypeDescription(td *xsd.TypeDef, expl *TypeExplanation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s {%s}%s", expl.Kind, expl.Namespace, expl.Name)

	if expl.ContentType != "" && expl.Kind == "complex-type" {
		fmt.Fprintf(&b, " with %s content", expl.ContentType)
	}

	if expl.BaseType != "" {
		fmt.Fprintf(&b, ". Base type: %s", expl.BaseType)
		if expl.Derivation != "" {
			fmt.Fprintf(&b, " (derived by %s)", expl.Derivation)
		}
	}

	if len(expl.Children) > 0 {
		fmt.Fprintf(&b, ". Child elements: %s", strings.Join(expl.Children, ", "))
	}

	if len(expl.Attributes) > 0 {
		fmt.Fprintf(&b, ". Attributes: %s", strings.Join(expl.Attributes, ", "))
	}

	if len(expl.Enumeration) > 0 {
		fmt.Fprintf(&b, ". Allowed values: %s", strings.Join(expl.Enumeration, ", "))
	}

	if expl.Pattern != "" {
		fmt.Fprintf(&b, ". Must match pattern: %s", expl.Pattern)
	}

	return b.String()
}

// SchemaGraph represents a schema's component dependency graph.
type SchemaGraph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GraphNode represents a node in the schema dependency graph.
type GraphNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"` // "element", "simple-type", "complex-type", "attr-group", "group"
}

// GraphEdge represents a directed edge in the schema dependency graph.
type GraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"` // "type-of", "base-type", "ref", "subst-group"
}

// BuildSchemaGraph constructs the dependency graph from a compiled XSD schema.
func BuildSchemaGraph(schema *xsd.Schema) *SchemaGraph {
	graph := &SchemaGraph{}
	seenNodes := map[string]bool{}
	seenEdges := map[string]bool{}

	targetNS := schema.TargetNamespace()

	// Add all named types
	for _, qn := range schema.NamedTypes() {
		td, _ := schema.LookupType(qn.Local, qn.NS)
		if td == nil {
			continue
		}
		nodeID := qnameStr(qn)
		if !seenNodes[nodeID] {
			kind := "complex-type"
			if isSimpleTypeFamily(td) {
				kind = "simple-type"
			}
			graph.Nodes = append(graph.Nodes, GraphNode{
				ID:    nodeID,
				Label: qn.Local,
				Kind:  kind,
			})
			seenNodes[nodeID] = true
		}

		// Base type edge
		if td.BaseType != nil && td.BaseType.Name.Local != "" {
			baseID := qnameStr(td.BaseType.Name)
			addEdge(graph, seenNodes, seenEdges, nodeID, baseID, "base-type",
				func() GraphNode {
					return GraphNode{ID: baseID, Label: td.BaseType.Name.Local, Kind: "complex-type"}
				})
		}

		// Content model refs
		if td.ContentModel != nil {
			collectModelGraphEdges(td.ContentModel, nodeID, graph, seenNodes, seenEdges, schema)
		}
	}

	// Walk global elements
	// We use the named types list + substitution groups to find elements
	for _, qn := range schema.NamedTypes() {
		// Try looking up as element (heuristic: same QName)
		// Unfortunately Schema doesn't expose Elements() directly,
		// so we rely on walking types and their content models
		_ = qn
	}

	// Check substitution groups
	for _, qn := range schema.NamedTypes() {
		members := schema.SubstGroupMembers(qn)
		for _, member := range members {
			headID := qnameStr(qn)
			memberID := qnameStr(member.Name)
			addEdge(graph, seenNodes, seenEdges, memberID, headID, "subst-group",
				func() GraphNode {
					return GraphNode{ID: headID, Label: qn.Local, Kind: "element"}
				})
			if !seenNodes[memberID] {
				graph.Nodes = append(graph.Nodes, GraphNode{
					ID:    memberID,
					Label: member.Name.Local,
					Kind:  "element",
				})
				seenNodes[memberID] = true
			}
		}
	}

	_ = targetNS
	return graph
}

func collectModelGraphEdges(mg *xsd.ModelGroup, parentID string, graph *SchemaGraph, seenNodes map[string]bool, seenEdges map[string]bool, schema *xsd.Schema) {
	for _, p := range mg.Particles {
		switch term := p.Term.(type) {
		case *xsd.ElementDecl:
			childID := qnameStr(term.Name)
			edgeKey := parentID + "->" + childID + ":ref"
			if !seenEdges[edgeKey] {
				graph.Edges = append(graph.Edges, GraphEdge{
					From: parentID,
					To:   childID,
					Kind: "ref",
				})
				seenEdges[edgeKey] = true
			}
			if !seenNodes[childID] {
				graph.Nodes = append(graph.Nodes, GraphNode{
					ID:    childID,
					Label: term.Name.Local,
					Kind:  "element",
				})
				seenNodes[childID] = true
			}
			// Type-of edge
			if term.Type != nil && term.Type.Name.Local != "" {
				typeID := qnameStr(term.Type.Name)
				addEdge(graph, seenNodes, seenEdges, childID, typeID, "type-of",
					func() GraphNode {
						return GraphNode{ID: typeID, Label: term.Type.Name.Local, Kind: "complex-type"}
					})
			}
		case *xsd.ModelGroup:
			collectModelGraphEdges(term, parentID, graph, seenNodes, seenEdges, schema)
		}
	}
}

func addEdge(graph *SchemaGraph, seenNodes map[string]bool, seenEdges map[string]bool, from, to, kind string, makeToNode func() GraphNode) {
	edgeKey := from + "->" + to + ":" + kind
	if !seenEdges[edgeKey] {
		graph.Edges = append(graph.Edges, GraphEdge{
			From: from,
			To:   to,
			Kind: kind,
		})
		seenEdges[edgeKey] = true
	}
	if !seenNodes[to] {
		graph.Nodes = append(graph.Nodes, makeToNode())
		seenNodes[to] = true
	}
}

// GraphToMermaid converts a SchemaGraph to Mermaid diagram syntax.
func GraphToMermaid(graph *SchemaGraph) string {
	var b strings.Builder
	b.WriteString("graph TD\n")

	// Define node styles by kind
	styles := map[string]string{
		"element":      ":::elem",
		"complex-type": ":::ctype",
		"simple-type":  ":::stype",
		"attr-group":   ":::agroup",
		"group":        ":::group",
	}

	for _, node := range graph.Nodes {
		style := styles[node.Kind]
		safeID := mermaidSafeID(node.ID)
		fmt.Fprintf(&b, "  %s%s[\"%s\"]%s\n", safeID, safeID, node.Label, style)
	}

	// Edge styles by kind
	edgeStyles := map[string]string{
		"type-of":      "-->",
		"base-type":    "-.->",
		"ref":          "-->",
		"subst-group":  "==>",
	}

	for _, edge := range graph.Edges {
		style := edgeStyles[edge.Kind]
		if style == "" {
			style = "-->"
		}
		fromID := mermaidSafeID(edge.From)
		toID := mermaidSafeID(edge.To)
		fmt.Fprintf(&b, "  %s %s|%s| %s\n", fromID, style, edge.Kind, toID)
	}

	// Style definitions
	b.WriteString("\nclassDef elem fill:#4CAF50,stroke:#2E7D32,color:#fff\n")
	b.WriteString("classDef ctype fill:#2196F3,stroke:#1565C0,color:#fff\n")
	b.WriteString("classDef stype fill:#FF9800,stroke:#E65100,color:#fff\n")
	b.WriteString("classDef agroup fill:#9C27B0,stroke:#6A1B9A,color:#fff\n")
	b.WriteString("classDef group fill:#607D8B,stroke:#37474F,color:#fff\n")

	return b.String()
}

// GraphToDOT converts a SchemaGraph to Graphviz DOT syntax.
func GraphToDOT(graph *SchemaGraph) string {
	var b strings.Builder
	b.WriteString("digraph schema {\n")
	b.WriteString("  rankdir=TB;\n")
	b.WriteString("  node [shape=record, style=filled];\n\n")

	// Node shape/color by kind
	nodeStyles := map[string]struct {
		shape  string
		fillcolor string
		fontcolor string
	}{
		"element":      {"box", "#4CAF50", "#fff"},
		"complex-type": {"box", "#2196F3", "#fff"},
		"simple-type":  {"box", "#FF9800", "#fff"},
		"attr-group":   {"diamond", "#9C27B0", "#fff"},
		"group":        {"ellipse", "#607D8B", "#fff"},
	}

	for _, node := range graph.Nodes {
		s := nodeStyles[node.Kind]
		if s.fillcolor == "" {
			s = nodeStyles["complex-type"]
		}
		safeID := dotSafeID(node.ID)
		fmt.Fprintf(&b, "  %s [label=\"%s\", shape=%s, fillcolor=\"%s\", fontcolor=\"%s\"];\n",
			safeID, node.Label, s.shape, s.fillcolor, s.fontcolor)
	}

	b.WriteString("\n")

	edgeStyles := map[string]struct {
		style string
		label string
	}{
		"type-of":     {"solid", "type"},
		"base-type":   {"dashed", "base"},
		"ref":         {"solid", "ref"},
		"subst-group": {"bold", "subst"},
	}

	for _, edge := range graph.Edges {
		s := edgeStyles[edge.Kind]
		if s.style == "" {
			s = edgeStyles["ref"]
		}
		fmt.Fprintf(&b, "  %s -> %s [style=%s, label=\"%s\"];\n",
			dotSafeID(edge.From), dotSafeID(edge.To), s.style, s.label)
	}

	b.WriteString("}\n")
	return b.String()
}

func mermaidSafeID(id string) string {
	// Replace characters that are invalid in Mermaid node IDs
	r := strings.NewReplacer(
		"{", "_",
		"}", "_",
		":", "_",
		"/", "_",
		".", "_",
		" ", "_",
		"-", "_",
	)
	return r.Replace(id)
}

func dotSafeID(id string) string {
	// Replace characters that are invalid in DOT node IDs
	r := strings.NewReplacer(
		"{", "_",
		"}", "_",
		":", "_",
		"/", "_",
		".", "_",
		" ", "_",
		"-", "_",
	)
	return r.Replace(id)
}

// SchemaLintFinding represents a single finding from schema lint analysis.
type SchemaLintFinding struct {
	Severity   string `json:"severity"`    // "warning", "info"
	Category   string `json:"category"`   // "unused-type", "unreachable-element", "ambiguous-model", "missing-docs"
	Name       string `json:"name"`       // the component name
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
}

// LintSchema performs static analysis on a compiled XSD schema.
func LintSchema(schema *xsd.Schema) []SchemaLintFinding {
	var findings []SchemaLintFinding

	// 1. Find unreferenced types (types that are never used as element types or base types)
	// Skip XSD built-in types (namespace http://www.w3.org/2001/XMLSchema)
	referencedTypes := collectReferencedTypes(schema)
	xsdNS := "http://www.w3.org/2001/XMLSchema"

	for _, qn := range schema.NamedTypes() {
		// Skip XSD built-in types
		if qn.NS == xsdNS {
			continue
		}
		qname := qnameStr(qn)
		if !referencedTypes[qname] {
			findings = append(findings, SchemaLintFinding{
				Severity:   "info",
				Category:   "unused-type",
				Name:       qname,
				Message:    fmt.Sprintf("Type %s is not referenced by any element or other type", qname),
				Suggestion: "Consider removing unused types or documenting their purpose",
			})
		}
	}

	// 2. Check for abstract types with no concrete implementations
	for _, qn := range schema.NamedTypes() {
		if qn.NS == xsdNS {
			continue
		}
		td, _ := schema.LookupType(qn.Local, qn.NS)
		if td == nil {
			continue
		}
		if td.Abstract && td.ContentType != xsd.ContentTypeSimple {
			// Check if any element uses this type via substitution group or xsi:type
			hasConcrete := false
			for _, qn2 := range schema.NamedTypes() {
				td2, _ := schema.LookupType(qn2.Local, qn2.NS)
				if td2 != nil && td2.BaseType == td && !td2.Abstract {
					hasConcrete = true
					break
				}
			}
			if !hasConcrete {
				findings = append(findings, SchemaLintFinding{
					Severity:   "warning",
					Category:   "unreachable-element",
					Name:       qnameStr(qn),
					Message:    fmt.Sprintf("Abstract type %s has no concrete derivations", qnameStr(qn)),
					Suggestion: "Add a concrete type that extends or restricts this abstract type",
				})
			}
		}
	}

	// 3. Check for deeply nested model groups (complexity warning)
	for _, qn := range schema.NamedTypes() {
		if qn.NS == xsdNS {
			continue
		}
		td, _ := schema.LookupType(qn.Local, qn.NS)
		if td == nil || td.ContentModel == nil {
			continue
		}
		depth := maxModelNesting(td.ContentModel, 0)
		if depth > 5 {
			findings = append(findings, SchemaLintFinding{
				Severity:   "warning",
				Category:   "ambiguous-model",
				Name:       qnameStr(qn),
				Message:    fmt.Sprintf("Type %s has deeply nested content model (depth %d)", qnameStr(qn), depth),
				Suggestion: "Consider simplifying the content model or extracting sub-groups",
			})
		}
	}

	return findings
}

func collectReferencedTypes(schema *xsd.Schema) map[string]bool {
	refs := map[string]bool{}
	targetNS := schema.TargetNamespace()

	for _, qn := range schema.NamedTypes() {
		td, _ := schema.LookupType(qn.Local, qn.NS)
		if td == nil {
			continue
		}

		// Base type reference
		if td.BaseType != nil && td.BaseType.Name.Local != "" {
			refs[qnameStr(td.BaseType.Name)] = true
		}

		// Content model element type references
		if td.ContentModel != nil {
			collectModelTypeRefs(td.ContentModel, refs)
		}

		// Member types (union)
		for _, mt := range td.MemberTypes {
			if mt.Name.Local != "" {
				refs[qnameStr(mt.Name)] = true
			}
		}

		// Item type (list)
		if td.ItemType != nil && td.ItemType.Name.Local != "" {
			refs[qnameStr(td.ItemType.Name)] = true
		}
	}

	_ = targetNS
	return refs
}

func collectModelTypeRefs(mg *xsd.ModelGroup, refs map[string]bool) {
	for _, p := range mg.Particles {
		if elem, ok := p.Term.(*xsd.ElementDecl); ok {
			if elem.Type != nil && elem.Type.Name.Local != "" {
				refs[qnameStr(elem.Type.Name)] = true
			}
		}
		if sub, ok := p.Term.(*xsd.ModelGroup); ok {
			collectModelTypeRefs(sub, refs)
		}
	}
}

func maxModelNesting(mg *xsd.ModelGroup, depth int) int {
	maxDepth := depth
	for _, p := range mg.Particles {
		if sub, ok := p.Term.(*xsd.ModelGroup); ok {
			d := maxModelNesting(sub, depth+1)
			if d > maxDepth {
				maxDepth = d
			}
		}
	}
	return maxDepth
}

// RefResult represents a single reference to a schema component.
type RefResult struct {
	FromName string `json:"from-name"`
	FromKind string `json:"from-kind"` // "type", "element"
	ToName   string `json:"to-name"`
	ToKind   string `json:"to-kind"`   // "base-type", "type-of", "ref"
}

// FindRefs finds all references to a named type in the schema.
func FindRefs(schema *xsd.Schema, local, ns string) []RefResult {
	var refs []RefResult
	targetQName := xsd.QName{Local: local, NS: ns}

	for _, qn := range schema.NamedTypes() {
		td, _ := schema.LookupType(qn.Local, qn.NS)
		if td == nil {
			continue
		}

		// Check base type
		if td.BaseType != nil && td.BaseType.Name == targetQName {
			refs = append(refs, RefResult{
				FromName: qnameStr(qn),
				FromKind: "type",
				ToName:   qnameStr(targetQName),
				ToKind:   "base-type",
			})
		}

		// Check member types
		for _, mt := range td.MemberTypes {
			if mt.Name == targetQName {
				refs = append(refs, RefResult{
					FromName: qnameStr(qn),
					FromKind: "type",
					ToName:   qnameStr(targetQName),
					ToKind:   "member-type",
				})
			}
		}

		// Check content model element types
		if td.ContentModel != nil {
			findModelRefs(td.ContentModel, targetQName, qnameStr(qn), &refs)
		}
	}

	return refs
}

func findModelRefs(mg *xsd.ModelGroup, target xsd.QName, parentName string, refs *[]RefResult) {
	for _, p := range mg.Particles {
		if elem, ok := p.Term.(*xsd.ElementDecl); ok {
			if elem.Type != nil && elem.Type.Name == target {
				*refs = append(*refs, RefResult{
					FromName: parentName,
					FromKind: "element",
					ToName:   qnameStr(target),
					ToKind:   "type-of",
				})
			}
		}
		if sub, ok := p.Term.(*xsd.ModelGroup); ok {
			findModelRefs(sub, target, parentName, refs)
		}
	}
}

// ListSchemaComponents returns all named components in a schema for listing.
type SchemaComponent struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"` // "element", "simple-type", "complex-type"
}

// ListComponents returns all named elements and types in a compiled schema.
func ListComponents(schema *xsd.Schema) []SchemaComponent {
	var components []SchemaComponent

	for _, qn := range schema.NamedTypes() {
		td, _ := schema.LookupType(qn.Local, qn.NS)
		kind := "complex-type"
		if td != nil && isSimpleTypeFamily(td) {
			kind = "simple-type"
		}
		components = append(components, SchemaComponent{
			Name:      qn.Local,
			Namespace: qn.NS,
			Kind:      kind,
		})
	}

	return components
}
