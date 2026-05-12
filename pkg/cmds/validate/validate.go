package validate

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/spf13/cobra"

	"github.com/go-go-golems/xml/pkg/config"
	"github.com/go-go-golems/xml/pkg/engine"
	xmlerrors "github.com/go-go-golems/xml/pkg/errors"
	"github.com/go-go-golems/xml/pkg/output"
)

// ErrValidationFailed is returned when one or more validation errors are found.
var ErrValidationFailed = fmt.Errorf("validation failed")

// ValidateCommand implements the `xml validate` command.
type ValidateCommand struct {
	*cmds.CommandDescription
}

// ValidateSettings maps flags to typed Go values.
type ValidateSettings struct {
	Schema     string `glazed:"schema"`
	SchemaType string `glazed:"schema-type"`
	Xsd        string `glazed:"xsd"`
	Rng        string `glazed:"rng"`
	Sch        string `glazed:"sch"`
	Dtd        string `glazed:"dtd"`
	Profile    string `glazed:"profile"`
	Format     string `glazed:"format"`
	NoNetwork  bool   `glazed:"no-network"`
	Timing     bool   `glazed:"timing"`
	All        bool   `glazed:"all"`
	Catalogs   string `glazed:"catalogs"`
	Files      string `glazed:"files"`
}

var _ cmds.GlazeCommand = &ValidateCommand{}

func NewValidateCommand() (*ValidateCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}

	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	cmdDesc := cmds.NewCommandDescription(
		"validate",
		cmds.WithShort("Validate XML documents against schemas"),
		cmds.WithLong(`
Validate XML documents against DTD, XSD, RELAX NG, or Schematron schemas.

Supports multi-stage validation pipelines via --profile or by chaining
multiple schema flags. Auto-detects schema type from file extension.

Examples:
  xml validate doc.xml --xsd schema.xsd
  xml validate doc.xml --rng schema.rng
  xml validate doc.xml --sch rules.sch
  xml validate . --all --xsd schema.xsd --format github
  xml validate doc.xml --xsd schema.xsd --sch business-rules.sch
  xml validate . --profile docbook
`),
		cmds.WithFlags(
			fields.New("schema", fields.TypeString,
				fields.WithHelp("Schema file (type auto-detected from extension)"),
			),
			fields.New("schema-type", fields.TypeString,
				fields.WithDefault("auto"),
				fields.WithHelp("Force schema type: auto, xsd, rng, rnc, sch, dtd"),
			),
			fields.New("xsd", fields.TypeString,
				fields.WithHelp("Validate against XSD schema"),
			),
			fields.New("rng", fields.TypeString,
				fields.WithHelp("Validate against RELAX NG schema"),
			),
			fields.New("sch", fields.TypeString,
				fields.WithHelp("Validate against Schematron schema"),
			),
			fields.New("dtd", fields.TypeString,
				fields.WithHelp("Validate against DTD"),
			),
			fields.New("profile", fields.TypeString,
				fields.WithHelp("Named validation profile from xml.toml"),
			),
			fields.New("format", fields.TypeString,
				fields.WithDefault("glazed"),
				fields.WithHelp("Output format: glazed, sarif, github, junit"),
			),
			fields.New("no-network", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Block network access for DTD/entity resolution"),
			),
			fields.New("timing", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Print timing information to stderr"),
			),
			fields.New("all", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Walk current directory for XML files"),
			),
			fields.New("catalogs", fields.TypeString,
				fields.WithHelp("Comma-separated list of OASIS XML Catalog files"),
			),
		),
		cmds.WithArguments(
			fields.New("files", fields.TypeString,
				fields.WithHelp("XML files to validate"),
				fields.WithIsArgument(true),
			),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)

	return &ValidateCommand{CommandDescription: cmdDesc}, nil
}

func (c *ValidateCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	s := &ValidateSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
		return err
	}

	// Build validation steps from flags
	steps := buildSteps(s)

	// Load profile steps if specified
	if s.Profile != "" {
		cfg, err := config.LoadFromDir(".")
		if err != nil {
			return fmt.Errorf("loading xml.toml: %w", err)
		}
		profileSteps, err := cfg.GetProfile(s.Profile)
		if err != nil {
			return err
		}
		steps = append(steps, profileSteps...)
	}

	// Collect input files
	var inputPaths []string
	if s.Files != "" {
		inputPaths = append(inputPaths, s.Files)
	}
	files, err := engine.CollectFiles(inputPaths, s.All)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no XML files to validate")
	}

	// Build catalog file list
	var catalogFiles []string
	if s.Catalogs != "" {
		catalogFiles = strings.Split(s.Catalogs, ",")
	}
	// Also check xml.toml for catalog files
	if cfg, _ := config.LoadFromDir("."); cfg != nil {
		catalogFiles = append(catalogFiles, cfg.CatalogFiles()...)
	}

	// Build pipeline
	pipeline := engine.NewPipeline(steps,
		engine.WithPipelineNoNetwork(s.NoNetwork),
		engine.WithPipelineTiming(s.Timing),
		engine.WithPipelineCatalog(catalogFiles),
	)

	// Run validation and collect results
	hasErrors := false
	var allResults []engine.ValidationResult

	for _, file := range files {
		results, err := pipeline.Run(ctx, file)
		if err != nil {
			allResults = append(allResults, engine.ValidationResult{
				File:       file,
				Severity:   "error",
				Message:    err.Error(),
				SchemaType: "pipeline",
			})
			hasErrors = true
			continue
		}

		for i := range results {
			results[i].RawCode = xmlerrors.ExtractErrorCode(results[i].Message)
			allResults = append(allResults, results[i])
			if results[i].Severity == "error" {
				hasErrors = true
			}
		}
	}

	// Output in the requested format
	var writeErr error
	switch s.Format {
	case "sarif":
		writeErr = output.WriteSARIF(allResults, "xml", "0.1.0", os.Stdout)
	case "github":
		writeErr = output.WriteGitHubAnnotations(allResults, os.Stdout)
	case "junit":
		writeErr = output.WriteJUnit(allResults, os.Stdout)
	default:
		// Glazed table/JSON/YAML output
		for _, r := range allResults {
			row := types.NewRow(
				types.MRP("file", r.File),
				types.MRP("severity", r.Severity),
				types.MRP("message", r.Message),
				types.MRP("line", r.Line),
				types.MRP("column", r.Column),
				types.MRP("schema-type", r.SchemaType),
				types.MRP("schema-file", r.SchemaFile),
				types.MRP("rule", r.Rule),
				types.MRP("context", r.Context),
				types.MRP("raw-code", r.RawCode),
			)
			if err := gp.AddRow(ctx, row); err != nil {
				return err
			}
		}
	}

	if writeErr != nil {
		return writeErr
	}
	if hasErrors {
		return ErrValidationFailed
	}
	return nil
}

// buildSteps converts CLI flags into ValidationStep slices.
func buildSteps(s *ValidateSettings) []engine.ValidationStep {
	var steps []engine.ValidationStep

	if s.Xsd != "" {
		steps = append(steps, engine.ValidationStep{Type: "xsd", SchemaFile: s.Xsd})
	}
	if s.Rng != "" {
		steps = append(steps, engine.ValidationStep{Type: "rng", SchemaFile: s.Rng})
	}
	if s.Sch != "" {
		steps = append(steps, engine.ValidationStep{Type: "sch", SchemaFile: s.Sch})
	}
	if s.Dtd != "" {
		steps = append(steps, engine.ValidationStep{Type: "dtd", SchemaFile: s.Dtd})
	}
	if s.Schema != "" {
		schemaType := s.SchemaType
		if schemaType == "auto" {
			schemaType = engine.DetectSchemaType(s.Schema)
		}
		steps = append(steps, engine.ValidationStep{Type: schemaType, SchemaFile: s.Schema})
	}

	return steps
}

// Register adds the validate command to the root cobra command.
func Register(root *cobra.Command) error {
	cmd, err := NewValidateCommand()
	if err != nil {
		return err
	}

	cobraCmd, err := cli.BuildCobraCommandFromCommand(cmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			AppName:           "xml",
			ShortHelpSections: []string{schema.DefaultSlug},
			MiddlewaresFunc:   cli.CobraCommandDefaultMiddlewares,
		}),
	)
	if err != nil {
		return err
	}

	root.AddCommand(cobraCmd)
	return nil
}
