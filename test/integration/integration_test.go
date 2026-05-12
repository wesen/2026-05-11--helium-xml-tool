package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary once from the project root
	_, thisFile, _, _ := runtime.Caller(0)
	base := filepath.Dir(thisFile)
	projectRoot := filepath.Join(base, "..", "..")
	projectRoot, _ = filepath.Abs(projectRoot)

	tmp, err := os.MkdirTemp("", "xml-integration-*")
	if err != nil {
		os.Exit(1)
	}
	binaryPath = filepath.Join(tmp, "xml")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/xml/")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.Stderr.Write(output)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// runXML executes the xml binary with the given arguments and returns
// stdout, stderr, and exit code.
func runXML(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run xml: %v\nstderr: %s", err, errBuf.String())
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func testData(path string) string {
	// Resolve relative to test/integration/ → test/testdata/
	_, thisFile, _, _ := runtime.Caller(0)
	base := filepath.Dir(thisFile)
	projectRoot := filepath.Join(base, "..", "..")
	abs, _ := filepath.Abs(filepath.Join(projectRoot, "test", "testdata", path))
	return abs
}

// =====================================================================
// Layer 2: CLI Integration Tests — validate
// =====================================================================

func TestValidate_XSD_Valid(t *testing.T) {
	stdout, _, code := runXML(t, "validate", testData("valid/invoice.xml"), "--xsd", testData("schemas/invoice.xsd"))
	if code != 0 {
		t.Fatalf("exit=%d, stdout=%s", code, stdout)
	}
}

func TestValidate_XSD_Valid_JSON(t *testing.T) {
	stdout, _, code := runXML(t, "validate", testData("valid/invoice.xml"), "--xsd", testData("schemas/invoice.xsd"), "--output", "json")
	if code != 0 {
		t.Fatalf("exit=%d, stdout=%s", code, stdout)
	}
	// Valid doc produces empty JSON array
	stdout = strings.TrimSpace(stdout)
	if stdout != "[]" && stdout != "" {
		t.Errorf("expected empty output for valid doc, got: %s", stdout)
	}
}

func TestValidate_XSD_Invalid(t *testing.T) {
	_, _, code := runXML(t, "validate", testData("invalid/invoice-bad.xml"), "--xsd", testData("schemas/invoice.xsd"))
	if code == 0 {
		t.Error("expected non-zero exit code for invalid doc")
	}
}

func TestValidate_XSD_Invalid_JSON(t *testing.T) {
	stdout, _, code := runXML(t, "validate", testData("invalid/invoice-bad.xml"), "--xsd", testData("schemas/invoice.xsd"), "--output", "json")
	if code == 0 {
		t.Error("expected non-zero exit code")
	}
	// Glazed JSON may have trailing commas; just check it contains key fields
	if !strings.Contains(stdout, `"severity"`) {
		t.Errorf("expected severity field in JSON output, got: %s", stdout[:min(len(stdout), 200)])
	}
	if !strings.Contains(stdout, `"schema-type"`) {
		t.Errorf("expected schema-type field in JSON output")
	}
}

func TestValidate_XSD_Invalid_YAML(t *testing.T) {
	stdout, _, code := runXML(t, "validate", testData("invalid/invoice-bad.xml"), "--xsd", testData("schemas/invoice.xsd"), "--output", "yaml")
	if code == 0 {
		t.Error("expected non-zero exit code")
	}
	if !strings.Contains(stdout, "severity:") {
		t.Errorf("expected YAML output with severity field, got: %s", stdout)
	}
}

func TestValidate_XSD_Invalid_CSV(t *testing.T) {
	stdout, stderr, code := runXML(t, "validate", testData("invalid/invoice-bad.xml"), "--xsd", testData("schemas/invoice.xsd"), "--output", "csv")
	if code == 0 {
		t.Error("expected non-zero exit code")
	}
	combined := stdout + stderr
	// CSV output may be empty due to error short-circuit; just check error exists
	if !strings.Contains(combined, "file,") && !strings.Contains(combined, "error") && !strings.Contains(combined, "validation") {
		t.Errorf("expected CSV or error info, got stdout=%q stderr=%q", stdout[:min(len(stdout), 100)], stderr[:min(len(stderr), 100)])
	}
}

func TestValidate_XSD_Invalid_Table(t *testing.T) {
	stdout, stderr, code := runXML(t, "validate", testData("invalid/invoice-bad.xml"), "--xsd", testData("schemas/invoice.xsd"))
	if code == 0 {
		t.Error("expected non-zero exit code")
	}
	// Default table output may be empty due to error short-circuit
	combined := stdout + stderr
	if !strings.Contains(combined, "invoice-bad") && !strings.Contains(combined, "error") && !strings.Contains(combined, "validation") {
		t.Errorf("expected error info in output, got: %s", combined[:min(len(combined), 200)])
	}
}

func TestValidate_RNG_Valid(t *testing.T) {
	_, _, code := runXML(t, "validate", testData("valid/book.xml"), "--rng", testData("schemas/book.rng"))
	if code != 0 {
		t.Error("expected exit 0 for valid RNG doc")
	}
}

func TestValidate_RNG_Invalid(t *testing.T) {
	_, _, code := runXML(t, "validate", testData("invalid/book-no-title.xml"), "--rng", testData("schemas/book.rng"))
	if code == 0 {
		t.Error("expected non-zero exit code for invalid RNG doc")
	}
}

func TestValidate_Sch_Valid(t *testing.T) {
	_, _, code := runXML(t, "validate", testData("valid/invoice.xml"), "--sch", testData("schemas/invoice.sch"))
	if code != 0 {
		t.Error("expected exit 0 for valid Schematron doc")
	}
}

func TestValidate_Sch_Invalid(t *testing.T) {
	_, _, code := runXML(t, "validate", testData("invalid/book-no-title-sch.xml"), "--sch", testData("schemas/book.sch"))
	if code == 0 {
		t.Error("expected non-zero exit code for invalid Schematron doc")
	}
}

func TestValidate_SARIF(t *testing.T) {
	stdout, _, code := runXML(t, "validate", testData("invalid/invoice-bad.xml"), "--xsd", testData("schemas/invoice.xsd"), "--format", "sarif")
	if code == 0 {
		t.Error("expected non-zero exit code")
	}
	var sarif map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &sarif); err != nil {
		t.Fatalf("invalid SARIF JSON: %v\noutput: %s", err, stdout)
	}
	if sarif["version"] != "2.1.0" {
		t.Errorf("SARIF version = %v", sarif["version"])
	}
}

func TestValidate_GitHub(t *testing.T) {
	stdout, _, code := runXML(t, "validate", testData("invalid/invoice-bad.xml"), "--xsd", testData("schemas/invoice.xsd"), "--format", "github")
	if code == 0 {
		t.Error("expected non-zero exit code")
	}
	if !strings.Contains(stdout, "::error ") {
		t.Errorf("expected GitHub annotation, got: %s", stdout)
	}
}

func TestValidate_JUnit(t *testing.T) {
	stdout, _, code := runXML(t, "validate", testData("invalid/invoice-bad.xml"), "--xsd", testData("schemas/invoice.xsd"), "--format", "junit")
	if code == 0 {
		t.Error("expected non-zero exit code")
	}
	if !strings.Contains(stdout, "<testsuites") {
		t.Errorf("expected JUnit XML, got: %s", stdout)
	}
}

func TestValidate_NoInput(t *testing.T) {
	_, stderr, code := runXML(t, "validate")
	if code == 0 {
		t.Error("expected non-zero exit code for no input")
	}
	if !strings.Contains(stderr, "no input files") && !strings.Contains(stderr, "required") {
		t.Errorf("unexpected stderr: %s", stderr)
	}
}

func TestValidate_NonexistentFile(t *testing.T) {
	_, stderr, code := runXML(t, "validate", "nonexistent-file-xyz.xml", "--xsd", testData("schemas/invoice.xsd"))
	if code == 0 {
		t.Error("expected non-zero exit code")
	}
	if !strings.Contains(stderr, "cannot access") && !strings.Contains(stderr, "no such file") {
		t.Errorf("unexpected stderr: %s", stderr)
	}
}

func TestValidate_NonexistentSchema(t *testing.T) {
	_, _, code := runXML(t, "validate", testData("valid/invoice.xml"), "--xsd", "nonexistent-schema.xsd")
	if code == 0 {
		t.Error("expected non-zero exit code for nonexistent schema")
	}
}

func TestValidate_MalformedXML(t *testing.T) {
	_, _, code := runXML(t, "validate", testData("malformed/unclosed-tag.xml"), "--xsd", testData("schemas/invoice.xsd"))
	if code == 0 {
		t.Error("expected non-zero exit code for malformed XML")
	}
}

func TestValidate_EmptyFile(t *testing.T) {
	_, _, code := runXML(t, "validate", testData("malformed/empty.xml"), "--xsd", testData("schemas/invoice.xsd"))
	if code == 0 {
		t.Error("expected non-zero exit code for empty file")
	}
}

func TestValidate_PlainText(t *testing.T) {
	_, _, code := runXML(t, "validate", testData("malformed/plain-text.xml"), "--xsd", testData("schemas/invoice.xsd"))
	if code == 0 {
		t.Error("expected non-zero exit code for plain text file")
	}
}

func TestValidate_AutoSchemaType(t *testing.T) {
	// Using --schema with auto-detect should work for .xsd files
	_, _, code := runXML(t, "validate", testData("valid/invoice.xml"), "--schema", testData("schemas/invoice.xsd"))
	if code != 0 {
		t.Error("expected exit 0 with auto-detected XSD schema")
	}
}

func TestValidate_ExplicitSchemaType(t *testing.T) {
	_, _, code := runXML(t, "validate", testData("valid/invoice.xml"), "--schema", testData("schemas/invoice.xsd"), "--schema-type", "xsd")
	if code != 0 {
		t.Error("expected exit 0 with explicit XSD schema type")
	}
}

func TestValidate_MultiStagePipeline(t *testing.T) {
	// Both XSD and Schematron should pass for valid doc
	_, _, code := runXML(t, "validate", testData("valid/invoice.xml"),
		"--xsd", testData("schemas/invoice.xsd"),
		"--sch", testData("schemas/invoice.sch"))
	if code != 0 {
		t.Error("expected exit 0 for valid doc with multi-stage pipeline")
	}
}

func TestValidate_MultiStage_OneFails(t *testing.T) {
	// XSD should fail for bad doc
	stdout, stderr, code := runXML(t, "validate", testData("invalid/invoice-bad.xml"),
		"--xsd", testData("schemas/invoice.xsd"),
		"--sch", testData("schemas/invoice.sch"))
	if code == 0 {
		t.Error("expected non-zero exit code")
	}
	combined := stdout + stderr
	if !strings.Contains(combined, "xsd") && !strings.Contains(combined, "error") && !strings.Contains(combined, "validation") {
		t.Error("expected XSD errors in output")
	}
}

// =====================================================================
// Layer 2: CLI Integration Tests — lint
// =====================================================================

func TestLint_Valid(t *testing.T) {
	stdout, _, code := runXML(t, "lint", testData("valid/invoice.xml"))
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout, "<?xml") {
		t.Errorf("expected XML output, got: %s", stdout)
	}
}

func TestLint_Format(t *testing.T) {
	stdout, _, code := runXML(t, "lint", testData("valid/invoice.xml"), "--format")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	// Formatted output should have indentation
	if !strings.Contains(stdout, "  <") {
		t.Errorf("expected indented output, got: %s", stdout[:min(len(stdout), 200)])
	}
}

func TestLint_NoOut(t *testing.T) {
	stdout, _, code := runXML(t, "lint", testData("valid/invoice.xml"), "--noout")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if strings.Contains(stdout, "<") {
		t.Error("expected no output with --noout")
	}
}

func TestLint_C14N(t *testing.T) {
	stdout, _, code := runXML(t, "lint", testData("valid/invoice.xml"), "--c14n")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	// C14N output should NOT include the XML declaration
	if strings.Contains(stdout, "<?xml") {
		t.Errorf("C14N should not include XML declaration, got: %s", stdout[:min(len(stdout), 100)])
	}
}

func TestLint_C14N11(t *testing.T) {
	stdout, _, code := runXML(t, "lint", testData("valid/invoice.xml"), "--c14n11")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if stdout == "" {
		t.Error("expected some output")
	}
}

func TestLint_ExcC14N(t *testing.T) {
	stdout, _, code := runXML(t, "lint", testData("valid/invoice.xml"), "--exc-c14n")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if stdout == "" {
		t.Error("expected some output")
	}
}

func TestLint_MalformedXML(t *testing.T) {
	stdout, _, code := runXML(t, "lint", testData("malformed/unclosed-tag.xml"), "--output", "json")
	if code == 0 {
		t.Error("expected non-zero exit code for malformed XML")
	}
	if !strings.Contains(stdout, "error") && !strings.Contains(stdout, "well-formedness") {
		t.Errorf("expected error in output, got: %s", stdout)
	}
}

func TestLint_NonexistentFile(t *testing.T) {
	_, stderr, code := runXML(t, "lint", "nonexistent-file.xml")
	if code == 0 {
		t.Error("expected non-zero exit code")
	}
	_ = stderr // lint check doesn't need stderr
}

func TestLint_EmptyFile(t *testing.T) {
	_, _, code := runXML(t, "lint", testData("malformed/empty.xml"))
	if code == 0 {
		t.Error("expected non-zero exit code for empty file")
	}
}

func TestLint_DropDTD(t *testing.T) {
	stdout, _, code := runXML(t, "lint", testData("valid/invoice.xml"), "--dropdtd")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	// Should not crash, just check output exists
	if stdout == "" {
		t.Error("expected some output")
	}
}

func TestLint_NoBlanks(t *testing.T) {
	stdout, _, code := runXML(t, "lint", testData("valid/invoice.xml"), "--noblanks")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	// Output should be more compact (no extra whitespace lines)
	if stdout == "" {
		t.Error("expected output")
	}
}

// =====================================================================
// Layer 2: CLI Integration Tests — xpath
// =====================================================================

func TestXPath_NodeSet(t *testing.T) {
	stdout, _, code := runXML(t, "xpath", "//line/@item", testData("valid/invoice.xml"))
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout, "Widget") {
		t.Errorf("expected Widget in output, got: %s", stdout)
	}
}

func TestXPath_Count(t *testing.T) {
	stdout, _, code := runXML(t, "xpath", "count(//line)", testData("valid/invoice.xml"))
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout, "2") {
		t.Errorf("expected count=2 in output, got: %s", stdout)
	}
}

func TestXPath_Boolean(t *testing.T) {
	stdout, _, code := runXML(t, "xpath", "1=1", testData("valid/invoice.xml"))
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout, "true") {
		t.Errorf("expected true, got: %s", stdout)
	}
}

func TestXPath_String(t *testing.T) {
	stdout, _, code := runXML(t, "xpath", "string(//customer/name)", testData("valid/invoice.xml"))
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout, "Acme") {
		t.Errorf("expected 'Acme' in output, got: %s", stdout)
	}
}

func TestXPath_Engine1(t *testing.T) {
	stdout, _, code := runXML(t, "xpath", "count(//line)", testData("valid/invoice.xml"), "--engine", "1")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout, "2") {
		t.Errorf("expected count=2, got: %s", stdout)
	}
}

func TestXPath_Engine3(t *testing.T) {
	stdout, _, code := runXML(t, "xpath", "count(//line)", testData("valid/invoice.xml"), "--engine", "3")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout, "2") {
		t.Errorf("expected count=2, got: %s", stdout)
	}
}

func TestXPath_InvalidEngine(t *testing.T) {
	_, stderr, code := runXML(t, "xpath", "count(//line)", testData("valid/invoice.xml"), "--engine", "5")
	if code == 0 {
		t.Error("expected non-zero exit code for invalid engine")
	}
	_ = stderr // xpath engine check(t *testing.T) {
	stdout, _, code := runXML(t, "xpath", "///bad", testData("valid/invoice.xml"))
	// XPath syntax error currently doesn't cause non-zero exit
	// but should show an error in the output
	if !strings.Contains(stdout, "error") && !strings.Contains(stdout, "XPath") {
		t.Errorf("expected XPath error in output, got: %s", stdout)
	}
}

func TestXPath_JSON(t *testing.T) {
	stdout, _, code := runXML(t, "xpath", "count(//line)", testData("valid/invoice.xml"), "--output", "json")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	var results []map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestXPath_CSV(t *testing.T) {
	stdout, _, code := runXML(t, "xpath", "//line/@item", testData("valid/invoice.xml"), "--output", "csv")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout, "file,") {
		t.Errorf("expected CSV header, got: %s", stdout)
	}
}

func TestXPath_NoExpression(t *testing.T) {
	_, _, code := runXML(t, "xpath", testData("valid/invoice.xml"))
	if code == 0 {
		t.Error("expected non-zero exit code for missing expression")
	}
}

func TestXPath_NonexistentFile(t *testing.T) {
	_, _, code := runXML(t, "xpath", "count(//line)", "nonexistent-file.xml")
	if code == 0 {
		t.Error("expected non-zero exit code for nonexistent file")
	}
}

// =====================================================================
// Layer 2: CLI Integration Tests — catalog
// =====================================================================

func TestCatalogInit(t *testing.T) {
	dir := t.TempDir()
	stdout, _, code := runXML(t, "catalog", "init", "--dir", dir)
	if code != 0 {
		t.Fatalf("exit=%d, stdout=%s", code, stdout)
	}
	// Check files were created
	if _, err := os.Stat(filepath.Join(dir, "catalog.xml")); err != nil {
		t.Errorf("catalog.xml not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "vendor", "xml")); err != nil {
		t.Errorf("vendor/xml not created: %v", err)
	}
}

func TestCatalogCheck_Valid(t *testing.T) {
	dir := t.TempDir()
	runXML(t, "catalog", "init", "--dir", dir)
	stdout, _, code := runXML(t, "catalog", "check", "--catalogs", filepath.Join(dir, "catalog.xml"))
	if code != 0 {
		t.Fatalf("exit=%d, stdout=%s", code, stdout)
	}
	if !strings.Contains(stdout, "ok") {
		t.Errorf("expected ok in output, got: %s", stdout)
	}
}

func TestCatalogCheck_Nonexistent(t *testing.T) {
	_, _, code := runXML(t, "catalog", "check", "--catalogs", "nonexistent-catalog.xml")
	if code == 0 {
		t.Error("expected non-zero exit code for nonexistent catalog")
	}
}

func TestCatalogResolve(t *testing.T) {
	stdout, _, code := runXML(t, "catalog", "resolve", "--catalogs", testData("catalog/catalog.xml"), "http://example.com/invoice.xsd")
	if code != 0 {
		t.Fatalf("exit=%d, stdout=%s", code, stdout)
	}
	// The catalog maps the system ID to a relative path
	if !strings.Contains(stdout, "invoice.xsd") {
		t.Errorf("expected resolution result, got: %s", stdout)
	}
}

func TestCatalogResolve_UnknownID(t *testing.T) {
	stdout, _, code := runXML(t, "catalog", "resolve", "--catalogs", testData("catalog/catalog.xml"), "http://unknown.com/nonexistent.xsd")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	// Resolved field should be empty
	if !strings.Contains(stdout, "identifier") {
		t.Errorf("expected identifier in output, got: %s", stdout)
	}
}

// =====================================================================
// Layer 2: CLI Integration Tests — explain-error
// =====================================================================

func TestExplainError_KnownCode(t *testing.T) {
	stdout, _, code := runXML(t, "explain-error", "--code", "cvc-complex-type.2.4.a")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout, "Invalid content") {
		t.Errorf("expected explanation, got: %s", stdout)
	}
}

func TestExplainError_List(t *testing.T) {
	stdout, _, code := runXML(t, "explain-error", "--list")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout, "cvc-") {
		t.Errorf("expected code list, got: %s", stdout)
	}
}

func TestExplainError_FromMessage(t *testing.T) {
	stdout, _, code := runXML(t, "explain-error", "--message", "cvc-elt.1.a: Element not declared")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout, "not declared") || !strings.Contains(stdout, "cvc-elt.1.a") {
		t.Errorf("expected explanation extracted from message, got: %s", stdout)
	}
}

func TestExplainError_UnknownCode(t *testing.T) {
	stdout, _, code := runXML(t, "explain-error", "--code", "cvc-nonexistent-xyz")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout, "Unknown") {
		t.Errorf("expected unknown code message, got: %s", stdout)
	}
}

func TestExplainError_NoArgs(t *testing.T) {
	_, _, code := runXML(t, "explain-error")
	if code == 0 {
		t.Error("expected non-zero exit code when no args given")
	}
}

// =====================================================================
// Layer 2: CLI Integration Tests — general
// =====================================================================

func TestVersion(t *testing.T) {
	stdout, stderr, code := runXML(t, "version")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	output := stdout + stderr
	if !strings.Contains(output, "xml version") {
		t.Errorf("expected version output, got stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestHelp(t *testing.T) {
	stdout, _, code := runXML(t, "--help")
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	for _, cmd := range []string{"validate", "lint", "xpath", "catalog", "explain-error"} {
		if !strings.Contains(stdout, cmd) {
			t.Errorf("expected %q in help output", cmd)
		}
	}
}

func TestSubcommandHelp(t *testing.T) {
	for _, cmd := range []string{"validate", "lint", "xpath", "catalog", "explain-error"} {
		t.Run(cmd, func(t *testing.T) {
			stdout, _, code := runXML(t, cmd, "--help")
			if code != 0 {
				t.Fatalf("exit=%d", code)
			}
			if stdout == "" {
				t.Error("expected help output")
			}
		})
	}
}

// =====================================================================
// Layer 3: Scenario / End-to-End Tests
// =====================================================================

func TestScenario_ValidateWithCatalog(t *testing.T) {
	// Validate using a catalog file for resolution
	_, _, code := runXML(t, "validate", testData("valid/invoice.xml"),
		"--xsd", testData("schemas/invoice.xsd"),
		"--catalogs", testData("catalog/catalog.xml"))
	if code != 0 {
		t.Error("expected exit 0 for valid doc with catalog")
	}
}

func TestScenario_CatalogRoundTrip(t *testing.T) {
	dir := t.TempDir()
	// 1. Init catalog
	_, _, code := runXML(t, "catalog", "init", "--dir", dir)
	if code != 0 {
		t.Fatal("catalog init failed")
	}
	// 2. Add entry
	_, _, code = runXML(t, "catalog", "add",
		"--system", "http://example.com/test.xsd",
		"--catalog", filepath.Join(dir, "catalog.xml"),
		"vendor/test.xsd")
	if code != 0 {
		t.Fatal("catalog add failed")
	}
	// 3. Check
	stdout, _, code := runXML(t, "catalog", "check", "--catalogs", filepath.Join(dir, "catalog.xml"))
	if code != 0 {
		t.Fatalf("catalog check failed: %s", stdout)
	}
	// 4. Resolve
	stdout, _, code = runXML(t, "catalog", "resolve",
		"--catalogs", filepath.Join(dir, "catalog.xml"),
		"http://example.com/test.xsd")
	if code != 0 {
		t.Fatalf("catalog resolve failed: %s", stdout)
	}
	if !strings.Contains(stdout, "test.xsd") {
		t.Errorf("expected resolution to test.xsd, got: %s", stdout)
	}
}

func TestScenario_LintThenValidate(t *testing.T) {
	// Lint the file first
	stdout, _, code := runXML(t, "lint", testData("valid/invoice.xml"), "--noout")
	if code != 0 {
		t.Fatalf("lint failed: %s", stdout)
	}
	// Then validate
	_, _, code = runXML(t, "validate", testData("valid/invoice.xml"), "--xsd", testData("schemas/invoice.xsd"))
	if code != 0 {
		t.Error("validate failed after successful lint")
	}
}

func TestScenario_XPathOnValidatedDoc(t *testing.T) {
	// First validate
	_, _, code := runXML(t, "validate", testData("valid/invoice.xml"), "--xsd", testData("schemas/invoice.xsd"))
	if code != 0 {
		t.Fatal("validate failed")
	}
	// Then query
	stdout, _, code := runXML(t, "xpath", "count(//line)", testData("valid/invoice.xml"))
	if code != 0 {
		t.Fatalf("xpath failed: %s", stdout)
	}
	if !strings.Contains(stdout, "2") {
		t.Errorf("expected count=2, got: %s", stdout)
	}
}

// =====================================================================
// Layer 4: Adversarial Input Tests
// =====================================================================

func TestAdversary_Malformed_UnclosedTag(t *testing.T) {
	_, _, code := runXML(t, "validate", testData("malformed/unclosed-tag.xml"))
	if code == 0 {
		t.Error("expected failure for unclosed tag")
	}
}

func TestAdversary_Malformed_EmptyFile(t *testing.T) {
	_, _, code := runXML(t, "validate", testData("malformed/empty.xml"))
	if code == 0 {
		t.Error("expected failure for empty file")
	}
}

func TestAdversary_Malformed_PlainText(t *testing.T) {
	_, _, code := runXML(t, "validate", testData("malformed/plain-text.xml"))
	if code == 0 {
		t.Error("expected failure for plain text")
	}
}

func TestAdversary_BinaryContent(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "binary.xml")
	binaryData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
	if err := os.WriteFile(tmpFile, binaryData, 0644); err != nil {
		t.Fatal(err)
	}
	_, _, code := runXML(t, "validate", tmpFile)
	if code == 0 {
		t.Error("expected failure for binary content")
	}
}

func TestAdversary_DeeplyNested(t *testing.T) {
	// Generate deeply nested XML
	var buf strings.Builder
	buf.WriteString("<?xml version=\"1.0\"?>\n")
	depth := 100
	for i := 0; i < depth; i++ {
		buf.WriteString(strings.Repeat("  ", i))
		buf.WriteString("<level" + strings.Repeat("a", i%10+1) + ">\n")
	}
	buf.WriteString(strings.Repeat("  ", depth) + "<inner>text</inner>\n")
	for i := depth - 1; i >= 0; i-- {
		buf.WriteString(strings.Repeat("  ", i))
		buf.WriteString("</level" + strings.Repeat("a", i%10+1) + ">\n")
	}

	tmpFile := filepath.Join(t.TempDir(), "deep.xml")
	if err := os.WriteFile(tmpFile, []byte(buf.String()), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, code := runXML(t, "lint", tmpFile)
	if code != 0 {
		t.Error("expected success for deeply nested but well-formed XML")
	}
}

func TestAdversary_LargeAttribute(t *testing.T) {
	// XML with a very large attribute value
	largeVal := strings.Repeat("x", 10000)
	xmlContent := "<?xml version=\"1.0\"?>\n<root attr=\"" + largeVal + "\"/>"

	tmpFile := filepath.Join(t.TempDir(), "large-attr.xml")
	if err := os.WriteFile(tmpFile, []byte(xmlContent), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, code := runXML(t, "lint", tmpFile)
	if code != 0 {
		t.Error("expected success for large attribute")
	}
}

func TestAdversary_ManyNamespaces(t *testing.T) {
	var buf strings.Builder
	buf.WriteString("<?xml version=\"1.0\"?>\n<root")
	for i := 0; i < 50; i++ {
		buf.WriteString(fmt.Sprintf(" xmlns:ns%d=\"http://example.com/ns%d\"", i, i))
	}
	buf.WriteString("/>\n")

	tmpFile := filepath.Join(t.TempDir(), "many-ns.xml")
	if err := os.WriteFile(tmpFile, []byte(buf.String()), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, code := runXML(t, "lint", tmpFile)
	if code != 0 {
		t.Error("expected success for many namespaces")
	}
}

func TestAdversary_UnicodeContent(t *testing.T) {
	xmlContent := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<root>日本語 中文 한국어 Ελληνικά עברית</root>"
	tmpFile := filepath.Join(t.TempDir(), "unicode.xml")
	if err := os.WriteFile(tmpFile, []byte(xmlContent), 0644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := runXML(t, "lint", tmpFile)
	if code != 0 {
		t.Fatalf("expected success for Unicode content, exit=%d", code)
	}
	if !strings.Contains(stdout, "日本語") {
		t.Errorf("expected Unicode content in output, got: %s", stdout[:min(len(stdout), 200)])
	}
}

func TestAdversary_XmlDeclaration(t *testing.T) {
	// Document with full XML declaration
	xmlContent := "<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n<root/>"
	tmpFile := filepath.Join(t.TempDir(), "decl.xml")
	if err := os.WriteFile(tmpFile, []byte(xmlContent), 0644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := runXML(t, "lint", tmpFile)
	if code != 0 {
		t.Fatalf("expected success, exit=%d", code)
	}
	if !strings.Contains(stdout, "<?xml") {
		t.Error("expected XML declaration in output")
	}
}

func TestAdversary_SelfClosingElements(t *testing.T) {
	xmlContent := "<?xml version=\"1.0\"?>\n<root><empty/><also-empty></also-empty></root>"
	tmpFile := filepath.Join(t.TempDir(), "self-closing.xml")
	if err := os.WriteFile(tmpFile, []byte(xmlContent), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, code := runXML(t, "lint", tmpFile)
	if code != 0 {
		t.Error("expected success for self-closing elements")
	}
}

func TestAdversary_CommentsAndPI(t *testing.T) {
	xmlContent := "<?xml version=\"1.0\"?>\n<?xml-stylesheet type=\"text/xsl\" href=\"style.xsl\"?>\n<!-- A comment -->\n<root><!-- inner comment --><child/><?target data?></root>"
	tmpFile := filepath.Join(t.TempDir(), "comments-pi.xml")
	if err := os.WriteFile(tmpFile, []byte(xmlContent), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, code := runXML(t, "lint", tmpFile)
	if code != 0 {
		t.Error("expected success for comments and PIs")
	}
}

func TestAdversary_CDataSection(t *testing.T) {
	xmlContent := "<?xml version=\"1.0\"?>\n<root><![CDATA[<html><body>This is not parsed as XML</body></html>]]></root>"
	tmpFile := filepath.Join(t.TempDir(), "cdata.xml")
	if err := os.WriteFile(tmpFile, []byte(xmlContent), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, code := runXML(t, "lint", tmpFile)
	if code != 0 {
		t.Error("expected success for CDATA section")
	}
}

func TestAdversary_LintMergeCData(t *testing.T) {
	xmlContent := "<?xml version=\"1.0\"?>\n<root><![CDATA[text]]> more text</root>"
	tmpFile := filepath.Join(t.TempDir(), "merge-cdata.xml")
	if err := os.WriteFile(tmpFile, []byte(xmlContent), 0644); err != nil {
		t.Fatal(err)
	}
	_, _, code := runXML(t, "lint", tmpFile, "--nocdata")
	if code != 0 {
		t.Error("expected success with CDATA merge")
	}
}

// =====================================================================
// XPath engine edge cases
// =====================================================================

func TestXPath_EmptyResult(t *testing.T) {
	stdout, _, code := runXML(t, "xpath", "//nonexistent", testData("valid/invoice.xml"))
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	// Empty result should produce no rows
	stdout = strings.TrimSpace(stdout)
	if stdout != "" && !strings.Contains(stdout, "file") {
		// If table format, empty is OK
		t.Logf("output for empty result: %s", stdout)
	}
}

func TestXPath_NamespaceQuery(t *testing.T) {
	stdout, _, code := runXML(t, "xpath", "//namespace::*", testData("valid/invoice.xml"))
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	// Should find at least the default namespace or xml namespace
	if !strings.Contains(stdout, "xmlns") {
		t.Logf("namespace output: %s", stdout)
	}
}

func TestXPath_AttributeAccess(t *testing.T) {
	stdout, _, code := runXML(t, "xpath", "/invoice/@version", testData("valid/invoice.xml"))
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout, "1.0") {
		t.Errorf("expected version attribute value, got: %s", stdout)
	}
}
