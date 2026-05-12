package engine

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/relaxng"
	"github.com/lestrrat-go/helium/schematron"
	"github.com/lestrrat-go/helium/xsd"
)

// ValidationResult represents a single finding from a validation step.
type ValidationResult struct {
	File         string `json:"file"`
	Severity     string `json:"severity"`      // error, warning, info
	Message      string `json:"message"`
	Line         int    `json:"line"`
	Column       int    `json:"column"`
	SchemaFile   string `json:"schema-file,omitempty"`
	SchemaType   string `json:"schema-type"`   // xsd, rng, sch, dtd, xpath-assert, well-formedness
	Rule         string `json:"rule,omitempty"`
	Context      string `json:"context,omitempty"`
	SuggestedFix string `json:"suggested-fix,omitempty"`
	RawCode      string `json:"raw-code,omitempty"`
}

// ValidationStep represents one stage in a validation pipeline.
type ValidationStep struct {
	Type       string `toml:"type"`                  // xsd, rng, rnc, sch, dtd, xpath-assert
	SchemaFile string `toml:"schema"`                 // path to schema file
	AssertExpr string `toml:"assert,omitempty"`        // XPath expression (for xpath-assert)
	AssertMsg  string `toml:"message,omitempty"`       // Message on failure (for xpath-assert)
}

// PipelineOption configures a ValidationPipeline.
type PipelineOption func(*ValidationPipeline)

// WithPipelineNoNetwork blocks network access during validation.
func WithPipelineNoNetwork(v bool) PipelineOption {
	return func(p *ValidationPipeline) { p.noNetwork = v }
}

// WithPipelineTiming enables timing output to stderr.
func WithPipelineTiming(v bool) PipelineOption {
	return func(p *ValidationPipeline) { p.timing = v }
}

// WithPipelineCatalog sets catalog files for resolution.
func WithPipelineCatalog(files []string) PipelineOption {
	return func(p *ValidationPipeline) { p.catalogFiles = files }
}

// ValidationPipeline runs a sequence of validation steps against a document.
type ValidationPipeline struct {
	steps        []ValidationStep
	catalogFiles []string
	noNetwork    bool
	timing       bool
}

// NewPipeline creates a validation pipeline from steps and options.
func NewPipeline(steps []ValidationStep, opts ...PipelineOption) *ValidationPipeline {
	p := &ValidationPipeline{steps: steps}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Run executes all validation steps against a file and returns results.
// A well-formedness check is always performed first (parsing the document).
// If the document fails to parse, subsequent steps are skipped.
func (p *ValidationPipeline) Run(ctx context.Context, file string) ([]ValidationResult, error) {
	parseOpts := ParseOptions{
		BaseURI:      file,
		NoNetwork:    p.noNetwork,
		CatalogFiles: p.catalogFiles,
	}

	parser, _, err := NewParser(parseOpts)
	if err != nil {
		return nil, err
	}

	doc, parseDur, err := ParseDocument(ctx, parser, file, p.timing)
	if p.timing && parseDur > 0 {
		fmt.Fprintf(os.Stderr, "Parsing %s took %s\n", file, parseDur)
	}

	if err != nil {
		// Parse error → well-formedness violation
		result := ValidationResult{
			File:       file,
			Severity:   "error",
			Message:    err.Error(),
			SchemaType: "well-formedness",
		}
		return []ValidationResult{result}, nil
	}

	if doc == nil {
		return []ValidationResult{{
			File:       file,
			Severity:   "error",
			Message:    "parsing produced nil document",
			SchemaType: "well-formedness",
		}}, nil
	}

	var results []ValidationResult
	for _, step := range p.steps {
		stepResults := p.runStep(ctx, step, file, doc)
		results = append(results, stepResults...)
	}

	return results, nil
}

// runStep dispatches to the appropriate validation method.
func (p *ValidationPipeline) runStep(ctx context.Context, step ValidationStep, file string, doc *helium.Document) []ValidationResult {
	switch step.Type {
	case "xsd":
		return p.runXSDValidation(ctx, step, file, doc)
	case "rng":
		return p.runRNGValidation(ctx, step, file, doc)
	case "sch":
		return p.runSchematronValidation(ctx, step, file, doc)
	case "dtd":
		return p.runDTDValidation(ctx, step, file, doc)
	default:
		return []ValidationResult{{
			File:       file,
			Severity:   "error",
			Message:    fmt.Sprintf("unknown validation step type: %s", step.Type),
			SchemaType: step.Type,
		}}
	}
}

func (p *ValidationPipeline) runXSDValidation(ctx context.Context, step ValidationStep, file string, doc *helium.Document) []ValidationResult {
	var t0 time.Time
	if p.timing {
		t0 = time.Now()
	}

	schema, err := xsd.NewCompiler().CompileFile(ctx, step.SchemaFile)
	if p.timing {
		fmt.Fprintf(os.Stderr, "Compiling XSD schema %s took %s\n", step.SchemaFile, time.Since(t0))
	}
	if err != nil {
		return []ValidationResult{{
			File:       file,
			Severity:   "error",
			Message:    fmt.Sprintf("XSD schema compilation failed: %s", err),
			SchemaType: "xsd",
			SchemaFile: step.SchemaFile,
		}}
	}

	collector := &resultCollector{
		file:       file,
		schemaType: "xsd",
		schemaFile: step.SchemaFile,
	}

	if p.timing {
		t0 = time.Now()
	}
	_ = xsd.NewValidator(schema).ErrorHandler(collector).Validate(ctx, doc)
	if p.timing {
		fmt.Fprintf(os.Stderr, "Validating %s against XSD took %s\n", file, time.Since(t0))
	}

	return collector.results
}

func (p *ValidationPipeline) runRNGValidation(ctx context.Context, step ValidationStep, file string, doc *helium.Document) []ValidationResult {
	var t0 time.Time
	if p.timing {
		t0 = time.Now()
	}

	grammar, err := relaxng.NewCompiler().CompileFile(ctx, step.SchemaFile)
	if p.timing {
		fmt.Fprintf(os.Stderr, "Compiling RELAX NG schema %s took %s\n", step.SchemaFile, time.Since(t0))
	}
	if err != nil {
		return []ValidationResult{{
			File:       file,
			Severity:   "error",
			Message:    fmt.Sprintf("RELAX NG schema compilation failed: %s", err),
			SchemaType: "rng",
			SchemaFile: step.SchemaFile,
		}}
	}

	collector := &resultCollector{
		file:       file,
		schemaType: "rng",
		schemaFile: step.SchemaFile,
	}

	if p.timing {
		t0 = time.Now()
	}
	_ = relaxng.NewValidator(grammar).Label(file).ErrorHandler(collector).Validate(ctx, doc)
	if p.timing {
		fmt.Fprintf(os.Stderr, "Validating %s against RELAX NG took %s\n", file, time.Since(t0))
	}

	return collector.results
}

func (p *ValidationPipeline) runSchematronValidation(ctx context.Context, step ValidationStep, file string, doc *helium.Document) []ValidationResult {
	var t0 time.Time
	if p.timing {
		t0 = time.Now()
	}

	schema, err := schematron.NewCompiler().Label(step.SchemaFile).CompileFile(ctx, step.SchemaFile)
	if p.timing {
		fmt.Fprintf(os.Stderr, "Compiling Schematron schema %s took %s\n", step.SchemaFile, time.Since(t0))
	}
	if err != nil {
		return []ValidationResult{{
			File:       file,
			Severity:   "error",
			Message:    fmt.Sprintf("Schematron schema compilation failed: %s", err),
			SchemaType: "sch",
			SchemaFile: step.SchemaFile,
		}}
	}

	collector := &resultCollector{
		file:       file,
		schemaType: "sch",
		schemaFile: step.SchemaFile,
	}

	if p.timing {
		t0 = time.Now()
	}
	_ = schematron.NewValidator(schema).Label(file).ErrorHandler(collector).Validate(ctx, doc)
	if p.timing {
		fmt.Fprintf(os.Stderr, "Validating %s against Schematron took %s\n", file, time.Since(t0))
	}

	return collector.results
}

func (p *ValidationPipeline) runDTDValidation(ctx context.Context, step ValidationStep, file string, doc *helium.Document) []ValidationResult {
	// DTD validation in helium happens during parsing via ValidateDTD(true).
	// We re-parse the document with DTD validation enabled.
	parseOpts := ParseOptions{
		BaseURI:      file,
		NoNetwork:    p.noNetwork,
		ValidateDTD:  true,
		LoadDTD:      true,
		CatalogFiles: p.catalogFiles,
	}

	parser, _, err := NewParser(parseOpts)
	if err != nil {
		return []ValidationResult{{
			File:       file,
			Severity:   "error",
			Message:    fmt.Sprintf("creating parser for DTD validation: %s", err),
			SchemaType: "dtd",
			SchemaFile: step.SchemaFile,
		}}
	}

	buf, err := ReadInput(file)
	if err != nil {
		return []ValidationResult{{
			File:       file,
			Severity:   "error",
			Message:    fmt.Sprintf("reading file: %s", err),
			SchemaType: "dtd",
		}}
	}

	var t0 time.Time
	if p.timing {
		t0 = time.Now()
	}
	_, parseErr := parser.Parse(ctx, buf)
	if p.timing {
		fmt.Fprintf(os.Stderr, "Validating %s against DTD took %s\n", file, time.Since(t0))
	}

	if parseErr != nil {
		msg := parseErr.Error()
		return []ValidationResult{{
			File:       file,
			Severity:   "error",
			Message:    msg,
			SchemaType: "dtd",
			SchemaFile: step.SchemaFile,
		}}
	}

	return nil
}

// resultCollector implements helium.ErrorHandler to collect validation errors
// as ValidationResult structs.
type resultCollector struct {
	results    []ValidationResult
	file       string
	schemaType string
	schemaFile string
}

func (c *resultCollector) Handle(_ context.Context, err error) {
	c.results = append(c.results, ValidationResult{
		File:       c.file,
		Severity:   "error",
		Message:    err.Error(),
		SchemaType: c.schemaType,
		SchemaFile: c.schemaFile,
	})
}

// DetectSchemaType guesses the schema type from file extension.
func DetectSchemaType(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".xsd"):
		return "xsd"
	case strings.HasSuffix(lower, ".rng"):
		return "rng"
	case strings.HasSuffix(lower, ".rnc"):
		return "rnc"
	case strings.HasSuffix(lower, ".sch"):
		return "sch"
	case strings.HasSuffix(lower, ".dtd"):
		return "dtd"
	default:
		return "xsd" // default
	}
}
