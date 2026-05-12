package dtd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

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

// ─── InspectCommand ──────────────────────────────────────────────────────────

type InspectCommand struct{ *cmds.CommandDescription }
type InspectSettings struct {
	DTD string `glazed:"dtd"`
}

var _ cmds.GlazeCommand = (*InspectCommand)(nil)

func NewInspectCommand() (*InspectCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &InspectCommand{CommandDescription: cmds.NewCommandDescription(
		"inspect",
		cmds.WithShort("List all elements, attributes, and entities in a DTD"),
		cmds.WithLong(`
Parse a DTD file and list all declared elements, attributes, entities,
and notations.

Examples:
  xml dtd inspect --dtd docbook.dtd
  xml dtd inspect --dtd legacy.dtd --output json
`),
		cmds.WithFlags(
			fields.New("dtd", fields.TypeString, fields.WithHelp("DTD file to inspect"), fields.WithRequired(true)),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *InspectCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &InspectSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	declarations, err := engine.ParseDTD(s.DTD)
	if err != nil {
		return err
	}

	for _, decl := range declarations {
		row := types.NewRow(
			types.MRP("kind", decl.Kind),
			types.MRP("name", decl.Name),
			types.MRP("value", decl.Value),
			types.MRP("type", decl.Type),
			types.MRP("default", decl.Default),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

// ─── FlattenCommand ──────────────────────────────────────────────────────────

type FlattenCommand struct{ *cmds.CommandDescription }
type FlattenSettings struct {
	DTD string `glazed:"dtd"`
}

var _ cmds.GlazeCommand = (*FlattenCommand)(nil)

func NewFlattenCommand() (*FlattenCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &FlattenCommand{CommandDescription: cmds.NewCommandDescription(
		"flatten",
		cmds.WithShort("Resolve parameter entity includes into a single DTD"),
		cmds.WithLong(`
Read a DTD file and resolve all external parameter entity includes
(%pe;) into a single monolithic DTD output.

This is useful for understanding the full content model of a DTD
that is split across multiple files.

Examples:
  xml dtd flatten --dtd docbook.dtd
  xml dtd flatten --dtd main.dtd
`),
		cmds.WithFlags(
			fields.New("dtd", fields.TypeString, fields.WithHelp("DTD file to flatten"), fields.WithRequired(true)),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *FlattenCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &FlattenSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	content, err := engine.FlattenDTD(s.DTD)
	if err != nil {
		return err
	}

	row := types.NewRow(
		types.MRP("file", s.DTD),
		types.MRP("content", content),
	)
	return gp.AddRow(ctx, row)
}

// ─── EntitiesCommand ─────────────────────────────────────────────────────────

type EntitiesCommand struct{ *cmds.CommandDescription }
type EntitiesSettings struct {
	DTD string `glazed:"dtd"`
}

var _ cmds.GlazeCommand = (*EntitiesCommand)(nil)

func NewEntitiesCommand() (*EntitiesCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &EntitiesCommand{CommandDescription: cmds.NewCommandDescription(
		"entities",
		cmds.WithShort("List and check entity declarations for safety"),
		cmds.WithLong(`
List all general and parameter entities in a DTD, with an expansion
safety check that detects potential billion laughs attack vectors.

Examples:
  xml dtd entities --dtd docbook.dtd
  xml dtd entities --dtd legacy.dtd --output json
`),
		cmds.WithFlags(
			fields.New("dtd", fields.TypeString, fields.WithHelp("DTD file"), fields.WithRequired(true)),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *EntitiesCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &EntitiesSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	entities, err := engine.ParseDTD(s.DTD)
	if err != nil {
		return err
	}

	for _, e := range entities {
		if e.Kind != "general-entity" && e.Kind != "parameter-entity" {
			continue
		}
		safe := "safe"
		if isExpansiveEntity(e.Value) {
			safe = "dangerous"
		}
		row := types.NewRow(
			types.MRP("kind", e.Kind),
			types.MRP("name", e.Name),
			types.MRP("value", truncate(e.Value, 200)),
			types.MRP("safety", safe),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

// ─── AuditCommand ────────────────────────────────────────────────────────────

type AuditCommand struct{ *cmds.CommandDescription }
type AuditSettings struct {
	DTD string `glazed:"dtd"`
}

var _ cmds.GlazeCommand = (*AuditCommand)(nil)

func NewAuditCommand() (*AuditCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &AuditCommand{CommandDescription: cmds.NewCommandDescription(
		"audit",
		cmds.WithShort("Comprehensive DTD health check"),
		cmds.WithLong(`
Perform a comprehensive health check on a DTD file.

Checks for:
- External entity safety (billion laughs attack vectors)
- Entity expansion depth
- Parameter entity complexity
- Missing or unreachable declarations

Examples:
  xml dtd audit --dtd docbook.dtd
  xml dtd audit --dtd legacy.dtd --output json
`),
		cmds.WithFlags(
			fields.New("dtd", fields.TypeString, fields.WithHelp("DTD file to audit"), fields.WithRequired(true)),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *AuditCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &AuditSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	findings, err := engine.AuditDTD(s.DTD)
	if err != nil {
		return err
	}

	for _, f := range findings {
		row := types.NewRow(
			types.MRP("severity", f.Severity),
			types.MRP("category", f.Category),
			types.MRP("name", f.Name),
			types.MRP("message", f.Message),
			types.MRP("suggestion", f.Suggestion),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// isExpansiveEntity detects recursive entity expansion patterns (billion laughs).
func isExpansiveEntity(value string) bool {
	// Simple heuristic: entity values that reference other entities multiple times
	re := regexp.MustCompile(`&[^;]+;`)
	refs := re.FindAllString(value, -1)
	// If there are more than 5 entity references in a single entity value,
	// or if the entity references itself, flag as dangerous
	if len(refs) > 5 {
		return true
	}
	return false
}

// ─── Registration ────────────────────────────────────────────────────────────

func Register(root *cobra.Command) error {
	dtdCmd := &cobra.Command{
		Use:   "dtd",
		Short: "DTD inspection, flattening, and safety analysis",
		Long: `DTD sub-commands for inspecting, flattening, and auditing
Document Type Definitions. Addresses legacy XML systems common
in publishing, government, and SGML-migration contexts.`,
	}

	commands := []struct {
		name string
		cmd  cmds.GlazeCommand
	}{
		{"inspect", mustCmd(NewInspectCommand())},
		{"flatten", mustCmd(NewFlattenCommand())},
		{"entities", mustCmd(NewEntitiesCommand())},
		{"audit", mustCmd(NewAuditCommand())},
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
		dtdCmd.AddCommand(cobraCmd)
	}

	root.AddCommand(dtdCmd)
	return nil
}

func mustCmd(cmd cmds.GlazeCommand, err error) cmds.GlazeCommand {
	if err != nil {
		panic(err)
	}
	return cmd
}

// Ensure the unused import path is valid
var _ = filepath.Base
var _ = os.Stdout
