package engine

import (
	"testing"
)

func TestDetectSchemaType(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"schema.xsd", "xsd"},
		{"schema.XSD", "xsd"},
		{"schema.rng", "rng"},
		{"schema.rnc", "rnc"},
		{"schema.sch", "sch"},
		{"schema.dtd", "dtd"},
		{"unknown.txt", "xsd"}, // default
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

func TestCollectFiles(t *testing.T) {
	// Single file
	files, err := CollectFiles([]string{"../../test/testdata/valid/invoice.xml"}, false)
	if err != nil {
		t.Fatalf("CollectFiles: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}

	// Nonexistent file
	_, err = CollectFiles([]string{"nonexistent.xml"}, false)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
