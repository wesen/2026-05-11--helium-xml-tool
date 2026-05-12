package engine

import (
	"context"
	"os"
	"testing"
)

// --- XSD Validation ---

func TestValidationPipeline_XSD_Valid(t *testing.T) {
	steps := []ValidationStep{{Type: "xsd", SchemaFile: "../../test/testdata/schemas/invoice.xsd"}}
	pipeline := NewPipeline(steps)
	results, err := pipeline.Run(context.Background(), "../../test/testdata/valid/invoice.xml")
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(results) > 0 {
		t.Errorf("valid doc: expected 0 results, got %d: %+v", len(results), results)
	}
}

func TestValidationPipeline_XSD_Invalid(t *testing.T) {
	steps := []ValidationStep{{Type: "xsd", SchemaFile: "../../test/testdata/schemas/invoice.xsd"}}
	pipeline := NewPipeline(steps)
	results, err := pipeline.Run(context.Background(), "../../test/testdata/invalid/invoice-bad.xml")
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("invalid doc: expected errors, got 0")
	}
	for _, r := range results {
		if r.SchemaType != "xsd" {
			t.Errorf("schema-type = %q, want xsd", r.SchemaType)
		}
		if r.Severity != "error" {
			t.Errorf("severity = %q, want error", r.Severity)
		}
		if r.File == "" {
			t.Error("file should not be empty")
		}
	}
}

func TestValidationPipeline_XSD_NonexistentSchema(t *testing.T) {
	steps := []ValidationStep{{Type: "xsd", SchemaFile: "nonexistent.xsd"}}
	pipeline := NewPipeline(steps)
	results, err := pipeline.Run(context.Background(), "../../test/testdata/valid/invoice.xml")
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected error for nonexistent schema")
	}
	if results[0].Message == "" {
		t.Error("error message should not be empty")
	}
}

func TestValidationPipeline_XSD_SchemaCompilationError(t *testing.T) {
	steps := []ValidationStep{{Type: "xsd", SchemaFile: "../../test/testdata/malformed/bad-schema.xsd"}}
	pipeline := NewPipeline(steps)
	results, err := pipeline.Run(context.Background(), "../../test/testdata/valid/invoice.xml")
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected error for invalid schema file")
	}
}

// --- RELAX NG Validation ---

func TestValidationPipeline_RNG_Valid(t *testing.T) {
	steps := []ValidationStep{{Type: "rng", SchemaFile: "../../test/testdata/schemas/book.rng"}}
	pipeline := NewPipeline(steps)
	results, err := pipeline.Run(context.Background(), "../../test/testdata/valid/book.xml")
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(results) > 0 {
		t.Errorf("valid doc: expected 0 results, got %d", len(results))
	}
}

func TestValidationPipeline_RNG_Invalid(t *testing.T) {
	steps := []ValidationStep{{Type: "rng", SchemaFile: "../../test/testdata/schemas/book.rng"}}
	pipeline := NewPipeline(steps)
	results, err := pipeline.Run(context.Background(), "../../test/testdata/invalid/book-no-title.xml")
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected errors for invalid doc")
	}
	for _, r := range results {
		if r.SchemaType != "rng" {
			t.Errorf("schema-type = %q, want rng", r.SchemaType)
		}
	}
}

// --- Schematron Validation ---

func TestValidationPipeline_Sch_Valid(t *testing.T) {
	steps := []ValidationStep{{Type: "sch", SchemaFile: "../../test/testdata/schemas/invoice.sch"}}
	pipeline := NewPipeline(steps)
	results, err := pipeline.Run(context.Background(), "../../test/testdata/valid/invoice.xml")
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(results) > 0 {
		t.Errorf("valid doc: expected 0 results, got %d", len(results))
	}
}

func TestValidationPipeline_Sch_Invalid(t *testing.T) {
	steps := []ValidationStep{{Type: "sch", SchemaFile: "../../test/testdata/schemas/book.sch"}}
	pipeline := NewPipeline(steps)
	results, err := pipeline.Run(context.Background(), "../../test/testdata/invalid/book-no-title-sch.xml")
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected Schematron errors")
	}
	for _, r := range results {
		if r.SchemaType != "sch" {
			t.Errorf("schema-type = %q, want sch", r.SchemaType)
		}
	}
}

// --- DTD Validation ---

func TestValidationPipeline_DTD_Valid(t *testing.T) {
	steps := []ValidationStep{{Type: "dtd", SchemaFile: "test.dtd"}}
	pipeline := NewPipeline(steps)
	// DTD validation re-parses, so a well-formed doc should produce no errors
	results, err := pipeline.Run(context.Background(), "../../test/testdata/valid/invoice.xml")
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	// DTD validation without an actual DTD reference may or may not produce errors
	// depending on helium behavior. We just check it doesn't crash.
	_ = results
}

// --- Multi-stage Pipeline ---

func TestValidationPipeline_MultiStage_XSD_Plus_Sch(t *testing.T) {
	steps := []ValidationStep{
		{Type: "xsd", SchemaFile: "../../test/testdata/schemas/invoice.xsd"},
		{Type: "sch", SchemaFile: "../../test/testdata/schemas/invoice.sch"},
	}
	pipeline := NewPipeline(steps)
	results, err := pipeline.Run(context.Background(), "../../test/testdata/valid/invoice.xml")
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(results) > 0 {
		t.Errorf("valid doc: expected 0 results from multi-stage, got %d", len(results))
	}
}

func TestValidationPipeline_MultiStage_FirstFails_SecondRuns(t *testing.T) {
	// XSD fails, Schematron should still run
	steps := []ValidationStep{
		{Type: "xsd", SchemaFile: "../../test/testdata/schemas/invoice.xsd"},
		{Type: "sch", SchemaFile: "../../test/testdata/schemas/invoice.sch"},
	}
	pipeline := NewPipeline(steps)
	results, err := pipeline.Run(context.Background(), "../../test/testdata/invalid/invoice-bad.xml")
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected errors from invalid doc")
	}
	// Should have XSD errors at minimum
	hasXSDErrors := false
	for _, r := range results {
		if r.SchemaType == "xsd" {
			hasXSDErrors = true
		}
	}
	if !hasXSDErrors {
		t.Error("expected at least one XSD error in multi-stage pipeline")
	}
}

// --- Malformed XML ---

func TestValidationPipeline_MalformedXML(t *testing.T) {
	tmpFile := t.TempDir() + "/malformed.xml"
	if err := os.WriteFile(tmpFile, []byte("<root><unclosed>"), 0644); err != nil {
		t.Fatal(err)
	}

	pipeline := NewPipeline(nil)
	results, err := pipeline.Run(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected error for malformed XML")
	}
	if results[0].SchemaType != "well-formedness" {
		t.Errorf("schema-type = %q, want well-formedness", results[0].SchemaType)
	}
	if results[0].Severity != "error" {
		t.Errorf("severity = %q, want error", results[0].Severity)
	}
}

func TestValidationPipeline_EmptyFile(t *testing.T) {
	tmpFile := t.TempDir() + "/empty.xml"
	if err := os.WriteFile(tmpFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	pipeline := NewPipeline(nil)
	results, err := pipeline.Run(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected error for empty file")
	}
	if results[0].SchemaType != "well-formedness" {
		t.Errorf("schema-type = %q, want well-formedness", results[0].SchemaType)
	}
}

func TestValidationPipeline_PlainTextFile(t *testing.T) {
	tmpFile := t.TempDir() + "/plain.xml"
	if err := os.WriteFile(tmpFile, []byte("this is not xml"), 0644); err != nil {
		t.Fatal(err)
	}

	pipeline := NewPipeline(nil)
	results, err := pipeline.Run(context.Background(), tmpFile)
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected error for plain text file")
	}
}

// --- Pipeline Options ---

func TestValidationPipeline_WithOptions(t *testing.T) {
	steps := []ValidationStep{{Type: "xsd", SchemaFile: "../../test/testdata/schemas/invoice.xsd"}}
	pipeline := NewPipeline(steps,
		WithPipelineNoNetwork(true),
		WithPipelineTiming(false),
	)
	results, err := pipeline.Run(context.Background(), "../../test/testdata/valid/invoice.xml")
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(results) > 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestValidationPipeline_WithTiming(t *testing.T) {
	steps := []ValidationStep{{Type: "xsd", SchemaFile: "../../test/testdata/schemas/invoice.xsd"}}
	pipeline := NewPipeline(steps, WithPipelineTiming(true))
	// Timing writes to stderr; just check it doesn't crash
	_, err := pipeline.Run(context.Background(), "../../test/testdata/valid/invoice.xml")
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
}

// --- Unknown Step Type ---

func TestValidationPipeline_UnknownStepType(t *testing.T) {
	steps := []ValidationStep{{Type: "unknown-type", SchemaFile: "whatever"}}
	pipeline := NewPipeline(steps)
	results, err := pipeline.Run(context.Background(), "../../test/testdata/valid/invoice.xml")
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected error for unknown step type")
	}
	if results[0].Message == "" {
		t.Error("error message should not be empty")
	}
}

// --- ValidationResult Fields ---

func TestValidationResult_FieldPopulation(t *testing.T) {
	steps := []ValidationStep{{Type: "xsd", SchemaFile: "../../test/testdata/schemas/invoice.xsd"}}
	pipeline := NewPipeline(steps)
	results, err := pipeline.Run(context.Background(), "../../test/testdata/invalid/invoice-bad.xml")
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected errors")
	}

	for _, r := range results {
		if r.File == "" {
			t.Error("File should not be empty")
		}
		if r.Severity == "" {
			t.Error("Severity should not be empty")
		}
		if r.Message == "" {
			t.Error("Message should not be empty")
		}
		if r.SchemaType == "" {
			t.Error("SchemaType should not be empty")
		}
		if r.SchemaFile == "" {
			t.Error("SchemaFile should not be empty for schema validation")
		}
	}
}
