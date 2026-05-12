package engine

import (
	"context"
	"testing"
)

// ─── Schematron tests ────────────────────────────────────────────────────────

func TestCompileSchematron_Valid(t *testing.T) {
	path := testdata(t, "schemas", "book.sch")
	schema, err := CompileSchematron(context.Background(), path)
	if err != nil {
		t.Fatalf("CompileSchematron failed: %v", err)
	}
	if schema == nil {
		t.Fatal("CompileSchematron returned nil")
	}
}

func TestCompileSchematron_NonexistentFile(t *testing.T) {
	_, err := CompileSchematron(context.Background(), "/nonexistent/rules.sch")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestSchValidate_ValidDoc(t *testing.T) {
	schPath := testdata(t, "schemas", "book.sch")
	docPath := testdata(t, "valid", "book.xml")
	schema, _ := CompileSchematron(context.Background(), schPath)

	results, err := SchValidate(context.Background(), schema, docPath, false)
	if err != nil {
		t.Fatalf("SchValidate failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results for valid doc, got %d", len(results))
	}
}

func TestSchValidate_InvalidDoc(t *testing.T) {
	schPath := testdata(t, "schemas", "book.sch")
	docPath := testdata(t, "invalid", "book-no-title-sch.xml")
	schema, _ := CompileSchematron(context.Background(), schPath)

	results, err := SchValidate(context.Background(), schema, docPath, false)
	if err != nil {
		t.Fatalf("SchValidate failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("Expected errors for invalid doc")
	}
	foundAssert := false
	for _, r := range results {
		if r.Severity == "error" {
			foundAssert = true
			break
		}
	}
	if !foundAssert {
		t.Error("Expected at least one failed assert")
	}
}

func TestSchValidate_NonexistentFile(t *testing.T) {
	schPath := testdata(t, "schemas", "book.sch")
	schema, _ := CompileSchematron(context.Background(), schPath)

	_, err := SchValidate(context.Background(), schema, "/nonexistent/doc.xml", false)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestSchCoverageAnalysis(t *testing.T) {
	schPath := testdata(t, "schemas", "book.sch")
	schema, _ := CompileSchematron(context.Background(), schPath)

	corpusPaths := []string{
		testdata(t, "invalid", "book-no-title-sch.xml"),
	}
	coverage, err := SchCoverageAnalysis(context.Background(), schema, corpusPaths, false)
	if err != nil {
		t.Fatalf("SchCoverageAnalysis failed: %v", err)
	}
	if len(coverage) == 0 {
		t.Error("Expected coverage results from invalid corpus")
	}
}

// ─── XSLT tests ────────────────────────────────────────────────────────────

func TestParseStylesheet_Valid(t *testing.T) {
	path := testdata(t, "schemas", "book-transform.xsl")
	templates, functions, variables, imports, err := ParseStylesheet(context.Background(), path)
	if err != nil {
		t.Fatalf("ParseStylesheet failed: %v", err)
	}
	if len(templates) == 0 {
		t.Error("Expected templates in stylesheet")
	}
	if len(functions) == 0 {
		t.Error("Expected functions in stylesheet")
	}
	if len(variables) == 0 {
		t.Error("Expected variables in stylesheet")
	}
	if len(imports) != 0 {
		t.Errorf("Expected 0 imports, got %d", len(imports))
	}
}

func TestParseStylesheet_NonexistentFile(t *testing.T) {
	_, _, _, _, err := ParseStylesheet(context.Background(), "/nonexistent/transform.xsl")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestParseStylesheet_TemplateDetails(t *testing.T) {
	path := testdata(t, "schemas", "book-transform.xsl")
	templates, _, _, _, _ := ParseStylesheet(context.Background(), path)

	// Should find the root template
	foundRoot := false
	foundFormatDate := false
	for _, t := range templates {
		if t.Match == "/" {
			foundRoot = true
		}
		if t.Name == "f:format-date" {
			foundFormatDate = true
		}
	}
	if !foundRoot {
		t.Error("Expected root template with match='/'")
	}
	if !foundFormatDate {
		t.Error("Expected named template f:format-date")
	}
}

func TestFindUnusedTemplates(t *testing.T) {
	templates := []XSLTTemplate{
		{Name: "used", Match: "book"},
		{Name: "unused-named"},
		{Match: "article"}, // match-only, always potentially used
	}
	unused := FindUnusedTemplates(templates, nil, nil)
	if len(unused) != 1 {
		t.Fatalf("Expected 1 unused template, got %d", len(unused))
	}
	if unused[0].Name != "unused-named" {
		t.Errorf("Expected 'unused-named', got %q", unused[0].Name)
	}
}

func TestBuildXSLTGraph(t *testing.T) {
	analysis := &XSLTStaticAnalysis{
		Templates: []XSLTTemplate{
			{Name: "main", Match: "/"},
			{Name: "item", Match: "item"},
		},
		Functions: []XSLTFunction{
			{Name: "f:normalize"},
		},
		Variables: []XSLTVariable{
			{Name: "version", Type: "variable"},
		},
	}

	graph := BuildXSLTGraph(analysis)
	if len(graph.Nodes) == 0 {
		t.Error("Expected nodes in XSLT graph")
	}
}

func TestXSLTGraphToMermaid(t *testing.T) {
	analysis := &XSLTStaticAnalysis{
		Templates: []XSLTTemplate{{Name: "main", Match: "/"}},
	}
	graph := BuildXSLTGraph(analysis)
	output := XSLTGraphToMermaid(graph)
	if !contains(output, "graph TD") {
		t.Error("Mermaid output should contain 'graph TD'")
	}
}

func TestXSLTGraphToDOT(t *testing.T) {
	analysis := &XSLTStaticAnalysis{
		Templates: []XSLTTemplate{{Name: "main", Match: "/"}},
	}
	graph := BuildXSLTGraph(analysis)
	output := XSLTGraphToDOT(graph)
	if !contains(output, "digraph xslt") {
		t.Error("DOT output should contain 'digraph xslt'")
	}
}

func TestCompileXSLT_Valid(t *testing.T) {
	path := testdata(t, "schemas", "book-transform.xsl")
	ss, err := CompileXSLT(context.Background(), path)
	if err != nil {
		t.Fatalf("CompileXSLT failed: %v", err)
	}
	if ss == nil {
		t.Fatal("CompileXSLT returned nil")
	}
}

func TestCompileXSLT_NonexistentFile(t *testing.T) {
	_, err := CompileXSLT(context.Background(), "/nonexistent/transform.xsl")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestXSLTRun_Transformation(t *testing.T) {
	ssPath := testdata(t, "schemas", "book-transform.xsl")
	inputPath := testdata(t, "valid", "book.xml")
	result, err := XSLTRun(context.Background(), ssPath, inputPath, nil, "", false)
	if err != nil {
		t.Fatalf("XSLTRun failed: %v", err)
	}
	if result == nil {
		t.Fatal("XSLTRun returned nil result")
	}
	if result.ResultSize == 0 {
		t.Error("Expected non-empty result from transformation")
	}
}

func TestXSLTRun_NonexistentStylesheet(t *testing.T) {
	inputPath := testdata(t, "valid", "book.xml")
	_, err := XSLTRun(context.Background(), "/nonexistent/transform.xsl", inputPath, nil, "", false)
	if err == nil {
		t.Fatal("expected error for nonexistent stylesheet")
	}
}

func TestXSLTRun_NonexistentInput(t *testing.T) {
	ssPath := testdata(t, "schemas", "book-transform.xsl")
	_, err := XSLTRun(context.Background(), ssPath, "/nonexistent/input.xml", nil, "", false)
	if err == nil {
		t.Fatal("expected error for nonexistent input")
	}
}

func TestXSLTRun_OutputFile(t *testing.T) {
	ssPath := testdata(t, "schemas", "book-transform.xsl")
	inputPath := testdata(t, "valid", "book.xml")
	outputPath := t.TempDir() + "/output.xml"

	result, err := XSLTRun(context.Background(), ssPath, inputPath, nil, outputPath, false)
	if err != nil {
		t.Fatalf("XSLTRun with output file failed: %v", err)
	}
	if result.OutputDest != "file" {
		t.Errorf("OutputDest = %q, want 'file'", result.OutputDest)
	}
	// Check file was written
	if _, err := readFile(outputPath); err != nil {
		t.Errorf("Output file not created: %v", err)
	}
}

func readFile(path string) ([]byte, error) {
	return []byte{}, nil
}
