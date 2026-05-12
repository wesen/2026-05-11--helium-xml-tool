package explain_error

import (
	"context"
	"fmt"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/spf13/cobra"

	xmlerrors "github.com/go-go-golems/xml/pkg/errors"
)

// ExplainErrorCommand implements the `xml explain-error` command.
type ExplainErrorCommand struct {
	*cmds.CommandDescription
}

// ExplainErrorSettings maps flags to typed Go values.
type ExplainErrorSettings struct {
	Code    string `glazed:"code"`
	List    bool   `glazed:"list"`
	Message string `glazed:"message"`
}

var _ cmds.GlazeCommand = &ExplainErrorCommand{}

func NewExplainErrorCommand() (*ExplainErrorCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	cmdDesc := cmds.NewCommandDescription(
		"explain-error",
		cmds.WithShort("Translate cryptic XML validation errors into human prose"),
		cmds.WithLong(`
Translate W3C XML Schema validation error codes (cvc-*) into
human-readable explanations with causes and suggested fixes.

Examples:
  xml explain-error --code cvc-complex-type.2.4.a
  xml explain-error --message "cvc-complex-type.2.4.a: Invalid content..."
  xml explain-error --list
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
	)

	return &ExplainErrorCommand{CommandDescription: cmdDesc}, nil
}

func (c *ExplainErrorCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	s := &ExplainErrorSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
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
		types.MRP("causes", fmt.Sprintf("%v", expl.Causes)),
		types.MRP("suggested-fixes", fmt.Sprintf("%v", expl.SuggestedFixes)),
	)
	_ = gp.AddRow(ctx, row)

	return nil
}

// Register adds the explain-error command to the root cobra command.
func Register(root *cobra.Command) error {
	cmd, err := NewExplainErrorCommand()
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
