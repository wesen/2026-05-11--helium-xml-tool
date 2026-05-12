package explain_error

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	glazedSchema "github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/spf13/cobra"

	xmlerrors "github.com/go-go-golems/xml/pkg/errors"
)

const slug = glazedSchema.DefaultSlug

// ExplainErrorCommand implements both GlazeCommand and WriterCommand.
// Default mode: human-readable prose (WriterCommand).
// With --with-glaze-output: structured table/JSON/YAML rows.
type ExplainErrorCommand struct {
	*cmds.CommandDescription
}

type ExplainErrorSettings struct {
	Code    string `glazed:"code"`
	List    bool   `glazed:"list"`
	Message string `glazed:"message"`
}

// GlazeCommand interface
var _ cmds.GlazeCommand = (*ExplainErrorCommand)(nil)

// WriterCommand interface
var _ cmds.WriterCommand = (*ExplainErrorCommand)(nil)

func NewExplainErrorCommand() (*ExplainErrorCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &ExplainErrorCommand{CommandDescription: cmds.NewCommandDescription(
		"explain-error",
		cmds.WithShort("Translate cryptic XML validation errors into human prose"),
		cmds.WithLong(`
Translate W3C XML Schema validation error codes (cvc-*) into
human-readable explanations with causes and suggested fixes.

By default, outputs human-readable prose. Use --with-glaze-output
for structured table/JSON/YAML output.

Examples:
  xml explain-error --code cvc-complex-type.2.4.a
  xml explain-error --message "cvc-complex-type.2.4.a: Invalid content..."
  xml explain-error --list
  xml explain-error --code cvc-complex-type.2.4.a --with-glaze-output
`),
		cmds.WithFlags(
			fields.New("code", fields.TypeString,
				fields.WithHelp("W3C error code to explain (e.g., cvc-complex-type.2.4.a)"),
			),
			fields.New("message", fields.TypeString,
				fields.WithHelp("Error message to extract code from"),
			),
			fields.New("list", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("List all known error codes"),
			),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

// RunIntoWriter produces human-readable output (default mode).
func (c *ExplainErrorCommand) RunIntoWriter(
	ctx context.Context,
	vals *values.Values,
	w io.Writer,
) error {
	s := &ExplainErrorSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	// List all codes
	if s.List {
		return writeCodeList(w)
	}

	// Determine code
	code := s.Code
	if code == "" && s.Message != "" {
		code = xmlerrors.ExtractErrorCode(s.Message)
	}
	if code == "" {
		return fmt.Errorf("specify --code, --message, or --list")
	}

	expl := xmlerrors.ExplainError(code)
	if expl == nil {
		fmt.Fprintf(w, "Error code: %s\n\n", code)
		fmt.Fprintf(w, "This error code is not in the database yet.\n")
		fmt.Fprintf(w, "Run `xml explain-error --list` to see all known codes.\n")
		return nil
	}

	return writeExplanation(w, expl)
}

// RunIntoGlazeProcessor produces structured rows (glaze mode).
func (c *ExplainErrorCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	s := &ExplainErrorSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	// List all codes
	if s.List {
		for _, code := range xmlerrors.ListCodes() {
			expl := xmlerrors.ExplainError(code)
			if expl != nil {
				row := types.NewRow(
					types.MRP("code", expl.Code),
					types.MRP("summary", expl.Summary),
				)
				_ = gp.AddRow(ctx, row)
			}
		}
		return nil
	}

	// Determine code
	code := s.Code
	if code == "" && s.Message != "" {
		code = xmlerrors.ExtractErrorCode(s.Message)
	}
	if code == "" {
		return fmt.Errorf("specify --code, --message, or --list")
	}

	expl := xmlerrors.ExplainError(code)
	if expl == nil {
		row := types.NewRow(
			types.MRP("code", code),
			types.MRP("summary", "Unknown error code"),
			types.MRP("meaning", "This error code is not in the database yet"),
		)
		_ = gp.AddRow(ctx, row)
		return nil
	}

	row := types.NewRow(
		types.MRP("code", expl.Code),
		types.MRP("summary", expl.Summary),
		types.MRP("meaning", expl.Meaning),
		types.MRP("causes", strings.Join(expl.Causes, "; ")),
		types.MRP("suggested-fixes", strings.Join(expl.SuggestedFixes, "; ")),
	)
	_ = gp.AddRow(ctx, row)

	return nil
}

// ─── Human-readable writers ──────────────────────────────────────────────────

func writeExplanation(w io.Writer, expl *xmlerrors.ErrorExplanation) error {
	fmt.Fprintf(w, "Error code: %s\n\n", expl.Code)
	fmt.Fprintf(w, "Summary: %s\n\n", expl.Summary)
	fmt.Fprintf(w, "Meaning:\n  %s\n\n", expl.Meaning)

	if len(expl.Causes) > 0 {
		fmt.Fprintf(w, "Likely causes:\n")
		for i, cause := range expl.Causes {
			fmt.Fprintf(w, "  %d. %s\n", i+1, cause)
		}
		fmt.Fprintf(w, "\n")
	}

	if len(expl.SuggestedFixes) > 0 {
		fmt.Fprintf(w, "Suggested fixes:\n")
		for i, fix := range expl.SuggestedFixes {
			fmt.Fprintf(w, "  %d. %s\n", i+1, fix)
		}
	}

	return nil
}

func writeCodeList(w io.Writer) error {
	codes := xmlerrors.ListCodes()
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "CODE\tSUMMARY\n")
	fmt.Fprintf(tw, "----\t-------\n")
	for _, code := range codes {
		expl := xmlerrors.ExplainError(code)
		if expl != nil {
			fmt.Fprintf(tw, "%s\t%s\n", expl.Code, expl.Summary)
		}
	}
	return tw.Flush()
}

// ─── Registration ────────────────────────────────────────────────────────────

func Register(root *cobra.Command) error {
	cmd, err := NewExplainErrorCommand()
	if err != nil {
		return err
	}

	cobraCmd, err := cli.BuildCobraCommandFromCommand(cmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			AppName:           "xml",
			ShortHelpSections: []string{slug},
			MiddlewaresFunc:   cli.CobraCommandDefaultMiddlewares,
		}),
		cli.WithDualMode(true),
	)
	if err != nil {
		return err
	}

	root.AddCommand(cobraCmd)
	return nil
}
