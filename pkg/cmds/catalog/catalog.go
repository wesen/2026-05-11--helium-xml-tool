package catalog

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
	heliumcatalog "github.com/lestrrat-go/helium/catalog"
	"github.com/spf13/cobra"
)

// ---- Init Command ----

type CatalogInitCommand struct {
	*cmds.CommandDescription
}

type CatalogInitSettings struct {
	Dir string `glazed:"dir"`
}

var _ cmds.GlazeCommand = &CatalogInitCommand{}

func NewCatalogInitCommand() (*CatalogInitCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	cmdDesc := cmds.NewCommandDescription(
		"init",
		cmds.WithShort("Initialize a new OASIS XML Catalog"),
		cmds.WithLong(`
Create a new catalog.xml and vendor/xml/ directory in the current project.

Examples:
  xml catalog init
  xml catalog init --dir /path/to/project
`),
		cmds.WithFlags(
			fields.New("dir", fields.TypeString,
				fields.WithDefault("."),
				fields.WithHelp("Directory to create catalog in"),
			),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)

	return &CatalogInitCommand{CommandDescription: cmdDesc}, nil
}

func (c *CatalogInitCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	s := &CatalogInitSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
		return err
	}

	catalogPath := fmt.Sprintf("%s/catalog.xml", s.Dir)
	vendorDir := fmt.Sprintf("%s/vendor/xml", s.Dir)

	// Create vendor directory
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		return fmt.Errorf("creating vendor directory: %w", err)
	}

	// Write catalog.xml
	catalogContent := `<?xml version="1.0"?>
<catalog xmlns="urn:oasis:names:tc:entity:xmlns:xml:catalog">
  <!-- Add your catalog entries here -->
</catalog>
`
	if err := os.WriteFile(catalogPath, []byte(catalogContent), 0644); err != nil {
		return fmt.Errorf("writing catalog.xml: %w", err)
	}

	row := types.NewRow(
		types.MRP("created", catalogPath),
		types.MRP("type", "catalog"),
	)
	_ = gp.AddRow(ctx, row)

	row = types.NewRow(
		types.MRP("created", vendorDir),
		types.MRP("type", "directory"),
	)
	_ = gp.AddRow(ctx, row)

	return nil
}

// ---- Add Command ----

type CatalogAddCommand struct {
	*cmds.CommandDescription
}

type CatalogAddSettings struct {
	Public  string `glazed:"public"`
	System  string `glazed:"system"`
	URI     string `glazed:"uri"`
	Catalog string `glazed:"catalog"`
	File    string `glazed:"file"`
}

var _ cmds.GlazeCommand = &CatalogAddCommand{}

func NewCatalogAddCommand() (*CatalogAddCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	cmdDesc := cmds.NewCommandDescription(
		"add",
		cmds.WithShort("Add an entry to an OASIS XML Catalog"),
		cmds.WithLong(`
Add a public, system, or URI mapping to an OASIS XML Catalog file.

Examples:
  xml catalog add --public "-//OASIS//DTD DocBook V4.5//EN" --file vendor/docbook/docbookx.dtd
  xml catalog add --system "http://example.com/schema.xsd" --file vendor/schemas/schema.xsd
  xml catalog add --uri "http://example.com/resource" --file vendor/resource.xml
`),
		cmds.WithFlags(
			fields.New("public", fields.TypeString,
				fields.WithHelp("Public identifier to map"),
			),
			fields.New("system", fields.TypeString,
				fields.WithHelp("System identifier to map"),
			),
			fields.New("uri", fields.TypeString,
				fields.WithHelp("URI to map"),
			),
			fields.New("catalog", fields.TypeString,
				fields.WithDefault("catalog.xml"),
				fields.WithHelp("Path to catalog.xml file"),
			),
		),
		cmds.WithArguments(
			fields.New("file", fields.TypeString,
				fields.WithHelp("Local file path for the mapping"),
				fields.WithIsArgument(true),
			),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)

	return &CatalogAddCommand{CommandDescription: cmdDesc}, nil
}

func (c *CatalogAddCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	s := &CatalogAddSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
		return err
	}

	if s.File == "" {
		return fmt.Errorf("local file path is required")
	}

	if s.Public == "" && s.System == "" && s.URI == "" {
		return fmt.Errorf("at least one of --public, --system, or --uri is required")
	}

	// Read existing catalog
	data, err := os.ReadFile(s.Catalog)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading catalog: %w", err)
	}

	var newEntry string
	switch {
	case s.Public != "":
		newEntry = fmt.Sprintf("  <public publicId=%q uri=%q/>\n", s.Public, s.File)
	case s.System != "":
		newEntry = fmt.Sprintf("  <system systemId=%q uri=%q/>\n", s.System, s.File)
	case s.URI != "":
		newEntry = fmt.Sprintf("  <uri name=%q uri=%q/>\n", s.URI, s.File)
	}

	// Insert before closing </catalog> tag
	if len(data) > 0 {
		content := string(data)
		closeIdx := strings.LastIndex(content, "</catalog>")
		if closeIdx == -1 {
			return fmt.Errorf("catalog file missing </catalog> closing tag")
		}
		content = content[:closeIdx] + newEntry + content[closeIdx:]
		if err := os.WriteFile(s.Catalog, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing catalog: %w", err)
		}
	} else {
		// Create new catalog
		catalogContent := fmt.Sprintf(`<?xml version="1.0"?>
<catalog xmlns="urn:oasis:names:tc:entity:xmlns:xml:catalog">
%s</catalog>
`, newEntry)
		if err := os.WriteFile(s.Catalog, []byte(catalogContent), 0644); err != nil {
			return fmt.Errorf("writing catalog: %w", err)
		}
	}

	row := types.NewRow(
		types.MRP("catalog", s.Catalog),
		types.MRP("action", "added"),
		types.MRP("local-file", s.File),
	)
	_ = gp.AddRow(ctx, row)
	return nil
}

// ---- Resolve Command ----

type CatalogResolveCommand struct {
	*cmds.CommandDescription
}

type CatalogResolveSettings struct {
	Catalogs string `glazed:"catalogs"`
	Identifier string `glazed:"identifier"`
}

var _ cmds.GlazeCommand = &CatalogResolveCommand{}

func NewCatalogResolveCommand() (*CatalogResolveCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	cmdDesc := cmds.NewCommandDescription(
		"resolve",
		cmds.WithShort("Resolve an identifier through OASIS XML Catalog"),
		cmds.WithLong(`
Resolve a public identifier, system identifier, or URI through
an OASIS XML Catalog.

Examples:
  xml catalog resolve "-//OASIS//DTD DocBook V4.5//EN"
  xml catalog resolve "http://example.com/schema.xsd" --catalogs catalog.xml
`),
		cmds.WithFlags(
			fields.New("catalogs", fields.TypeString,
				fields.WithDefault("catalog.xml"),
				fields.WithHelp("Comma-separated list of catalog files"),
			),
		),
		cmds.WithArguments(
			fields.New("identifier", fields.TypeString,
				fields.WithHelp("Identifier to resolve"),
				fields.WithIsArgument(true),
				fields.WithRequired(true),
			),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)

	return &CatalogResolveCommand{CommandDescription: cmdDesc}, nil
}

func (c *CatalogResolveCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	s := &CatalogResolveSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
		return err
	}

	catFiles := strings.Split(s.Catalogs, ",")
	cat, err := engineLoadCatalogs(ctx, catFiles)
	if err != nil {
		return fmt.Errorf("loading catalogs: %w", err)
	}

	// Try as public ID first, then as system ID
	resolved := cat.Resolve(ctx, s.Identifier, "")
	if resolved == "" {
		resolved = cat.Resolve(ctx, "", s.Identifier)
	}

	row := types.NewRow(
		types.MRP("identifier", s.Identifier),
		types.MRP("resolved", resolved),
	)
	_ = gp.AddRow(ctx, row)
	return nil
}

func engineLoadCatalogs(ctx context.Context, paths []string) (*heliumcatalog.Catalog, error) {
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		cat, err := heliumcatalog.Load(ctx, path)
		if err != nil {
			continue
		}
		return cat, nil
	}
	return nil, fmt.Errorf("no valid catalog found in: %v", paths)
}

// ---- Check Command ----

type CatalogCheckCommand struct {
	*cmds.CommandDescription
}

type CatalogCheckSettings struct {
	Catalogs string `glazed:"catalogs"`
}

var _ cmds.GlazeCommand = &CatalogCheckCommand{}

func NewCatalogCheckCommand() (*CatalogCheckCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	cmdDesc := cmds.NewCommandDescription(
		"check",
		cmds.WithShort("Validate OASIS XML Catalog files"),
		cmds.WithLong(`
Check catalog.xml files for structural validity and verify
that all referenced local files exist.

Examples:
  xml catalog check
  xml catalog check --catalogs catalog.xml,extra-catalog.xml
`),
		cmds.WithFlags(
			fields.New("catalogs", fields.TypeString,
				fields.WithDefault("catalog.xml"),
				fields.WithHelp("Comma-separated list of catalog files"),
			),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)

	return &CatalogCheckCommand{CommandDescription: cmdDesc}, nil
}

func (c *CatalogCheckCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	s := &CatalogCheckSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
		return err
	}

	catFiles := strings.Split(s.Catalogs, ",")
	hasErrors := false

	for _, path := range catFiles {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}

		// Check file exists
		if _, err := os.Stat(path); err != nil {
			row := types.NewRow(
				types.MRP("catalog", path),
				types.MRP("severity", "error"),
				types.MRP("message", fmt.Sprintf("file not found: %s", err)),
			)
			_ = gp.AddRow(ctx, row)
			hasErrors = true
			continue
		}

		// Try to load
		_, err := heliumcatalog.Load(ctx, path)
		if err != nil {
			row := types.NewRow(
				types.MRP("catalog", path),
				types.MRP("severity", "error"),
				types.MRP("message", fmt.Sprintf("parse error: %s", err)),
			)
			_ = gp.AddRow(ctx, row)
			hasErrors = true
			continue
		}

		row := types.NewRow(
			types.MRP("catalog", path),
			types.MRP("severity", "ok"),
			types.MRP("message", "catalog is valid"),
		)
		_ = gp.AddRow(ctx, row)
	}

	if hasErrors {
		return fmt.Errorf("catalog check found errors")
	}
	return nil
}

// Register adds all catalog commands to the root cobra command.
func Register(root *cobra.Command) error {
	groupCmd := &cobra.Command{
		Use:   "catalog",
		Short: "OASIS XML Catalog management",
		Long: `Manage OASIS XML Catalogs for offline, reproducible XML resolution.

Subcommands:
  init    — Create a new catalog.xml and vendor/ directory
  add     — Add a public/system/URI mapping to catalog.xml
  resolve — Resolve an identifier through catalog(s)
  check   — Validate catalog files`,
	}

	initCmd, err := NewCatalogInitCommand()
	if err != nil {
		return err
	}
	initCobra, err := cli.BuildCobraCommandFromCommand(initCmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			AppName:           "xml",
			ShortHelpSections: []string{schema.DefaultSlug},
			MiddlewaresFunc:   cli.CobraCommandDefaultMiddlewares,
		}),
	)
	if err != nil {
		return err
	}
	groupCmd.AddCommand(initCobra)

	addCmd, err := NewCatalogAddCommand()
	if err != nil {
		return err
	}
	addCobra, err := cli.BuildCobraCommandFromCommand(addCmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			AppName:           "xml",
			ShortHelpSections: []string{schema.DefaultSlug},
			MiddlewaresFunc:   cli.CobraCommandDefaultMiddlewares,
		}),
	)
	if err != nil {
		return err
	}
	groupCmd.AddCommand(addCobra)

	resolveCmd, err := NewCatalogResolveCommand()
	if err != nil {
		return err
	}
	resolveCobra, err := cli.BuildCobraCommandFromCommand(resolveCmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			AppName:           "xml",
			ShortHelpSections: []string{schema.DefaultSlug},
			MiddlewaresFunc:   cli.CobraCommandDefaultMiddlewares,
		}),
	)
	if err != nil {
		return err
	}
	groupCmd.AddCommand(resolveCobra)

	checkCmd, err := NewCatalogCheckCommand()
	if err != nil {
		return err
	}
	checkCobra, err := cli.BuildCobraCommandFromCommand(checkCmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			AppName:           "xml",
			ShortHelpSections: []string{schema.DefaultSlug},
			MiddlewaresFunc:   cli.CobraCommandDefaultMiddlewares,
		}),
	)
	if err != nil {
		return err
	}
	groupCmd.AddCommand(checkCobra)

	root.AddCommand(groupCmd)
	return nil
}
