package engine

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDetectSchemaType(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"schema.xsd", "xsd"},
		{"schema.XSD", "xsd"},
		{"dir/schema.Xsd", "xsd"},
		{"schema.rng", "rng"},
		{"schema.RNG", "rng"},
		{"schema.rnc", "rnc"},
		{"schema.sch", "sch"},
		{"schema.dtd", "dtd"},
		{"unknown.txt", "xsd"},  // default fallback
		{"Makefile", "xsd"},      // no extension match
		{"", "xsd"},              // empty path
		{".xsd", "xsd"},          // just extension
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := DetectSchemaType(tt.path)
			if got != tt.expected {
				t.Errorf("DetectSchemaType(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestCollectFiles_SingleFile(t *testing.T) {
	files, err := CollectFiles([]string{"../../test/testdata/valid/invoice.xml"}, false)
	if err != nil {
		t.Fatalf("CollectFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0] != "../../test/testdata/valid/invoice.xml" {
		t.Errorf("file = %q, want %q", files[0], "../../test/testdata/valid/invoice.xml")
	}
}

func TestCollectFiles_MultipleFiles(t *testing.T) {
	files, err := CollectFiles([]string{
		"../../test/testdata/valid/invoice.xml",
		"../../test/testdata/valid/book.xml",
	}, false)
	if err != nil {
		t.Fatalf("CollectFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestCollectFiles_NonexistentFile(t *testing.T) {
	_, err := CollectFiles([]string{"nonexistent.xml"}, false)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestCollectFiles_Directory(t *testing.T) {
	files, err := CollectFiles([]string{"../../test/testdata/valid"}, false)
	if err != nil {
		t.Fatalf("CollectFiles: %v", err)
	}
	if len(files) == 0 {
		t.Error("expected files from directory walk")
	}
	// All results should be .xml files
	for _, f := range files {
		if filepath.Ext(f) != ".xml" {
			t.Errorf("non-XML file returned: %s", f)
		}
	}
}

func TestCollectFiles_EmptyInput(t *testing.T) {
	_, err := CollectFiles(nil, false)
	if err == nil {
		t.Error("expected error for no input files")
	}
}

func TestCollectFiles_Dedup(t *testing.T) {
	files, err := CollectFiles([]string{
		"../../test/testdata/valid/invoice.xml",
		"../../test/testdata/valid/invoice.xml",
	}, false)
	if err != nil {
		t.Fatalf("CollectFiles: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected dedup to 1 file, got %d", len(files))
	}
}

func TestReadInput_File(t *testing.T) {
	data, err := ReadInput("../../test/testdata/valid/invoice.xml")
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty data")
	}
}

func TestReadInput_Nonexistent(t *testing.T) {
	_, err := ReadInput("nonexistent-file.xml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestNewParser_Basic(t *testing.T) {
	parser, cat, err := NewParser(ParseOptions{BaseURI: "test.xml"})
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	if cat != nil {
		t.Error("expected nil catalog when no catalog files")
	}
	_ = parser // just checking construction works
}

func TestNewParser_WithAllOptions(t *testing.T) {
	opts := ParseOptions{
		BaseURI:       "test.xml",
		Recover:       true,
		SubstituteEnt: true,
		LoadDTD:       true,
		ValidateDTD:   true,
		StripBlanks:    true,
		CleanNS:       true,
		MergeCDATA:    true,
		NoNetwork:     true,
		RelaxLimits:   true,
	}
	parser, _, err := NewParser(opts)
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	_ = parser
}

func TestNewParser_BadCatalogFile(t *testing.T) {
	_, _, err := NewParser(ParseOptions{
		CatalogFiles: []string{"nonexistent-catalog.xml"},
	})
	if err == nil {
		t.Error("expected error for bad catalog file")
	}
}

func TestLoadCatalogs_Empty(t *testing.T) {
	_, err := LoadCatalogs(context.Background(), nil)
	if err == nil {
		t.Error("expected error for empty catalog list")
	}
}

func TestLoadCatalogs_Nonexistent(t *testing.T) {
	_, err := LoadCatalogs(context.Background(), []string{"nonexistent.xml"})
	if err == nil {
		t.Error("expected error for nonexistent catalogs")
	}
}

func TestLoadCatalogs_Valid(t *testing.T) {
	cat, err := LoadCatalogs(context.Background(), []string{"../../test/testdata/catalog/catalog.xml"})
	if err != nil {
		t.Fatalf("LoadCatalogs: %v", err)
	}
	if cat == nil {
		t.Error("expected non-nil catalog")
	}
}

func TestParseDocument_Valid(t *testing.T) {
	parser, _, _ := NewParser(ParseOptions{BaseURI: "../../test/testdata/valid/invoice.xml"})
	doc, dur, err := ParseDocument(context.Background(), parser, "../../test/testdata/valid/invoice.xml", false)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if doc == nil {
		t.Fatal("doc is nil")
	}
	if dur != 0 {
		t.Error("expected 0 duration when timing=false")
	}
}

func TestParseDocument_WithTiming(t *testing.T) {
	parser, _, _ := NewParser(ParseOptions{BaseURI: "../../test/testdata/valid/invoice.xml"})
	_, dur, err := ParseDocument(context.Background(), parser, "../../test/testdata/valid/invoice.xml", true)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if dur == 0 {
		t.Error("expected non-zero duration when timing=true")
	}
}

func TestParseDocument_Malformed(t *testing.T) {
	parser, _, _ := NewParser(ParseOptions{BaseURI: "../../test/testdata/malformed/unclosed-tag.xml"})
	doc, _, err := ParseDocument(context.Background(), parser, "../../test/testdata/malformed/unclosed-tag.xml", false)
	if err == nil {
		t.Error("expected error for malformed XML")
	}
	// doc may be nil or partial depending on parser recovery
	_ = doc
}

func TestParseDocument_Nonexistent(t *testing.T) {
	parser, _, _ := NewParser(ParseOptions{})
	_, _, err := ParseDocument(context.Background(), parser, "nonexistent-file-xyz.xml", false)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
