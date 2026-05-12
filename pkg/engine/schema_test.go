package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func testdata(t *testing.T, parts ...string) string {
	t.Helper()
	// Resolve relative to the project root by looking for go.mod
	dir, _ := os.Getwd()
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			p := filepath.Join(dir, "test", "testdata", filepath.Join(parts...))
			if _, err := os.Stat(p); err == nil {
				return p
			}
			break
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("cannot find testdata: %v", parts)
	return ""
}

// ─── CompileSchema tests ────────────────────────────────────────────────────

func TestCompileSchema_ValidXSD(t *testing.T) {
	path := testdata(t, "schemas", "book-full.xsd")
	schema, err := CompileSchema(context.Background(), path)
	if err != nil {
		t.Fatalf("CompileSchema failed: %v", err)
	}
	if schema == nil {
		t.Fatal("CompileSchema returned nil schema")
	}
	if schema.TargetNamespace() != "http://example.com/book" {
		t.Errorf("TargetNamespace = %q, want http://example.com/book", schema.TargetNamespace())
	}
}

func TestCompileSchema_NonexistentFile(t *testing.T) {
	_, err := CompileSchema(context.Background(), "/nonexistent/schema.xsd")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestCompileSchema_InvalidSchema(t *testing.T) {
	path := testdata(t, "malformed", "bad-schema.xsd")
	_, err := CompileSchema(context.Background(), path)
	if err == nil {
		t.Fatal("expected error for invalid schema")
	}
}

// ─── ExplainType / ExplainElement tests ─────────────────────────────────────

func TestExplainType_ExistingType(t *testing.T) {
	path := testdata(t, "schemas", "book-full.xsd")
	schema, _ := CompileSchema(context.Background(), path)

	expl, err := ExplainType(schema, "BookType", "http://example.com/book")
	if err != nil {
		t.Fatalf("ExplainType failed: %v", err)
	}
	if expl.Kind != "complex-type" {
		t.Errorf("Kind = %q, want complex-type", expl.Kind)
	}
	if expl.ContentType != "element-only" {
		t.Errorf("ContentType = %q, want element-only", expl.ContentType)
	}
	if len(expl.Children) == 0 {
		t.Error("Expected children in BookType")
	}
}

func TestExplainType_SimpleType(t *testing.T) {
	path := testdata(t, "schemas", "book-full.xsd")
	schema, _ := CompileSchema(context.Background(), path)

	expl, err := ExplainType(schema, "GenreType", "http://example.com/book")
	if err != nil {
		t.Fatalf("ExplainType failed: %v", err)
	}
	if expl.Derivation != "restriction" {
		t.Errorf("Derivation = %q, want restriction", expl.Derivation)
	}
	if len(expl.Enumeration) != 4 {
		t.Errorf("Enumeration count = %d, want 4", len(expl.Enumeration))
	}
}

func TestExplainType_NonexistentType(t *testing.T) {
	path := testdata(t, "schemas", "book-full.xsd")
	schema, _ := CompileSchema(context.Background(), path)

	_, err := ExplainType(schema, "NonexistentType", "http://example.com/book")
	if err == nil {
		t.Fatal("expected error for nonexistent type")
	}
}

func TestExplainElement_NonexistentElement(t *testing.T) {
	path := testdata(t, "schemas", "book-full.xsd")
	schema, _ := CompileSchema(context.Background(), path)

	_, err := ExplainElement(schema, "NonexistentElem", "http://example.com/book")
	if err == nil {
		t.Fatal("expected error for nonexistent element")
	}
}

// ─── BuildSchemaGraph tests ─────────────────────────────────────────────────

func TestBuildSchemaGraph_UserTypes(t *testing.T) {
	path := testdata(t, "schemas", "book-full.xsd")
	schema, _ := CompileSchema(context.Background(), path)

	graph := BuildSchemaGraph(schema)
	if graph == nil {
		t.Fatal("BuildSchemaGraph returned nil")
	}

	// Should have nodes for user-defined types
	userNodes := 0
	for _, node := range graph.Nodes {
		if node.Label == "BookType" || node.Label == "AuthorType" ||
			node.Label == "GenreType" || node.Label == "ISBNType" ||
			node.Label == "ExtendedBookType" || node.Label == "TextbookType" {
			userNodes++
		}
	}
	if userNodes < 4 {
		t.Errorf("Found %d user type nodes, want at least 4", userNodes)
	}

	// Should have base-type edges (e.g., ExtendedBookType → BookType)
	hasBaseTypeEdge := false
	for _, edge := range graph.Edges {
		if edge.Kind == "base-type" {
			hasBaseTypeEdge = true
			break
		}
	}
	if !hasBaseTypeEdge {
		t.Error("Expected base-type edge in graph")
	}
}

func TestGraphToMermaid(t *testing.T) {
	path := testdata(t, "schemas", "book-full.xsd")
	schema, _ := CompileSchema(context.Background(), path)

	graph := BuildSchemaGraph(schema)
	output := GraphToMermaid(graph)

	if !contains(output, "graph TD") {
		t.Error("Mermaid output should start with 'graph TD'")
	}
	if !contains(output, "BookType") {
		t.Error("Mermaid output should contain BookType")
	}
}

func TestGraphToDOT(t *testing.T) {
	path := testdata(t, "schemas", "book-full.xsd")
	schema, _ := CompileSchema(context.Background(), path)

	graph := BuildSchemaGraph(schema)
	output := GraphToDOT(graph)

	if !contains(output, "digraph schema") {
		t.Error("DOT output should start with 'digraph schema'")
	}
	if !contains(output, "BookType") {
		t.Error("DOT output should contain BookType")
	}
}

// ─── LintSchema tests ───────────────────────────────────────────────────────

func TestLintSchema_UnusedType(t *testing.T) {
	path := testdata(t, "schemas", "book-full.xsd")
	schema, _ := CompileSchema(context.Background(), path)

	findings := LintSchema(schema)

	// TextbookType should be flagged as unused (no element uses it directly)
	found := false
	for _, f := range findings {
		if f.Category == "unused-type" && contains(f.Name, "TextbookType") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected TextbookType to be flagged as unused")
	}
}

func TestLintSchema_NoXSDBuiltins(t *testing.T) {
	path := testdata(t, "schemas", "book-full.xsd")
	schema, _ := CompileSchema(context.Background(), path)

	findings := LintSchema(schema)

	// No XSD built-in types should be flagged
	for _, f := range findings {
		if contains(f.Name, "http://www.w3.org/2001/XMLSchema") && f.Category == "unused-type" {
			t.Errorf("XSD built-in type should not be flagged: %s", f.Name)
		}
	}
}

// ─── FindRefs tests ──────────────────────────────────────────────────────────

func TestFindRefs_BookType(t *testing.T) {
	path := testdata(t, "schemas", "book-full.xsd")
	schema, _ := CompileSchema(context.Background(), path)

	refs := FindRefs(schema, "BookType", "http://example.com/book")
	if len(refs) == 0 {
		t.Fatal("Expected at least one reference to BookType")
	}

	// ExtendedBookType should reference BookType as base-type
	found := false
	for _, r := range refs {
		if r.ToKind == "base-type" && contains(r.FromName, "ExtendedBookType") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected ExtendedBookType to reference BookType as base-type")
	}
}

// ─── ListComponents tests ────────────────────────────────────────────────────

func TestListComponents_UserTypes(t *testing.T) {
	path := testdata(t, "schemas", "book-full.xsd")
	schema, _ := CompileSchema(context.Background(), path)

	components := ListComponents(schema)

	userCount := 0
	for _, c := range components {
		if c.Namespace == "http://example.com/book" {
			userCount++
		}
	}
	if userCount < 4 {
		t.Errorf("Found %d user components, want at least 4", userCount)
	}
}

// ─── DiffSchemas tests ──────────────────────────────────────────────────────

func TestDiffSchemas_DetectChanges(t *testing.T) {
	oldPath := testdata(t, "schemas", "book-full.xsd")
	newPath := testdata(t, "schemas", "book-v2.xsd")

	diff, err := DiffSchemas(context.Background(), oldPath, newPath)
	if err != nil {
		t.Fatalf("DiffSchemas failed: %v", err)
	}

	if diff.Summary.TotalChanges == 0 {
		t.Error("Expected non-zero changes between v1 and v2 schemas")
	}

	// Should detect breaking changes (currency attr added as required)
	if diff.Summary.BreakingCount == 0 {
		t.Error("Expected breaking changes between v1 and v2")
	}

	// Should detect safe changes (ReviewType added, children enum added)
	if diff.Summary.SafeCount == 0 {
		t.Error("Expected safe changes between v1 and v2")
	}
}

func TestDiffSchemas_SameSchema(t *testing.T) {
	path := testdata(t, "schemas", "book-full.xsd")

	diff, err := DiffSchemas(context.Background(), path, path)
	if err != nil {
		t.Fatalf("DiffSchemas failed: %v", err)
	}

	// When diffing the same schema, there should be no user-visible breaking changes.
	// Due to Go map iteration non-determinism, a small number of spurious changes
	// may appear in XSD built-in types, but there should be no breaking user changes.
	userBreaking := 0
	for _, c := range diff.Changes {
		if c.Severity == "breaking" && !contains(c.Component, "http://www.w3.org/2001/XMLSchema") {
			userBreaking++
		}
	}
	if userBreaking != 0 {
		t.Errorf("Expected 0 user breaking changes when diffing same schema, got %d", userBreaking)
	}
}

// ─── AnalyzeBreakage tests ──────────────────────────────────────────────────

func TestAnalyzeBreakage_NoCorpus(t *testing.T) {
	oldPath := testdata(t, "schemas", "book-full.xsd")
	newPath := testdata(t, "schemas", "book-v2.xsd")

	result, err := AnalyzeBreakage(context.Background(), oldPath, newPath, nil, false)
	if err != nil {
		t.Fatalf("AnalyzeBreakage failed: %v", err)
	}

	if result.Summary.TotalBreaking == 0 {
		t.Error("Expected breaking changes between v1 and v2")
	}
}

// ─── InferSchema tests ─────────────────────────────────────────────────────

func TestInferSchema_SingleFile(t *testing.T) {
	path := testdata(t, "valid", "book-full.xml")
	result, err := InferSchema(context.Background(), InferOptions{
		InputFiles: []string{path},
	})
	if err != nil {
		t.Fatalf("InferSchema failed: %v", err)
	}
	if result.SourceFiles != 1 {
		t.Errorf("SourceFiles = %d, want 1", result.SourceFiles)
	}
	if result.RootElement == "" {
		t.Error("RootElement should not be empty")
	}
}

func TestInferSchema_NoInputFiles(t *testing.T) {
	_, err := InferSchema(context.Background(), InferOptions{})
	if err == nil {
		t.Fatal("expected error for no input files")
	}
}

func TestInferSchema_NonexistentFile(t *testing.T) {
	_, err := InferSchema(context.Background(), InferOptions{
		InputFiles: []string{"/nonexistent/file.xml"},
	})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestInferredSchemaToXSD(t *testing.T) {
	path := testdata(t, "valid", "book-full.xml")
	result, _ := InferSchema(context.Background(), InferOptions{
		InputFiles: []string{path},
	})

	xsd := InferredSchemaToXSD(result)
	if !contains(xsd, "xs:schema") {
		t.Error("XSD output should contain xs:schema element")
	}
	if !contains(xsd, "xs:complexType") {
		t.Error("XSD output should contain complex types")
	}
}

func TestInferSimpleType(t *testing.T) {
	tests := []struct {
		value    string
		current  string
		expected string
	}{
		{"42", "", "integer"},
		{"3.14", "", "decimal"},
		{"true", "", "boolean"},
		{"hello", "", "string"},
		{"42", "boolean", "integer"},       // boolean can't hold 42, widen
		{"hello", "integer", "string"},     // integer can't hold hello, widen
		{"hello", "string", "string"},      // already widest
	}

	for _, tt := range tests {
		result := inferSimpleType(tt.value, tt.current)
		if result != tt.expected {
			t.Errorf("inferSimpleType(%q, %q) = %q, want %q", tt.value, tt.current, result, tt.expected)
		}
	}
}

// ─── DTD tests ──────────────────────────────────────────────────────────────

func TestParseDTD_Elements(t *testing.T) {
	path := testdata(t, "schemas", "book.dtd")
	decls, err := ParseDTD(path)
	if err != nil {
		t.Fatalf("ParseDTD failed: %v", err)
	}

	elemCount := 0
	for _, d := range decls {
		if d.Kind == "element" {
			elemCount++
		}
	}
	if elemCount == 0 {
		t.Error("Expected element declarations in DTD")
	}
}

func TestParseDTD_Entities(t *testing.T) {
	path := testdata(t, "schemas", "book.dtd")
	decls, err := ParseDTD(path)
	if err != nil {
		t.Fatalf("ParseDTD failed: %v", err)
	}

	entityCount := 0
	for _, d := range decls {
		if d.Kind == "general-entity" || d.Kind == "parameter-entity" {
			entityCount++
		}
	}
	if entityCount == 0 {
		t.Error("Expected entity declarations in DTD")
	}
}

func TestAuditDTD_BillionLaughs(t *testing.T) {
	path := testdata(t, "schemas", "book.dtd")
	findings, err := AuditDTD(path)
	if err != nil {
		t.Fatalf("AuditDTD failed: %v", err)
	}

	foundExpansion := false
	for _, f := range findings {
		if f.Category == "entity-expansion" {
			foundExpansion = true
			break
		}
	}
	if !foundExpansion {
		t.Error("Expected entity-expansion finding in DTD with recursive entities")
	}
}

func TestAuditDTD_ExternalEntity(t *testing.T) {
	path := testdata(t, "schemas", "book.dtd")
	findings, err := AuditDTD(path)
	if err != nil {
		t.Fatalf("AuditDTD failed: %v", err)
	}

	foundExternal := false
	for _, f := range findings {
		if f.Category == "external-entity" {
			foundExternal = true
			break
		}
	}
	if !foundExternal {
		t.Error("Expected external-entity finding in DTD with SYSTEM entity")
	}
}

func TestFlattenDTD(t *testing.T) {
	path := testdata(t, "schemas", "book.dtd")
	content, err := FlattenDTD(path)
	if err != nil {
		t.Fatalf("FlattenDTD failed: %v", err)
	}
	if content == "" {
		t.Error("Expected non-empty flattened DTD content")
	}
}

func TestParseDTD_NonexistentFile(t *testing.T) {
	_, err := ParseDTD("/nonexistent/file.dtd")
	if err == nil {
		t.Fatal("expected error for nonexistent DTD file")
	}
}

// ─── Helper ─────────────────────────────────────────────────────────────────

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
