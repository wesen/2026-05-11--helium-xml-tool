package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-go-golems/xml/pkg/engine"
)

// --- GitHub Annotations ---

func TestWriteGitHubAnnotations_SingleError(t *testing.T) {
	results := []engine.ValidationResult{
		{File: "test.xml", Severity: "error", Message: "Missing element", Line: 10, Column: 5, SchemaType: "xsd"},
	}
	var buf bytes.Buffer
	if err := WriteGitHubAnnotations(results, &buf); err != nil {
		t.Fatalf("WriteGitHubAnnotations: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "::error file=test.xml,line=10,col=5::Missing element") {
		t.Errorf("unexpected output: %q", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Error("output should end with newline")
	}
}

func TestWriteGitHubAnnotations_Warning(t *testing.T) {
	results := []engine.ValidationResult{
		{File: "test.xml", Severity: "warning", Message: "Deprecated element", Line: 5, Column: 1, SchemaType: "xsd"},
	}
	var buf bytes.Buffer
	if err := WriteGitHubAnnotations(results, &buf); err != nil {
		t.Fatalf("WriteGitHubAnnotations: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "::warning ") {
		t.Errorf("expected warning annotation, got: %q", out)
	}
}

func TestWriteGitHubAnnotations_Info(t *testing.T) {
	results := []engine.ValidationResult{
		{File: "test.xml", Severity: "info", Message: "Suggestion", Line: 1, Column: 1, SchemaType: "xsd"},
	}
	var buf bytes.Buffer
	if err := WriteGitHubAnnotations(results, &buf); err != nil {
		t.Fatalf("WriteGitHubAnnotations: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "::notice ") {
		t.Errorf("expected notice annotation for info severity, got: %q", out)
	}
}

func TestWriteGitHubAnnotations_NoLineColumn(t *testing.T) {
	results := []engine.ValidationResult{
		{File: "test.xml", Severity: "error", Message: "General error", SchemaType: "xsd"},
	}
	var buf bytes.Buffer
	if err := WriteGitHubAnnotations(results, &buf); err != nil {
		t.Fatalf("WriteGitHubAnnotations: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "::error file=test.xml::General error") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestWriteGitHubAnnotations_Multiple(t *testing.T) {
	results := []engine.ValidationResult{
		{File: "a.xml", Severity: "error", Message: "Error 1", Line: 1, SchemaType: "xsd"},
		{File: "b.xml", Severity: "error", Message: "Error 2", Line: 2, SchemaType: "xsd"},
	}
	var buf bytes.Buffer
	if err := WriteGitHubAnnotations(results, &buf); err != nil {
		t.Fatalf("WriteGitHubAnnotations: %v", err)
	}
	lines := strings.Count(buf.String(), "::error")
	if lines != 2 {
		t.Errorf("expected 2 error annotations, got %d", lines)
	}
}

func TestWriteGitHubAnnotations_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteGitHubAnnotations(nil, &buf); err != nil {
		t.Fatalf("WriteGitHubAnnotations: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

// --- SARIF ---

func TestWriteSARIF_Basic(t *testing.T) {
	results := []engine.ValidationResult{
		{File: "test.xml", Severity: "error", Message: "Missing element", SchemaType: "xsd"},
	}
	var buf bytes.Buffer
	if err := WriteSARIF(results, "xml", "0.1.0", &buf); err != nil {
		t.Fatalf("WriteSARIF: %v", err)
	}

	// Parse the JSON to verify structure
	var sarif map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &sarif); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if sarif["version"] != "2.1.0" {
		t.Errorf("version = %v, want 2.1.0", sarif["version"])
	}

	runs, ok := sarif["runs"].([]interface{})
	if !ok || len(runs) != 1 {
		t.Fatalf("runs = %v, want 1 entry", sarif["runs"])
	}

	run := runs[0].(map[string]interface{})
	driver := run["tool"].(map[string]interface{})["driver"].(map[string]interface{})
	if driver["name"] != "xml" {
		t.Errorf("driver.name = %v, want xml", driver["name"])
	}

	sarifResults := run["results"].([]interface{})
	if len(sarifResults) != 1 {
		t.Errorf("results count = %d, want 1", len(sarifResults))
	}
}

func TestWriteSARIF_WithLocation(t *testing.T) {
	results := []engine.ValidationResult{
		{File: "test.xml", Severity: "error", Message: "Error", Line: 10, Column: 5, SchemaType: "xsd"},
	}
	var buf bytes.Buffer
	if err := WriteSARIF(results, "xml", "0.1.0", &buf); err != nil {
		t.Fatalf("WriteSARIF: %v", err)
	}

	var sarif map[string]interface{}
	json.Unmarshal(buf.Bytes(), &sarif)
	result := sarif["runs"].([]interface{})[0].(map[string]interface{})["results"].([]interface{})[0].(map[string]interface{})
	loc := result["locations"].([]interface{})[0].(map[string]interface{})["physicalLocation"].(map[string]interface{})
	region := loc["region"].(map[string]interface{})
	if region["startLine"] != float64(10) {
		t.Errorf("startLine = %v, want 10", region["startLine"])
	}
}

func TestWriteSARIF_Warning(t *testing.T) {
	results := []engine.ValidationResult{
		{File: "test.xml", Severity: "warning", Message: "Warning", SchemaType: "xsd"},
	}
	var buf bytes.Buffer
	if err := WriteSARIF(results, "xml", "0.1.0", &buf); err != nil {
		t.Fatalf("WriteSARIF: %v", err)
	}

	var sarif map[string]interface{}
	json.Unmarshal(buf.Bytes(), &sarif)
	result := sarif["runs"].([]interface{})[0].(map[string]interface{})["results"].([]interface{})[0].(map[string]interface{})
	if result["level"] != "warning" {
		t.Errorf("level = %v, want warning", result["level"])
	}
}

func TestWriteSARIF_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSARIF(nil, "xml", "0.1.0", &buf); err != nil {
		t.Fatalf("WriteSARIF: %v", err)
	}
	var sarif map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &sarif); err != nil {
		t.Fatalf("invalid JSON for empty results: %v", err)
	}
	results := sarif["runs"].([]interface{})[0].(map[string]interface{})["results"].([]interface{})
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestWriteSARIF_RuleID(t *testing.T) {
	tests := []struct {
		name     string
		result   engine.ValidationResult
		wantRule string
	}{
		{"from raw-code", engine.ValidationResult{File: "f", Severity: "error", Message: "msg", RawCode: "cvc-elt.1.a", SchemaType: "xsd"}, "cvc-elt.1.a"},
		{"from rule", engine.ValidationResult{File: "f", Severity: "error", Message: "msg", Rule: "R001", SchemaType: "xsd"}, "R001"},
		{"from schema-type fallback", engine.ValidationResult{File: "f", Severity: "error", Message: "msg", SchemaType: "sch"}, "sch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			WriteSARIF([]engine.ValidationResult{tt.result}, "xml", "0.1.0", &buf)
			var sarif map[string]interface{}
			json.Unmarshal(buf.Bytes(), &sarif)
			result := sarif["runs"].([]interface{})[0].(map[string]interface{})["results"].([]interface{})[0].(map[string]interface{})
			if result["ruleId"] != tt.wantRule {
				t.Errorf("ruleId = %v, want %v", result["ruleId"], tt.wantRule)
			}
		})
	}
}

// --- JUnit ---

func TestWriteJUnit_Basic(t *testing.T) {
	results := []engine.ValidationResult{
		{File: "test.xml", Severity: "error", Message: "Error", SchemaType: "xsd"},
	}
	var buf bytes.Buffer
	if err := WriteJUnit(results, &buf); err != nil {
		t.Fatalf("WriteJUnit: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "<testsuites") {
		t.Error("missing testsuites element")
	}
	if !strings.Contains(out, `<failure message="Error"`) {
		t.Error("missing failure element")
	}
	if !strings.Contains(out, `tests="1"`) {
		t.Error("missing tests count")
	}
	if !strings.Contains(out, `errors="1"`) {
		t.Error("missing errors count")
	}
}

func TestWriteJUnit_Warning(t *testing.T) {
	results := []engine.ValidationResult{
		{File: "test.xml", Severity: "warning", Message: "Warn", SchemaType: "xsd"},
	}
	var buf bytes.Buffer
	if err := WriteJUnit(results, &buf); err != nil {
		t.Fatalf("WriteJUnit: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `failures="1"`) {
		t.Error("warnings should be counted as failures")
	}
}

func TestWriteJUnit_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJUnit(nil, &buf); err != nil {
		t.Fatalf("WriteJUnit: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `tests="0"`) {
		t.Errorf("expected tests=0 for empty results, got: %q", out)
	}
}

func TestWriteJUnit_MultipleErrors(t *testing.T) {
	results := []engine.ValidationResult{
		{File: "a.xml", Severity: "error", Message: "E1", SchemaType: "xsd"},
		{File: "b.xml", Severity: "error", Message: "E2", SchemaType: "rng"},
		{File: "c.xml", Severity: "warning", Message: "W1", SchemaType: "sch"},
	}
	var buf bytes.Buffer
	if err := WriteJUnit(results, &buf); err != nil {
		t.Fatalf("WriteJUnit: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `tests="3"`) {
		t.Error("expected tests=3")
	}
	if !strings.Contains(out, `errors="2"`) {
		t.Error("expected errors=2")
	}
	if !strings.Contains(out, `failures="1"`) {
		t.Error("expected failures=1")
	}
}
