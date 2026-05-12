package sch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	glazedSchema "github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/spf13/cobra"

	"github.com/go-go-golems/xml/pkg/engine"
)

const slug = glazedSchema.DefaultSlug

// ─── ValidateCommand ─────────────────────────────────────────────────────────

type ValidateCommand struct{ *cmds.CommandDescription }
type ValidateSettings struct {
	Schema    string `glazed:"schema"`
	Files     string `glazed:"files"`
	NoNetwork bool  `glazed:"no-network"`
}

var _ cmds.GlazeCommand = (*ValidateCommand)(nil)

func NewValidateCommand() (*ValidateCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &ValidateCommand{CommandDescription: cmds.NewCommandDescription(
		"validate",
		cmds.WithShort("Validate XML against a Schematron schema"),
		cmds.WithLong(`
Validate XML documents against a Schematron schema with structured output.

Schematron validates business rules using XPath assertions.
Each failed assert produces an error; each successful report produces an info.

Examples:
  xml sch validate --schema rules.sch doc.xml
  xml sch validate --schema rules.sch doc.xml --output json
  xml sch validate --schema rules.sch doc.xml --no-network
`),
		cmds.WithFlags(
			fields.New("schema", fields.TypeString, fields.WithHelp("Schematron schema file"), fields.WithRequired(true)),
			fields.New("files", fields.TypeString, fields.WithHelp("XML file to validate"), fields.WithIsArgument(true)),
			fields.New("no-network", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Block network access")),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *ValidateCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &ValidateSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	schema, err := engine.CompileSchematron(ctx, s.Schema)
	if err != nil {
		return err
	}

	var inputPaths []string
	if s.Files != "" {
		inputPaths = append(inputPaths, s.Files)
	}
	if len(inputPaths) == 0 {
		return fmt.Errorf("no XML files to validate")
	}

	hasErrors := false
	for _, file := range inputPaths {
		results, err := engine.SchValidate(ctx, schema, file, s.NoNetwork)
		if err != nil {
			row := types.NewRow(
				types.MRP("file", file),
				types.MRP("severity", "error"),
				types.MRP("message", err.Error()),
				types.MRP("type", "pipeline"),
			)
			if err := gp.AddRow(ctx, row); err != nil {
				return err
			}
			hasErrors = true
			continue
		}

		for _, r := range results {
			row := types.NewRow(
				types.MRP("file", r.File),
				types.MRP("severity", r.Severity),
				types.MRP("type", r.Type),
				types.MRP("message", r.Message),
				types.MRP("pattern", r.Pattern),
				types.MRP("rule", r.Rule),
			)
			if err := gp.AddRow(ctx, row); err != nil {
				return err
			}
			if r.Severity == "error" {
				hasErrors = true
			}
		}
	}

	if hasErrors {
		return fmt.Errorf("schematron validation failed")
	}
	return nil
}

// ─── CompileCommand ──────────────────────────────────────────────────────────

type CompileCommand struct{ *cmds.CommandDescription }
type CompileSettings struct {
	Schema string `glazed:"schema"`
}

var _ cmds.GlazeCommand = (*CompileCommand)(nil)

func NewCompileCommand() (*CompileCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &CompileCommand{CommandDescription: cmds.NewCommandDescription(
		"compile",
		cmds.WithShort("Compile a Schematron schema and show its structure"),
		cmds.WithLong(`
Compile a Schematron schema and display its patterns, rules, and assertions.

Shows the parsed structure without running validation.

Examples:
  xml sch compile --schema rules.sch
  xml sch compile --schema rules.sch --output json
`),
		cmds.WithFlags(
			fields.New("schema", fields.TypeString, fields.WithHelp("Schematron schema file"), fields.WithRequired(true)),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *CompileCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &CompileSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	_, err := engine.CompileSchematron(ctx, s.Schema)
	if err != nil {
		return err
	}

	row := types.NewRow(
		types.MRP("file", s.Schema),
		types.MRP("status", "compiled"),
	)
	return gp.AddRow(ctx, row)
}

// ─── TestCommand ─────────────────────────────────────────────────────────────

type TestCommand struct{ *cmds.CommandDescription }
type TestSettings struct {
	Schema    string `glazed:"schema"`
	Files     string `glazed:"files"`
	NoNetwork bool   `glazed:"no-network"`
}

var _ cmds.GlazeCommand = (*TestCommand)(nil)

func NewTestCommand() (*TestCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &TestCommand{CommandDescription: cmds.NewCommandDescription(
		"test",
		cmds.WithShort("Test documents against a Schematron schema"),
		cmds.WithLong(`
Test XML documents against a Schematron schema, reporting pass/fail per document.

Examples:
  xml sch test --schema rules.sch valid-doc.xml
  xml sch test --schema rules.sch invalid-doc.xml
`),
		cmds.WithFlags(
			fields.New("schema", fields.TypeString, fields.WithHelp("Schematron schema file"), fields.WithRequired(true)),
			fields.New("files", fields.TypeString, fields.WithHelp("XML file to test"), fields.WithIsArgument(true)),
			fields.New("no-network", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Block network access")),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *TestCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &TestSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	schema, err := engine.CompileSchematron(ctx, s.Schema)
	if err != nil {
		return err
	}

	var inputPaths []string
	if s.Files != "" {
		inputPaths = append(inputPaths, s.Files)
	}
	if len(inputPaths) == 0 {
		return fmt.Errorf("no XML files to test")
	}

	for _, file := range inputPaths {
		results, err := engine.SchValidate(ctx, schema, file, s.NoNetwork)
		if err != nil {
			row := types.NewRow(
				types.MRP("file", file),
				types.MRP("result", "ERROR"),
				types.MRP("errors", "1"),
				types.MRP("message", err.Error()),
			)
			if err := gp.AddRow(ctx, row); err != nil {
				return err
			}
			continue
		}

		errorCount := 0
		for _, r := range results {
			if r.Severity == "error" {
				errorCount++
			}
		}

		result := "PASS"
		if errorCount > 0 {
			result = "FAIL"
		}

		row := types.NewRow(
			types.MRP("file", file),
			types.MRP("result", result),
			types.MRP("errors", fmt.Sprintf("%d", errorCount)),
			types.MRP("total", fmt.Sprintf("%d", len(results))),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

// ─── ReportCommand ───────────────────────────────────────────────────────────

type ReportCommand struct{ *cmds.CommandDescription }
type ReportSettings struct {
	Schema    string `glazed:"schema"`
	Files     string `glazed:"files"`
	NoNetwork bool   `glazed:"no-network"`
}

var _ cmds.GlazeCommand = (*ReportCommand)(nil)

func NewReportCommand() (*ReportCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &ReportCommand{CommandDescription: cmds.NewCommandDescription(
		"report",
		cmds.WithShort("Produce a human-readable Schematron validation report"),
		cmds.WithLong(`
Validate XML against a Schematron schema and produce a structured report
with pattern/rule context, test expression, and human-readable messages.

Examples:
  xml sch report --schema rules.sch doc.xml
  xml sch report --schema rules.sch doc.xml --output json
`),
		cmds.WithFlags(
			fields.New("schema", fields.TypeString, fields.WithHelp("Schematron schema file"), fields.WithRequired(true)),
			fields.New("files", fields.TypeString, fields.WithHelp("XML file to validate"), fields.WithIsArgument(true)),
			fields.New("no-network", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Block network access")),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *ReportCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &ReportSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	schema, err := engine.CompileSchematron(ctx, s.Schema)
	if err != nil {
		return err
	}

	var inputPaths []string
	if s.Files != "" {
		inputPaths = append(inputPaths, s.Files)
	}
	if len(inputPaths) == 0 {
		return fmt.Errorf("no XML files specified")
	}

	for _, file := range inputPaths {
		results, err := engine.SchValidate(ctx, schema, file, s.NoNetwork)
		if err != nil {
			row := types.NewRow(
				types.MRP("file", file),
				types.MRP("severity", "error"),
				types.MRP("message", err.Error()),
			)
			if err := gp.AddRow(ctx, row); err != nil {
				return err
			}
			continue
		}

		for _, r := range results {
			row := types.NewRow(
				types.MRP("file", r.File),
				types.MRP("severity", r.Severity),
				types.MRP("type", r.Type),
				types.MRP("pattern", r.Pattern),
				types.MRP("rule", r.Rule),
				types.MRP("message", r.Message),
			)
			if err := gp.AddRow(ctx, row); err != nil {
				return err
			}
		}
	}

	return nil
}

// ─── CoverageCommand ─────────────────────────────────────────────────────────

type CoverageCommand struct{ *cmds.CommandDescription }
type CoverageSettings struct {
	Schema    string `glazed:"schema"`
	Corpus    string `glazed:"corpus"`
	NoNetwork bool   `glazed:"no-network"`
}

var _ cmds.GlazeCommand = (*CoverageCommand)(nil)

func NewCoverageCommand() (*CoverageCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &CoverageCommand{CommandDescription: cmds.NewCommandDescription(
		"coverage",
		cmds.WithShort("Measure Schematron rule coverage against a document corpus"),
		cmds.WithLong(`
Validate all documents in a corpus against a Schematron schema and
report which rules were exercised and which were not.

Examples:
  xml sch coverage --schema rules.sch --corpus testdata/
  xml sch coverage --schema rules.sch --corpus testdata/ --output json
`),
		cmds.WithFlags(
			fields.New("schema", fields.TypeString, fields.WithHelp("Schematron schema file"), fields.WithRequired(true)),
			fields.New("corpus", fields.TypeString, fields.WithHelp("Directory with corpus XML files"), fields.WithRequired(true)),
			fields.New("no-network", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Block network access")),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *CoverageCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &CoverageSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	schema, err := engine.CompileSchematron(ctx, s.Schema)
	if err != nil {
		return err
	}

	// Collect corpus files
	var corpusPaths []string
	err = filepath.WalkDir(s.Corpus, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".xml") {
			corpusPaths = append(corpusPaths, p)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walking corpus: %w", err)
	}
	if len(corpusPaths) == 0 {
		return fmt.Errorf("no XML files found in corpus directory")
	}

	coverage, err := engine.SchCoverageAnalysis(ctx, schema, corpusPaths, s.NoNetwork)
	if err != nil {
		return err
	}

	for _, c := range coverage {
		row := types.NewRow(
			types.MRP("rule", c.Rule),
			types.MRP("hits", c.Hits),
			types.MRP("status", c.Status),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

// ─── Registration ────────────────────────────────────────────────────────────

func Register(root *cobra.Command) error {
	schCmd := &cobra.Command{
		Use:   "sch",
		Short: "Schematron validation, compilation, and coverage",
		Long: `Schematron sub-commands for rule-based validation,
schema compilation, testing, reporting, and coverage analysis.`,
	}

	commands := []struct {
		name string
		cmd  cmds.GlazeCommand
	}{
		{"validate", mustCmd(NewValidateCommand())},
		{"compile", mustCmd(NewCompileCommand())},
		{"test", mustCmd(NewTestCommand())},
		{"report", mustCmd(NewReportCommand())},
		{"coverage", mustCmd(NewCoverageCommand())},
	}

	for _, c := range commands {
		cobraCmd, err := cli.BuildCobraCommandFromCommand(c.cmd,
			cli.WithParserConfig(cli.CobraParserConfig{
				AppName:           "xml",
				ShortHelpSections: []string{slug},
				MiddlewaresFunc:   cli.CobraCommandDefaultMiddlewares,
			}),
		)
		if err != nil {
			return fmt.Errorf("building %s command: %w", c.name, err)
		}
		schCmd.AddCommand(cobraCmd)
	}

	root.AddCommand(schCmd)
	return nil
}

func mustCmd(cmd cmds.GlazeCommand, err error) cmds.GlazeCommand {
	if err != nil {
		panic(err)
	}
	return cmd
}
