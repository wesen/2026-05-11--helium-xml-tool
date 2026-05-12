package errors

import (
	"testing"
)

// --- ExplainError ---

func TestExplainError_KnownCodes(t *testing.T) {
	knownCodes := []string{
		"cvc-complex-type.2.4.a",
		"cvc-complex-type.2.4.b",
		"cvc-complex-type.2.3",
		"cvc-complex-type.3.2.1",
		"cvc-complex-type.4",
		"cvc-datatype-valid.1.2.1",
		"cvc-enumeration-valid",
		"cvc-length-valid",
		"cvc-pattern-valid",
		"cvc-type.3.1.1",
		"cvc-elt.1.a",
		"cvc-elt.3.1",
		"cvc-elt.4.2",
		"cvc-id.1",
		"cvc-id.2",
	}

	for _, code := range knownCodes {
		t.Run(code, func(t *testing.T) {
			expl := ExplainError(code)
			if expl == nil {
				t.Fatalf("expected explanation for %q", code)
			}
			if expl.Code != code {
				t.Errorf("Code = %q, want %q", expl.Code, code)
			}
			if expl.Summary == "" {
				t.Error("Summary should not be empty")
			}
			if expl.Meaning == "" {
				t.Error("Meaning should not be empty")
			}
			if len(expl.Causes) == 0 {
				t.Error("Causes should not be empty")
			}
			if len(expl.SuggestedFixes) == 0 {
				t.Error("SuggestedFixes should not be empty")
			}
		})
	}
}

func TestExplainError_Unknown(t *testing.T) {
	expl := ExplainError("cvc-nonexistent-code")
	if expl != nil {
		t.Error("expected nil for unknown code")
	}
}

func TestExplainError_EmptyString(t *testing.T) {
	expl := ExplainError("")
	if expl != nil {
		t.Error("expected nil for empty string")
	}
}

// --- ExtractErrorCode ---

func TestExtractErrorCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"exact code", "cvc-complex-type.2.4.a", "cvc-complex-type.2.4.a"},
		{"code with suffix", "cvc-complex-type.2.4.a: Invalid content", "cvc-complex-type.2.4.a"},
		{"code in middle", "some prefix cvc-elt.1.a suffix", "cvc-elt.1.a"},
		{"no code", "no error code here", ""},
		{"empty string", "", ""},
		{"enumeration", "cvc-enumeration-valid: bad value", "cvc-enumeration-valid"},
		{"with parens", "error (cvc-id.1)", "cvc-id.1"},
		{"with comma", "errors: cvc-elt.4.2, other", "cvc-elt.4.2"},
		{"pattern valid", "cvc-pattern-valid does not match", "cvc-pattern-valid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractErrorCode(tt.input)
			if got != tt.expected {
				t.Errorf("ExtractErrorCode(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestExtractErrorCode_AllKnownCodes(t *testing.T) {
	// Every known code should be extractable from a message containing it
	for _, code := range ListCodes() {
		msg := "Element error: " + code + " something went wrong"
		extracted := ExtractErrorCode(msg)
		if extracted != code {
			t.Errorf("ExtractErrorCode(%q) = %q, want %q", msg, extracted, code)
		}
	}
}

// --- ListCodes ---

func TestListCodes(t *testing.T) {
	codes := ListCodes()
	if len(codes) == 0 {
		t.Fatal("ListCodes should return at least one code")
	}

	// All codes should start with "cvc-"
	for _, code := range codes {
		if len(code) < 4 || code[:4] != "cvc-" {
			t.Errorf("code %q doesn't start with cvc-", code)
		}
	}

	// No duplicates
	seen := map[string]bool{}
	for _, code := range codes {
		if seen[code] {
			t.Errorf("duplicate code: %q", code)
		}
		seen[code] = true
	}
}

func TestListCodes_MatchesDB(t *testing.T) {
	codes := ListCodes()
	if len(codes) != len(errorCodeDB) {
		t.Errorf("ListCodes returned %d codes, but errorCodeDB has %d", len(codes), len(errorCodeDB))
	}
}

// --- ErrorExplanation structure ---

func TestErrorExplanation_Fields(t *testing.T) {
	expl := ExplainError("cvc-complex-type.2.4.a")
	if expl == nil {
		t.Fatal("expected explanation")
	}

	// Code should match
	if expl.Code != "cvc-complex-type.2.4.a" {
		t.Errorf("Code = %q", expl.Code)
	}

	// All fields should be populated for known codes
	if expl.Summary == "" {
		t.Error("Summary empty")
	}
	if expl.Meaning == "" {
		t.Error("Meaning empty")
	}
	if len(expl.Causes) == 0 {
		t.Error("Causes empty")
	}
	if len(expl.SuggestedFixes) == 0 {
		t.Error("SuggestedFixes empty")
	}
}
