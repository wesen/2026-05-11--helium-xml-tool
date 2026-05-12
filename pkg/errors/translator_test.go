package errors

import (
	"testing"
)

func TestExplainError(t *testing.T) {
	expl := ExplainError("cvc-complex-type.2.4.a")
	if expl == nil {
		t.Fatal("expected explanation for cvc-complex-type.2.4.a")
	}
	if expl.Code != "cvc-complex-type.2.4.a" {
		t.Errorf("Code = %q, want %q", expl.Code, "cvc-complex-type.2.4.a")
	}
	if expl.Summary == "" {
		t.Error("Summary should not be empty")
	}
	if len(expl.Causes) == 0 {
		t.Error("Causes should not be empty")
	}
}

func TestExplainErrorUnknown(t *testing.T) {
	expl := ExplainError("nonexistent-code")
	if expl != nil {
		t.Error("expected nil for unknown code")
	}
}

func TestExtractErrorCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"cvc-complex-type.2.4.a: Invalid content", "cvc-complex-type.2.4.a"},
		{"some prefix cvc-elt.1.a suffix", "cvc-elt.1.a"},
		{"no error code here", ""},
		{"cvc-enumeration-valid: bad value", "cvc-enumeration-valid"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExtractErrorCode(tt.input)
			if got != tt.expected {
				t.Errorf("ExtractErrorCode(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestListCodes(t *testing.T) {
	codes := ListCodes()
	if len(codes) == 0 {
		t.Error("ListCodes should return at least one code")
	}
}
