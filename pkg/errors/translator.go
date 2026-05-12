package errors

// ErrorExplanation holds a human-friendly explanation for a W3C error code.
type ErrorExplanation struct {
	Code           string   `json:"code"`
	Summary        string   `json:"summary"`
	Meaning        string   `json:"meaning"`
	Causes         []string `json:"causes"`
	SuggestedFixes []string `json:"suggested-fixes"`
}

// errorCodeDB maps W3C validation error codes to human-friendly explanations.
var errorCodeDB = map[string]ErrorExplanation{
	"cvc-complex-type.2.4.a": {
		Code:    "cvc-complex-type.2.4.a",
		Summary: "Invalid content found at this position",
		Meaning: "The element contains a child that is not allowed, or the child elements are in the wrong order.",
		Causes: []string{
			"Child elements are out of the declared order",
			"A required earlier sibling element is missing",
			"The wrong namespace is being used",
			"The schema loaded is not the schema you expected",
		},
		SuggestedFixes: []string{
			"Check the content model of the parent element in the schema",
			"Verify namespace bindings match the schema's target namespace",
			"Run: xml schema explain SCHEMA PARENT_ELEMENT",
		},
	},
	"cvc-complex-type.2.4.b": {
		Code:    "cvc-complex-type.2.4.b",
		Summary: "Required element missing",
		Meaning: "The content model of this element requires a child element that was not found.",
		Causes: []string{
			"A mandatory child element was omitted",
			"Elements are present but in the wrong namespace",
			"The schema expects more elements than were provided",
		},
		SuggestedFixes: []string{
			"Check the schema's content model for this element",
			"Verify all required children are present and correctly ordered",
			"Run: xml validate FILE --xsd SCHEMA --format pretty",
		},
	},
	"cvc-complex-type.2.3": {
		Code:    "cvc-complex-type.2.3",
		Summary: "Element has text content but the type is element-only",
		Meaning: "The element contains text content, but its type declaration says it should only contain child elements.",
		Causes: []string{
			"Text was accidentally inserted between child elements",
			"The element type should be mixed content but is declared as element-only",
		},
		SuggestedFixes: []string{
			"Remove the text content or change the schema type to mixed content",
			"Check if whitespace is being treated as significant text",
		},
	},
	"cvc-complex-type.3.2.1": {
		Code:    "cvc-complex-type.3.2.1",
		Summary: "Attribute not allowed on element",
		Meaning: "The element has an attribute that is not declared in its type definition.",
		Causes: []string{
			"An undeclared attribute was used",
			"The attribute exists in the wrong namespace",
			"The attribute was removed from the schema but documents still use it",
		},
		SuggestedFixes: []string{
			"Remove the undeclared attribute from the document",
			"Add the attribute to the schema type definition",
			"Check namespace usage",
		},
	},
	"cvc-complex-type.4": {
		Code:    "cvc-complex-type.4",
		Summary: "Required attribute missing",
		Meaning: "The element is missing a required attribute.",
		Causes: []string{
			"A mandatory attribute was omitted from the element",
			"The attribute was misspelled",
			"The attribute is in a different namespace than expected",
		},
		SuggestedFixes: []string{
			"Add the required attribute to the element",
			"Check for typos in attribute names",
		},
	},
	"cvc-datatype-valid.1.2.1": {
		Code:    "cvc-datatype-valid.1.2.1",
		Summary: "Invalid value for declared type",
		Meaning: "The text content of an element or attribute does not match its declared datatype.",
		Causes: []string{
			"A string was provided where a number was expected",
			"A date format is incorrect",
			"A value does not match the allowed pattern",
		},
		SuggestedFixes: []string{
			"Check the datatype of the element/attribute in the schema",
			"Ensure the value matches the expected format",
		},
	},
	"cvc-enumeration-valid": {
		Code:    "cvc-enumeration-valid",
		Summary: "Value not in enumeration",
		Meaning: "The value does not match any of the allowed values in the enumeration facet.",
		Causes: []string{
			"An incorrect or misspelled value was used",
			"The enumeration in the schema does not include this value",
		},
		SuggestedFixes: []string{
			"Check the allowed values in the schema's enumeration",
			"Correct the value to match one of the allowed options",
		},
	},
	"cvc-length-valid": {
		Code:    "cvc-length-valid",
		Summary: "Value length constraint violated",
		Meaning: "The value does not satisfy the length constraint (minLength, maxLength, or length) defined in the schema.",
		Causes: []string{
			"The string is too short or too long",
			"A fixed-length field has the wrong number of characters",
		},
		SuggestedFixes: []string{
			"Check the length restrictions in the schema",
			"Adjust the value to satisfy the length constraint",
		},
	},
	"cvc-pattern-valid": {
		Code:    "cvc-pattern-valid",
		Summary: "Value does not match pattern",
		Meaning: "The value does not match the regular expression pattern defined in the schema.",
		Causes: []string{
			"The value format is incorrect",
			"A typo in the value broke the pattern",
		},
		SuggestedFixes: []string{
			"Check the pattern restriction in the schema",
			"Correct the value to match the expected format",
		},
	},
	"cvc-type.3.1.1": {
		Code:    "cvc-type.3.1.1",
		Summary: "Simple type element has child elements",
		Meaning: "An element declared as a simple type contains child elements, which is not allowed.",
		Causes: []string{
			"Child elements were added to a text-only element",
			"The schema type should be complexType instead of simpleType",
		},
		SuggestedFixes: []string{
			"Remove child elements or change the schema type",
			"Check if the element was incorrectly declared as simpleType",
		},
	},
	"cvc-elt.1.a": {
		Code:    "cvc-elt.1.a",
		Summary: "Element not declared in schema",
		Meaning: "The element name is not found in the schema. There is no declaration for this element.",
		Causes: []string{
			"The element is not part of the schema",
			"The element is in the wrong namespace",
			"The element name is misspelled",
		},
		SuggestedFixes: []string{
			"Check that the element name matches a declaration in the schema",
			"Verify the namespace of the element",
		},
	},
	"cvc-elt.3.1": {
		Code:    "cvc-elt.3.1",
		Summary: "Attribute has invalid value for type",
		Meaning: "An xsi:type attribute value does not derive from the type of the element declaration.",
		Causes: []string{
			"The xsi:type value is not a valid substitution for the declared type",
		},
		SuggestedFixes: []string{
			"Check the type hierarchy in the schema",
			"Ensure the xsi:type value is a valid subtype",
		},
	},
	"cvc-elt.4.2": {
		Code:    "cvc-elt.4.2",
		Summary: "Fixed value constraint violated",
		Meaning: "The element has a fixed value constraint and the actual value differs.",
		Causes: []string{
			"The value differs from the fixed value declared in the schema",
		},
		SuggestedFixes: []string{
			"Set the value to match the fixed constraint",
			"Remove the fixed constraint from the schema if the value should vary",
		},
	},
	"cvc-id.1": {
		Code:    "cvc-id.1",
		Summary: "Duplicate ID value",
		Meaning: "There is another element in the document with the same ID value.",
		Causes: []string{
			"The same ID value was used more than once in the document",
			"Copy-paste duplication of elements",
		},
		SuggestedFixes: []string{
			"Make each ID value unique within the document",
			"Search for duplicate ID values: xml xpath '//*[@id=\"VALUE\"]' FILE",
		},
	},
	"cvc-id.2": {
		Code:    "cvc-id.2",
		Summary: "IDREF refers to non-existent ID",
		Meaning: "An IDREF or IDREFS attribute references an ID value that does not exist in the document.",
		Causes: []string{
			"The referenced element was removed",
			"The IDREF value is misspelled",
		},
		SuggestedFixes: []string{
			"Ensure the referenced ID exists in the document",
			"Check for typos in IDREF values",
		},
	},
}

// ExplainError looks up a W3C error code and returns its explanation.
// Returns nil if the code is not found.
func ExplainError(code string) *ErrorExplanation {
	expl, ok := errorCodeDB[code]
	if !ok {
		return nil
	}
	return &expl
}

// ExtractErrorCode attempts to extract a W3C error code from an error message.
// It looks for patterns like "cvc-*" in the message text.
func ExtractErrorCode(msg string) string {
	// Look for cvc-* patterns
	start := -1
	for i := 0; i < len(msg); i++ {
		if i+3 < len(msg) && msg[i:i+4] == "cvc-" {
			start = i
			break
		}
	}
	if start == -1 {
		return ""
	}

	// Find end of code (ends at space, colon, or paren)
	end := start + 4
	for end < len(msg) && msg[end] != ' ' && msg[end] != ':' && msg[end] != ')' && msg[end] != ',' {
		end++
	}
	return msg[start:end]
}

// ListCodes returns all known error codes.
func ListCodes() []string {
	codes := make([]string, 0, len(errorCodeDB))
	for code := range errorCodeDB {
		codes = append(codes, code)
	}
	return codes
}
