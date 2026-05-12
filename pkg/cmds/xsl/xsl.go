package xsl

import (
	"context"
	"fmt"
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

// ─── RunCommand ─────────────────────────────────────────────────────────────

type RunCommand struct{ *cmds.CommandDescription }
type RunSettings struct {
	Stylesheet string `glazed:"stylesheet"`
	Input      string `glazed:"input"`
	Output     string `glazed:"xml-output"`
	Params     string `glazed:"params"` // comma-separated key=value
	NoNetwork  bool   `glazed:"no-network"`
}

var _ cmds.GlazeCommand = (*RunCommand)(nil)

func NewRunCommand() (*RunCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &RunCommand{CommandDescription: cmds.NewCommandDescription(
		"run",
		cmds.WithShort("Execute an XSLT transformation"),
		cmds.WithLong(`
Run an XSLT transformation on an XML document.

Supports XSLT 3.0 parameters and multiple output destinations.

Examples:
  xml xsl run --stylesheet transform.xsl --input doc.xml
  xml xsl run --stylesheet transform.xsl --input doc.xml --output result.xml
  xml xsl run --stylesheet transform.xsl --input doc.xml --params "key1=val1,key2=val2"
`),
		cmds.WithFlags(
			fields.New("stylesheet", fields.TypeString, fields.WithHelp("XSLT stylesheet file"), fields.WithRequired(true)),
			fields.New("input", fields.TypeString, fields.WithHelp("Input XML file"), fields.WithRequired(true)),
			fields.New("xml-output", fields.TypeString, fields.WithHelp("Output file path (default: stdout)")),
			fields.New("params", fields.TypeString, fields.WithHelp("Comma-separated key=value parameters")),
			fields.New("no-network", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Block network access")),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *RunCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &RunSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	// Parse params
	params := map[string]string{}
	if s.Params != "" {
		for _, pair := range splitOnCommas(s.Params) {
			kv := splitOnEquals(pair)
			if len(kv) == 2 {
				params[kv[0]] = kv[1]
			}
		}
	}

	result, err := engine.XSLTRun(ctx, s.Stylesheet, s.Input, params, s.Output, s.NoNetwork)
	if err != nil {
		return err
	}

	// If no output file specified, include result in the row
	outputDest := "file"
	if s.Output == "" {
		outputDest = "stdout"
	}

	row := types.NewRow(
		types.MRP("stylesheet", s.Stylesheet),
		types.MRP("input", s.Input),
		types.MRP("output-dest", outputDest),
		types.MRP("output", s.Output),
		types.MRP("result-size", fmt.Sprintf("%d", result.ResultSize)),
	)
	if s.Output == "" && result != nil {
		row.Set("result", result.Output)
	}
	return gp.AddRow(ctx, row)
}

// ─── ListCommand ─────────────────────────────────────────────────────────────

type ListCommand struct{ *cmds.CommandDescription }
type ListSettings struct {
	Stylesheet string `glazed:"stylesheet"`
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
		cmds.WithShort("List templates, functions, and variables in a stylesheet"),
		cmds.WithLong(`
Parse an XSLT stylesheet and list all declared templates, functions,
global variables, and parameters.

Examples:
  xml xsl list --stylesheet transform.xsl
  xml xsl list --stylesheet transform.xsl --output json
`),
		cmds.WithFlags(
			fields.New("stylesheet", fields.TypeString, fields.WithHelp("XSLT stylesheet file"), fields.WithRequired(true)),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *ListCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &ListSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	templates, functions, variables, _, err := engine.ParseStylesheet(ctx, s.Stylesheet)
	if err != nil {
		return err
	}

	for _, t := range templates {
		row := types.NewRow(
			types.MRP("kind", "template"),
			types.MRP("name", t.Name),
			types.MRP("match", t.Match),
			types.MRP("mode", t.Mode),
			types.MRP("visibility", t.Visibility),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	for _, f := range functions {
		row := types.NewRow(
			types.MRP("kind", "function"),
			types.MRP("name", f.Name),
			types.MRP("match", ""),
			types.MRP("mode", ""),
			types.MRP("visibility", ""),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	for _, v := range variables {
		row := types.NewRow(
			types.MRP("kind", v.Type),
			types.MRP("name", v.Name),
			types.MRP("match", ""),
			types.MRP("mode", ""),
			types.MRP("visibility", ""),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

// ─── RefsCommand ────────────────────────────────────────────────────────────

type RefsCommand struct{ *cmds.CommandDescription }
type RefsSettings struct {
	Stylesheet string `glazed:"stylesheet"`
	Name       string `glazed:"name"`
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
		cmds.WithShort("Find references to a named template or function"),
		cmds.WithLong(`
Find all references to a named template or function in a stylesheet.

Searches template names, match patterns, and function names.

Examples:
  xml xsl refs --stylesheet transform.xsl --name main
  xml xsl refs --stylesheet transform.xsl --name "f:normalize"
`),
		cmds.WithFlags(
			fields.New("stylesheet", fields.TypeString, fields.WithHelp("XSLT stylesheet file"), fields.WithRequired(true)),
			fields.New("name", fields.TypeString, fields.WithHelp("Template or function name to search for"), fields.WithRequired(true)),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *RefsCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &RefsSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	templates, functions, variables, _, err := engine.ParseStylesheet(ctx, s.Stylesheet)
	if err != nil {
		return err
	}

	searchName := s.Name

	// Search templates
	for _, t := range templates {
		if t.Name == searchName {
			row := types.NewRow(
				types.MRP("from", t.Name),
				types.MRP("from-kind", "template"),
				types.MRP("to", searchName),
				types.MRP("to-kind", "name-match"),
			)
			if err := gp.AddRow(ctx, row); err != nil {
				return err
			}
		}
		if containsString(t.Match, searchName) {
			row := types.NewRow(
				types.MRP("from", t.Name+" "+t.Match),
				types.MRP("from-kind", "template"),
				types.MRP("to", searchName),
				types.MRP("to-kind", "match-ref"),
			)
			if err := gp.AddRow(ctx, row); err != nil {
				return err
			}
		}
	}

	// Search functions
	for _, f := range functions {
		if f.Name == searchName {
			row := types.NewRow(
				types.MRP("from", f.Name),
				types.MRP("from-kind", "function"),
				types.MRP("to", searchName),
				types.MRP("to-kind", "definition"),
			)
			if err := gp.AddRow(ctx, row); err != nil {
				return err
			}
		}
	}

	// Search variables
	for _, v := range variables {
		if v.Name == searchName {
			row := types.NewRow(
				types.MRP("from", v.Name),
				types.MRP("from-kind", v.Type),
				types.MRP("to", searchName),
				types.MRP("to-kind", "definition"),
			)
			if err := gp.AddRow(ctx, row); err != nil {
				return err
			}
		}
	}

	return nil
}

// ─── UnusedCommand ───────────────────────────────────────────────────────────

type UnusedCommand struct{ *cmds.CommandDescription }
type UnusedSettings struct {
	Stylesheet string `glazed:"stylesheet"`
}

var _ cmds.GlazeCommand = (*UnusedCommand)(nil)

func NewUnusedCommand() (*UnusedCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &UnusedCommand{CommandDescription: cmds.NewCommandDescription(
		"unused",
		cmds.WithShort("Find unused templates and functions in a stylesheet"),
		cmds.WithLong(`
Identify templates and functions that are never referenced.

Match-only templates are always potentially used (invoked by apply-templates).
Named templates that no call-template references are flagged as unused.

Examples:
  xml xsl unused --stylesheet transform.xsl
  xml xsl unused --stylesheet transform.xsl --output json
`),
		cmds.WithFlags(
			fields.New("stylesheet", fields.TypeString, fields.WithHelp("XSLT stylesheet file"), fields.WithRequired(true)),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *UnusedCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &UnusedSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	templates, functions, variables, _, err := engine.ParseStylesheet(ctx, s.Stylesheet)
	if err != nil {
		return err
	}

	unused := engine.FindUnusedTemplates(templates, functions, variables)
	for _, t := range unused {
		row := types.NewRow(
			types.MRP("kind", "template"),
			types.MRP("name", t.Name),
			types.MRP("match", t.Match),
			types.MRP("mode", t.Mode),
			types.MRP("suggestion", "Remove or connect to a call-site"),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

// ─── GraphCommand ────────────────────────────────────────────────────────────

type GraphCommand struct{ *cmds.CommandDescription }
type GraphSettings struct {
	Stylesheet string `glazed:"stylesheet"`
	Format     string `glazed:"graph-format"`
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
		cmds.WithShort("Visualize stylesheet component dependencies"),
		cmds.WithLong(`
Generate a dependency graph of stylesheet components as Mermaid or DOT.

Shows templates, functions, variables, and import/include relationships.

Examples:
  xml xsl graph --stylesheet transform.xsl --graph-format mermaid
  xml xsl graph --stylesheet transform.xsl --graph-format dot
`),
		cmds.WithFlags(
			fields.New("stylesheet", fields.TypeString, fields.WithHelp("XSLT stylesheet file"), fields.WithRequired(true)),
			fields.New("graph-format", fields.TypeString, fields.WithDefault("mermaid"), fields.WithHelp("Graph format: mermaid, dot")),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *GraphCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &GraphSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	templates, functions, variables, imports, err := engine.ParseStylesheet(ctx, s.Stylesheet)
	if err != nil {
		return err
	}

	analysis := &engine.XSLTStaticAnalysis{
		Templates: templates,
		Functions: functions,
		Variables: variables,
		Imports:  imports,
	}

	graph := engine.BuildXSLTGraph(analysis)

	var output string
	switch s.Format {
	case "dot":
		output = xsltGraphToDOT(graph)
	default:
		output = xsltGraphToMermaid(graph)
	}

	row := types.NewRow(
		types.MRP("format", s.Format),
		types.MRP("graph", output),
		types.MRP("nodes", fmt.Sprintf("%d", len(graph.Nodes))),
		types.MRP("edges", fmt.Sprintf("%d", len(graph.Edges))),
	)
	return gp.AddRow(ctx, row)
}

// ─── DepsCommand ────────────────────────────────────────────────────────────

type DepsCommand struct{ *cmds.CommandDescription }
type DepsSettings struct {
	Stylesheet string `glazed:"stylesheet"`
}

var _ cmds.GlazeCommand = (*DepsCommand)(nil)

func NewDepsCommand() (*DepsCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}
	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	return &DepsCommand{CommandDescription: cmds.NewCommandDescription(
		"deps",
		cmds.WithShort("List import and include dependencies"),
		cmds.WithLong(`
List all import and include dependencies of an XSLT stylesheet.

Examples:
  xml xsl deps --stylesheet transform.xsl
  xml xsl deps --stylesheet transform.xsl --output json
`),
		cmds.WithFlags(
			fields.New("stylesheet", fields.TypeString, fields.WithHelp("XSLT stylesheet file"), fields.WithRequired(true)),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)}, nil
}

func (c *DepsCommand) RunIntoGlazeProcessor(ctx context.Context, vals *values.Values, gp middlewares.Processor) error {
	s := &DepsSettings{}
	if err := vals.DecodeSectionInto(slug, s); err != nil {
		return err
	}

	_, _, _, imports, err := engine.ParseStylesheet(ctx, s.Stylesheet)
	if err != nil {
		return err
	}

	for _, imp := range imports {
		row := types.NewRow(
			types.MRP("type", imp.Type),
			types.MRP("href", imp.Href),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func splitOnCommas(s string) []string {
	return splitOn(s, ',')
}

func splitOnEquals(s string) []string {
	idx := -1
	for i, c := range s {
		if c == '=' && idx == -1 {
			idx = i
			break
		}
	}
	if idx < 0 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+1:]}
}

func splitOn(s string, sep rune) []string {
	var result []string
	var current strings.Builder
	for _, c := range s {
		if c == sep {
			result = append(result, current.String())
			current.Reset()
		} else {
			current.WriteRune(c)
		}
	}
	result = append(result, current.String())
	return result
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func xsltGraphToMermaid(graph *engine.XSLTGraph) string {
	var b strings.Builder
	b.WriteString("graph TD\n")
	for _, node := range graph.Nodes {
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", node.ID, node.Label)
	}
	for _, edge := range graph.Edges {
		fmt.Fprintf(&b, "  %s -->|%s| %s\n", edge.From, edge.Kind, edge.To)
	}
	return b.String()
}

func xsltGraphToDOT(graph *engine.XSLTGraph) string {
	var b strings.Builder
	b.WriteString("digraph xslt {\n  rankdir=TB;\n")
	for _, node := range graph.Nodes {
		fmt.Fprintf(&b, "  %s [label=\"%s\"];\n", node.ID, node.Label)
	}
	for _, edge := range graph.Edges {
		fmt.Fprintf(&b, "  %s -> %s [label=\"%s\"];\n", edge.From, edge.To, edge.Kind)
	}
	b.WriteString("}\n")
	return b.String()
}

// ─── Registration ────────────────────────────────────────────────────────────

func Register(root *cobra.Command) error {
	xslCmd := &cobra.Command{
		Use:   "xsl",
		Short: "XSLT execution, static analysis, and profiling",
		Long: `XSLT sub-commands for running transformations, listing
components, finding references, detecting unused code,
visualizing dependencies, and profiling execution.`,
	}

	commands := []struct {
		name string
		cmd  cmds.GlazeCommand
	}{
		{"run", mustCmd(NewRunCommand())},
		{"list", mustCmd(NewListCommand())},
		{"refs", mustCmd(NewRefsCommand())},
		{"unused", mustCmd(NewUnusedCommand())},
		{"graph", mustCmd(NewGraphCommand())},
		{"deps", mustCmd(NewDepsCommand())},
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
		xslCmd.AddCommand(cobraCmd)
	}

	root.AddCommand(xslCmd)
	return nil
}

func mustCmd(cmd cmds.GlazeCommand, err error) cmds.GlazeCommand {
	if err != nil {
		panic(err)
	}
	return cmd
}
