package schema

import (
	"context"
	"fmt"

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

// ─── Explain ──────────────────────────────────────────────────────────────────

type ExplainCommand struct{ *cmds.CommandDescription }
type ExplainSettings struct {
	Schema    string `glazed:"schema"`
	Name      string `glazed:"name"`
	Namespace string `glazed:"namespace"`
	Kind      string `glazed:"kind"`
}

var _ cmds.GlazeCommand = (*ExplainCommand)(nil)

func NewExplainCommand() (*ExplainCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &ExplainCommand{CommandDescription: cmds.NewCommandDescription(
		"explain",
		cmds.WithShort("Describe a schema type or element in prose"),
		cmds.WithLong(`
Explain a named type or element from an XSD schema.

Shows the type hierarchy, content model, attributes, facets, and
generates a human-readable prose description.

Examples:
  xml schema explain --schema invoice.xsd --name InvoiceType
  xml schema explain --schema invoice.xsd --name Invoice --kind element
  xml schema explain --schema book.xsd --name BookType --namespace http://example.com/ns
`),
		cmds.WithFlags(
			fields.New("schema", fields.TypeString, fields.WithHelp("XSD schema file"), fields.WithRequired(true)),
			fields.New("name", fields.TypeString, fields.WithHelp("Name of the type or element to explain"), fields.WithRequired(true)),
			fields.New("namespace", fields.TypeString, fields.WithHelp("Namespace of the type/element (default: schema target namespace)")),
			fields.New("kind", fields.TypeString, fields.WithDefault("auto"), fields.WithHelp("Lookup kind: auto, type, element")),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *ExplainCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &ExplainSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	sch, err := engine.CompileSchema(ctx, s.Schema)
	if err != nil {
		return err
	}

	ns := s.Namespace
	if ns == "" {
		ns = sch.TargetNamespace()
	}

	var expl *engine.TypeExplanation
	switch s.Kind {
	case "element":
		expl, err = engine.ExplainElement(sch, s.Name, ns)
	case "type":
		expl, err = engine.ExplainType(sch, s.Name, ns)
	default: // "auto" — try element first, then type
		expl, err = engine.ExplainElement(sch, s.Name, ns)
		if err != nil {
			expl, err = engine.ExplainType(sch, s.Name, ns)
		}
	}
	if err != nil {
		return err
	}

	row := types.NewRow(
		types.MRP("name", expl.Name),
		types.MRP("namespace", expl.Namespace),
		types.MRP("kind", expl.Kind),
		types.MRP("content-type", expl.ContentType),
		types.MRP("base-type", expl.BaseType),
		types.MRP("derivation", expl.Derivation),
		types.MRP("abstract", fmt.Sprintf("%v", expl.Abstract)),
		types.MRP("nillable", fmt.Sprintf("%v", expl.Nillable)),
		types.MRP("description", expl.Description),
		types.MRP("children", fmt.Sprintf("%v", expl.Children)),
		types.MRP("attributes", fmt.Sprintf("%v", expl.Attributes)),
		types.MRP("enumeration", fmt.Sprintf("%v", expl.Enumeration)),
		types.MRP("pattern", expl.Pattern),
		types.MRP("min-occurs", expl.MinOccurs),
		types.MRP("max-occurs", expl.MaxOccurs),
		types.MRP("subst-group", expl.SubstGroup),
	)
	return gp.AddRow(ctx, row)
}

// ─── Graph ───────────────────────────────────────────────────────────────────

type GraphCommand struct{ *cmds.CommandDescription }
type GraphSettings struct {
	Schema string `glazed:"schema"`
	Format string `glazed:"graph-format"`
}

var _ cmds.GlazeCommand = (*GraphCommand)(nil)

func NewGraphCommand() (*GraphCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &GraphCommand{CommandDescription: cmds.NewCommandDescription(
		"graph",
		cmds.WithShort("Visualize schema component dependencies"),
		cmds.WithLong(`
Generate a dependency graph of schema components as Mermaid or DOT.

Shows elements, types, attribute groups, and their relationships
(type-of, base-type, ref, substitution-group).

Examples:
  xml schema graph --schema invoice.xsd --graph-format mermaid
  xml schema graph --schema book.xsd --graph-format dot > schema.dot
  xml schema graph --schema invoice.xsd --graph-format dot | dot -Tpng -o schema.png
`),
		cmds.WithFlags(
			fields.New("schema", fields.TypeString, fields.WithHelp("XSD schema file"), fields.WithRequired(true)),
			fields.New("graph-format", fields.TypeString, fields.WithDefault("mermaid"), fields.WithHelp("Graph output format: mermaid, dot")),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *GraphCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &GraphSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	sch, err := engine.CompileSchema(ctx, s.Schema)
	if err != nil {
		return err
	}

	graph := engine.BuildSchemaGraph(sch)

	var output string
	switch s.Format {
	case "dot":
		output = engine.GraphToDOT(graph)
	default:
		output = engine.GraphToMermaid(graph)
	}

	row := types.NewRow(
		types.MRP("format", s.Format),
		types.MRP("graph", output),
		types.MRP("nodes", fmt.Sprintf("%d", len(graph.Nodes))),
		types.MRP("edges", fmt.Sprintf("%d", len(graph.Edges))),
	)
	return gp.AddRow(ctx, row)
}

// ─── Lint ────────────────────────────────────────────────────────────────────

type LintCommand struct{ *cmds.CommandDescription }
type LintSettings struct {
	Schema string `glazed:"schema"`
}

var _ cmds.GlazeCommand = (*LintCommand)(nil)

func NewLintCommand() (*LintCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &LintCommand{CommandDescription: cmds.NewCommandDescription(
		"lint",
		cmds.WithShort("Detect schema issues: unused types, dead elements, complexity"),
		cmds.WithLong(`
Perform static analysis on an XSD schema to find common issues.

Checks for:
- Unused types (not referenced by any element or other type)
- Abstract types with no concrete derivations
- Deeply nested content models (complexity warning)

Examples:
  xml schema lint --schema invoice.xsd
  xml schema lint --schema book.xsd --output json
`),
		cmds.WithFlags(
			fields.New("schema", fields.TypeString, fields.WithHelp("XSD schema file"), fields.WithRequired(true)),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *LintCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &LintSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	sch, err := engine.CompileSchema(ctx, s.Schema)
	if err != nil {
		return err
	}

	findings := engine.LintSchema(sch)
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

// ─── Refs ────────────────────────────────────────────────────────────────────

type RefsCommand struct{ *cmds.CommandDescription }
type RefsSettings struct {
	Schema    string `glazed:"schema"`
	Name      string `glazed:"name"`
	Namespace string `glazed:"namespace"`
}

var _ cmds.GlazeCommand = (*RefsCommand)(nil)

func NewRefsCommand() (*RefsCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &RefsCommand{CommandDescription: cmds.NewCommandDescription(
		"refs",
		cmds.WithShort("Find all references to a schema type"),
		cmds.WithLong(`
Find all types and elements that reference a named type in the schema.

Shows base-type, type-of, and member-type relationships.

Examples:
  xml schema refs --schema invoice.xsd --name InvoiceType
  xml schema refs --schema book.xsd --name BookType --namespace http://example.com/ns
`),
		cmds.WithFlags(
			fields.New("schema", fields.TypeString, fields.WithHelp("XSD schema file"), fields.WithRequired(true)),
			fields.New("name", fields.TypeString, fields.WithHelp("Type name to search for"), fields.WithRequired(true)),
			fields.New("namespace", fields.TypeString, fields.WithHelp("Namespace of the type (default: schema target namespace)")),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *RefsCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &RefsSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	sch, err := engine.CompileSchema(ctx, s.Schema)
	if err != nil {
		return err
	}

	ns := s.Namespace
	if ns == "" {
		ns = sch.TargetNamespace()
	}

	refs := engine.FindRefs(sch, s.Name, ns)
	for _, r := range refs {
		row := types.NewRow(
			types.MRP("from-name", r.FromName),
			types.MRP("from-kind", r.FromKind),
			types.MRP("to-name", r.ToName),
			types.MRP("to-kind", r.ToKind),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

// ─── Infer ───────────────────────────────────────────────────────────────────

type InferCommand struct{ *cmds.CommandDescription }
type InferSettings struct {
	Files       string `glazed:"files"`
	All         bool   `glazed:"all"`
	TargetNS    string `glazed:"target-namespace"`
	RootElement string `glazed:"root-element"`
	SimpleTypes bool   `glazed:"simple-types"`
	OutputXSD   bool   `glazed:"xsd"`
}

var _ cmds.GlazeCommand = (*InferCommand)(nil)

func NewInferCommand() (*InferCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &InferCommand{CommandDescription: cmds.NewCommandDescription(
		"infer",
		cmds.WithShort("Generate a schema from example XML documents"),
		cmds.WithLong(`
Analyze one or more XML documents and infer an XSD schema.

Infers element names, content models, attributes, cardinalities,
and simple types (if --simple-types is enabled).

Examples:
  xml schema infer --files invoices/ --xsd
  xml schema infer --files doc1.xml --files doc2.xml --xsd --target-namespace http://example.com/ns
  xml schema infer --all --xsd --root-element Invoice
  xml schema infer --files examples/ --output json
`),
		cmds.WithFlags(
			fields.New("files", fields.TypeString, fields.WithHelp("Input XML file or directory"), fields.WithIsArgument(true)),
			fields.New("all", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Walk current directory for XML files")),
			fields.New("target-namespace", fields.TypeString, fields.WithHelp("Target namespace for the generated schema")),
			fields.New("root-element", fields.TypeString, fields.WithHelp("Force root element name (default: auto-detect from documents)")),
			fields.New("simple-types", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Attempt to infer simple types (integer, decimal, boolean)")),
			fields.New("xsd", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Output as XSD instead of structured data")),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *InferCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &InferSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	var inputPaths []string
	if s.Files != "" {
		inputPaths = append(inputPaths, s.Files)
	}

	opts := engine.InferOptions{
		InputFiles:   inputPaths,
		OutputType:   "xsd",
		TargetNS:     s.TargetNS,
		RootElement:  s.RootElement,
		SimpleTypes:  s.SimpleTypes,
	}

	result, err := engine.InferSchema(ctx, opts)
	if err != nil {
		return err
	}

	if s.OutputXSD {
		xsdStr := engine.InferredSchemaToXSD(result)
		row := types.NewRow(
			types.MRP("format", "xsd"),
			types.MRP("schema", xsdStr),
			types.MRP("source-files", result.SourceFiles),
			types.MRP("elements", result.TotalElements),
		)
		return gp.AddRow(ctx, row)
	}

	// Output as structured data
	for _, key := range engine.SortedElementKeys(result.Elements) {
		ie := result.Elements[key]
		row := types.NewRow(
			types.MRP("name", ie.Name),
			types.MRP("namespace", ie.Namespace),
			types.MRP("text-type", ie.TextType),
			types.MRP("has-text", fmt.Sprintf("%v", ie.HasText)),
			types.MRP("count", ie.Count),
			types.MRP("attributes", fmt.Sprintf("%v", ie.Attributes)),
			types.MRP("children", fmt.Sprintf("%v", ie.Children)),
			types.MRP("enum-values", fmt.Sprintf("%v", ie.EnumValues)),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

// ─── Diff ────────────────────────────────────────────────────────────────────

type DiffCommand struct{ *cmds.CommandDescription }
type DiffSettings struct {
	OldSchema string `glazed:"old"`
	NewSchema string `glazed:"new"`
}

var _ cmds.GlazeCommand = (*DiffCommand)(nil)

func NewDiffCommand() (*DiffCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &DiffCommand{CommandDescription: cmds.NewCommandDescription(
		"diff",
		cmds.WithShort("Compare two XSD schemas semantically"),
		cmds.WithLong(`
Perform a semantic comparison of two XSD schema versions.

Identifies added/removed types, changed content models, modified attributes,
and facet changes. Each change is classified as breaking, safe, or warning.

Examples:
  xml schema diff --old schema-v1.xsd --new schema-v2.xsd
  xml schema diff --old v1.xsd --new v2.xsd --output json
`),
		cmds.WithFlags(
			fields.New("old", fields.TypeString, fields.WithHelp("Old schema file"), fields.WithRequired(true)),
			fields.New("new", fields.TypeString, fields.WithHelp("New schema file"), fields.WithRequired(true)),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *DiffCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &DiffSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	diff, err := engine.DiffSchemas(ctx, s.OldSchema, s.NewSchema)
	if err != nil {
		return err
	}

	for _, c := range diff.Changes {
		row := types.NewRow(
			types.MRP("category", c.Category),
			types.MRP("severity", c.Severity),
			types.MRP("component", c.Component),
			types.MRP("detail", c.Detail),
			types.MRP("old-value", c.OldValue),
			types.MRP("new-value", c.NewValue),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	summaryRow := types.NewRow(
		types.MRP("category", "summary"),
		types.MRP("severity", ""),
		types.MRP("component", ""),
		types.MRP("detail", fmt.Sprintf(
			"%d changes: %d breaking, %d safe, %d warning",
			diff.Summary.TotalChanges, diff.Summary.BreakingCount,
			diff.Summary.SafeCount, diff.Summary.WarningCount,
		)),
	)
	return gp.AddRow(ctx, summaryRow)
}

// ─── Breakage ────────────────────────────────────────────────────────────────

type BreakageCommand struct{ *cmds.CommandDescription }
type BreakageSettings struct {
	OldSchema string `glazed:"old"`
	NewSchema string `glazed:"new"`
	Corpus    string `glazed:"corpus"`
	NoNetwork bool   `glazed:"no-network"`
}

var _ cmds.GlazeCommand = (*BreakageCommand)(nil)

func NewBreakageCommand() (*BreakageCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &BreakageCommand{CommandDescription: cmds.NewCommandDescription(
		"breakage",
		cmds.WithShort("Analyze breaking changes between two schema versions"),
		cmds.WithLong(`
Analyze breaking changes between two XSD schemas using a document corpus.

Validates all documents in the corpus against the new schema and correlates
failures with specific schema changes. Each breaking change shows which
documents in the corpus are affected.

Examples:
  xml schema breakage --old v1.xsd --new v2.xsd --corpus testdata/
  xml schema breakage --old v1.xsd --new v2.xsd --output json
`),
		cmds.WithFlags(
			fields.New("old", fields.TypeString, fields.WithHelp("Old schema file"), fields.WithRequired(true)),
			fields.New("new", fields.TypeString, fields.WithHelp("New schema file"), fields.WithRequired(true)),
			fields.New("corpus", fields.TypeString, fields.WithHelp("Directory or file with corpus of valid XML documents")),
			fields.New("no-network", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Block network access during validation")),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *BreakageCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &BreakageSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	var corpusPaths []string
	if s.Corpus != "" {
		corpusPaths = append(corpusPaths, s.Corpus)
	}

	result, err := engine.AnalyzeBreakage(ctx, s.OldSchema, s.NewSchema, corpusPaths, s.NoNetwork)
	if err != nil {
		return err
	}

	for _, bc := range result.BreakingChanges {
		row := types.NewRow(
			types.MRP("category", bc.Change.Category),
			types.MRP("severity", "breaking"),
			types.MRP("component", bc.Change.Component),
			types.MRP("detail", bc.Change.Detail),
			types.MRP("affected-count", bc.AffectedCount),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	for _, sc := range result.SafeChanges {
		row := types.NewRow(
			types.MRP("category", sc.Change.Category),
			types.MRP("severity", "safe"),
			types.MRP("component", sc.Change.Component),
			types.MRP("detail", sc.Change.Detail),
			types.MRP("affected-count", 0),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

// ─── List ────────────────────────────────────────────────────────────────────

type ListCommand struct{ *cmds.CommandDescription }
type ListSettings struct {
	Schema string `glazed:"schema"`
}

var _ cmds.GlazeCommand = (*ListCommand)(nil)

func NewListCommand() (*ListCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &ListCommand{CommandDescription: cmds.NewCommandDescription(
		"list",
		cmds.WithShort("List all named components in an XSD schema"),
		cmds.WithLong(`
List all named elements and types in a compiled XSD schema.

Examples:
  xml schema list --schema invoice.xsd
  xml schema list --schema book.xsd --output json
`),
		cmds.WithFlags(
			fields.New("schema", fields.TypeString, fields.WithHelp("XSD schema file"), fields.WithRequired(true)),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *ListCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &ListSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	sch, err := engine.CompileSchema(ctx, s.Schema)
	if err != nil {
		return err
	}

	components := engine.ListComponents(sch)
	for _, comp := range components {
		row := types.NewRow(
			types.MRP("name", comp.Name),
			types.MRP("namespace", comp.Namespace),
			types.MRP("kind", comp.Kind),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

// ─── Registration ────────────────────────────────────────────────────────────

// Register adds all schema subcommands to the root cobra command.
func Register(root *cobra.Command) error {
	schemaCmd := &cobra.Command{
		Use:   "schema",
		Short: "Schema authoring, analysis, and conversion",
		Long: `Schema workbench commands for XSD analysis, inference,
graphing, linting, and breakage detection.`,
	}

	commands := []struct {
		name string
		cmd  cmds.GlazeCommand
	}{
		{"explain", mustCmd(NewExplainCommand())},
		{"graph", mustCmd(NewGraphCommand())},
		{"lint", mustCmd(NewLintCommand())},
		{"refs", mustCmd(NewRefsCommand())},
		{"infer", mustCmd(NewInferCommand())},
		{"diff", mustCmd(NewDiffCommand())},
		{"breakage", mustCmd(NewBreakageCommand())},
		{"list", mustCmd(NewListCommand())},
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
		schemaCmd.AddCommand(cobraCmd)
	}

	root.AddCommand(schemaCmd)
	return nil
}

func mustCmd(cmd cmds.GlazeCommand, err error) cmds.GlazeCommand {
	if err != nil {
		panic(err)
	}
	return cmd
}

// CompileSchema re-exports engine.CompileSchema for use within this package.
var CompileSchema = engine.CompileSchema
