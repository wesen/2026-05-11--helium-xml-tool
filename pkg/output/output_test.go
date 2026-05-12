package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-go-golems/xml/pkg/engine"
)

func TestWriteGitHubAnnotations(t *testing.T) {
	results := []engine.ValidationResult{
		{
			File:       "test.xml",
			Severity:   "error",
			Message:    "Missing required element",
			Line:       10,
			Column:     5,
			SchemaType: "xsd",
		},
	}

	var buf bytes.Buffer
	err := WriteGitHubAnnotations(results, &buf)
	if err != nil {
		t.Fatalf("WriteGitHubAnnotations: %v", err)
	}

	output := buf.String()
	if output != "::error file=test.xml,line=10,col=5::Missing required element\n" {
		t.Errorf("unexpected output: %q", output)
	}
}

func TestWriteSARIF(t *testing.T) {
	results := []engine.ValidationResult{
		{
			File:       "test.xml",
			Severity:   "error",
			Message:    "Missing required element",
			SchemaType: "xsd",
		},
	}

	var buf bytes.Buffer
	err := WriteSARIF(results, "xml", "0.1.0", &buf)
	if err != nil {
		t.Fatalf("WriteSARIF: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, (`"version": "2.1.0"`)) {
		t.Error("SARIF output missing version")
	}
	if !strings.Contains(output, (`"name": "xml"`)) {
		t.Error("SARIF output missing tool name")
	}
}

func TestWriteJUnit(t *testing.T) {
	results := []engine.ValidationResult{
		{
			File:       "test.xml",
			Severity:   "error",
			Message:    "Missing required element",
			SchemaType: "xsd",
		},
	}

	var buf bytes.Buffer
	err := WriteJUnit(results, &buf)
	if err != nil {
		t.Fatalf("WriteJUnit: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, (`<testsuites`)) {
		t.Error("JUnit output missing testsuites element")
	}
}
