package engine

import (
	"context"
	"os"
	"testing"
)

func TestValidationPipeline_XSD(t *testing.T) {
	steps := []ValidationStep{
		{Type: "xsd", SchemaFile: "../../test/testdata/schemas/invoice.xsd"},
	}

	pipeline := NewPipeline(steps)

	// Valid document should produce no results
	results, err := pipeline.Run(context.Background(), "../../test/testdata/valid/invoice.xml")
	if err != nil {
		t.Fatalf("pipeline run: %v", err)
	}
	if len(results) > 0 {
		t.Errorf("expected 0 results for valid doc, got %d: %+v", len(results), results)
	}

	// Invalid document should produce errors
	results, err = pipeline.Run(context.Background(), "../../test/testdata/invalid/invoice-bad.xml")
	if err != nil {
		t.Fatalf("pipeline run: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected errors for invalid doc, got 0")
	}

	// Check all results are errors
	for _, r := range results {
		if r.SchemaType != "xsd" {
			t.Errorf("expected schema-type=xsd, got %q", r.SchemaType)
		}
	}
}

func TestValidationPipeline_RNG(t *testing.T) {
	steps := []ValidationStep{
		{Type: "rng", SchemaFile: "../../test/testdata/schemas/book.rng"},
	}

	pipeline := NewPipeline(steps)

	// Valid document
	results, err := pipeline.Run(context.Background(), "../../test/testdata/valid/book.xml")
	if err != nil {
		t.Fatalf("pipeline run: %v", err)
	}
	if len(results) > 0 {
		t.Errorf("expected 0 results for valid doc, got %d", len(results))
	}

	// Invalid document
	results, err = pipeline.Run(context.Background(), "../../test/testdata/invalid/book-no-title.xml")
	if err != nil {
		t.Fatalf("pipeline run: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected errors for invalid doc")
	}
}

func TestValidationPipeline_MalformedXML(t *testing.T) {
	// Create a temp file with malformed XML
	tmpFile := t.TempDir() + "/malformed.xml"
	if err := writeFile(tmpFile, []byte("<root><unclosed>")); err != nil {
		t.Fatal(err)
	}

	pipeline := NewPipeline(nil)
	results, err := pipeline.Run(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("pipeline run: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected error for malformed XML")
	}
	if results[0].SchemaType != "well-formedness" {
		t.Errorf("expected schema-type=well-formedness, got %q", results[0].SchemaType)
	}
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
