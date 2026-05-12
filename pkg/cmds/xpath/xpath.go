package xpath

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/xpath1"
	"github.com/lestrrat-go/helium/xpath3"
	"github.com/spf13/cobra"

	"github.com/go-go-golems/xml/pkg/engine"
)

// XPathCommand implements the `xml xpath` command.
type XPathCommand struct {
	*cmds.CommandDescription
}

// XPathSettings maps flags to typed Go values.
type XPathSettings struct {
	Expression string `glazed:"expr"`
	Engine     string `glazed:"engine"`
	Files      string `glazed:"files"`
}

var _ cmds.GlazeCommand = &XPathCommand{}

func NewXPathCommand() (*XPathCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}

	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	cmdDesc := cmds.NewCommandDescription(
		"xpath",
		cmds.WithShort("Evaluate XPath expressions against XML input"),
		cmds.WithLong(`
Evaluate XPath expressions against XML documents. Supports both
XPath 1.0 and XPath 3.1 engines.

Examples:
  xml xpath '/root/item' doc.xml
  xml xpath 'count(//item)' doc.xml --engine 1
  xml xpath '//item[@id="x1"]' doc.xml --output json
  xml xpath 'string-join(//item/@id, ",")' doc.xml --engine 3
`),
		cmds.WithFlags(
			fields.New("engine", fields.TypeString,
				fields.WithDefault("3"),
				fields.WithHelp("XPath engine version: 1 or 3"),
			),
		),
		cmds.WithArguments(
			fields.New("expr", fields.TypeString,
				fields.WithHelp("XPath expression to evaluate"),
				fields.WithIsArgument(true),
				fields.WithRequired(true),
			),
			fields.New("files", fields.TypeString,
				fields.WithHelp("XML files to query"),
				fields.WithIsArgument(true),
			),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)

	return &XPathCommand{CommandDescription: cmdDesc}, nil
}

func (c *XPathCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	s := &XPathSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
		return err
	}

	if s.Expression == "" {
		return fmt.Errorf("xpath expression is required")
	}
	if s.Engine != "1" && s.Engine != "3" {
		return fmt.Errorf("unsupported engine %q (use 1 or 3)", s.Engine)
	}

	var inputPaths []string
	if s.Files != "" {
		inputPaths = append(inputPaths, s.Files)
	}
	files, err := engine.CollectFiles(inputPaths, false)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no XML files to query")
	}

	for _, file := range files {
		parser := helium.NewParser()
		if file != "-" {
			parser = parser.BaseURI(file)
		}

		buf, err := engine.ReadInput(file)
		if err != nil {
			return err
		}

		doc, err := parser.Parse(ctx, buf)
		if err != nil {
			row := types.NewRow(
				types.MRP("file", file),
				types.MRP("severity", "error"),
				types.MRP("message", err.Error()),
			)
			_ = gp.AddRow(ctx, row)
			continue
		}

		if s.Engine == "1" {
			evalXPath1(ctx, s, file, doc, gp)
		} else {
			evalXPath3(ctx, s, file, doc, gp)
		}
	}

	return nil
}

func evalXPath1(ctx context.Context, s *XPathSettings, file string, doc *helium.Document, gp middlewares.Processor) {
	expr, err := xpath1.NewCompiler().Compile(s.Expression)
	if err != nil {
		row := types.NewRow(
			types.MRP("file", file),
			types.MRP("severity", "error"),
			types.MRP("message", fmt.Sprintf("XPath compilation: %s", err)),
		)
		_ = gp.AddRow(ctx, row)
		return
	}

	res, err := xpath1.NewEvaluator().Evaluate(ctx, expr, doc)
	if err != nil {
		row := types.NewRow(
			types.MRP("file", file),
			types.MRP("severity", "error"),
			types.MRP("message", fmt.Sprintf("XPath evaluation: %s", err)),
		)
		_ = gp.AddRow(ctx, row)
		return
	}

	switch res.Type {
	case xpath1.NodeSetResult:
		for _, n := range res.NodeSet {
			row := nodeToRow(file, n)
			_ = gp.AddRow(ctx, row)
		}
	case xpath1.BooleanResult:
		row := types.NewRow(
			types.MRP("file", file),
			types.MRP("type", "boolean"),
			types.MRP("value", fmt.Sprintf("%v", res.Bool)),
		)
		_ = gp.AddRow(ctx, row)
	case xpath1.NumberResult:
		row := types.NewRow(
			types.MRP("file", file),
			types.MRP("type", "number"),
			types.MRP("value", fmt.Sprintf("%g", res.Number)),
		)
		_ = gp.AddRow(ctx, row)
	default:
		row := types.NewRow(
			types.MRP("file", file),
			types.MRP("type", "string"),
			types.MRP("value", res.String),
		)
		_ = gp.AddRow(ctx, row)
	}
}

func evalXPath3(ctx context.Context, s *XPathSettings, file string, doc *helium.Document, gp middlewares.Processor) {
	expr, err := xpath3.NewCompiler().Compile(s.Expression)
	if err != nil {
		row := types.NewRow(
			types.MRP("file", file),
			types.MRP("severity", "error"),
			types.MRP("message", fmt.Sprintf("XPath compilation: %s", err)),
		)
		_ = gp.AddRow(ctx, row)
		return
	}

	res, err := xpath3.NewEvaluator(xpath3.DefaultEvaluatorOptions).Evaluate(ctx, expr, doc)
	if err != nil {
		row := types.NewRow(
			types.MRP("file", file),
			types.MRP("severity", "error"),
			types.MRP("message", fmt.Sprintf("XPath evaluation: %s", err)),
		)
		_ = gp.AddRow(ctx, row)
		return
	}

	if nodes, err := res.Nodes(); err == nil {
		for _, n := range nodes {
			row := nodeToRow(file, n)
			_ = gp.AddRow(ctx, row)
		}
		return
	}

	if b, ok := res.IsBoolean(); ok {
		row := types.NewRow(
			types.MRP("file", file),
			types.MRP("type", "boolean"),
			types.MRP("value", fmt.Sprintf("%v", b)),
		)
		_ = gp.AddRow(ctx, row)
		return
	}

	if n, ok := res.IsNumber(); ok {
		row := types.NewRow(
			types.MRP("file", file),
			types.MRP("type", "number"),
			types.MRP("value", fmt.Sprintf("%g", n)),
		)
		_ = gp.AddRow(ctx, row)
		return
	}

	if s, ok := res.IsString(); ok {
		row := types.NewRow(
			types.MRP("file", file),
			types.MRP("type", "string"),
			types.MRP("value", s),
		)
		_ = gp.AddRow(ctx, row)
		return
	}

	for item := range res.Sequence().Items() {
		switch v := item.(type) {
		case xpath3.NodeItem:
			row := nodeToRow(file, v.Node)
			_ = gp.AddRow(ctx, row)
		case xpath3.AtomicValue:
			row := types.NewRow(
				types.MRP("file", file),
				types.MRP("type", "atomic"),
				types.MRP("value", formatAtomic(v)),
			)
			_ = gp.AddRow(ctx, row)
		default:
			row := types.NewRow(
				types.MRP("file", file),
				types.MRP("type", "unknown"),
				types.MRP("value", fmt.Sprintf("%v", item)),
			)
			_ = gp.AddRow(ctx, row)
		}
	}
}

func nodeToRow(file string, n helium.Node) types.Row {
	switch n.Type() {
	case helium.AttributeNode:
		attr, ok := helium.AsNode[*helium.Attribute](n)
		if !ok {
			return types.NewRow(
				types.MRP("file", file),
				types.MRP("node-type", "attribute"),
				types.MRP("value", fmt.Sprintf("%T", n)),
			)
		}
		return types.NewRow(
			types.MRP("file", file),
			types.MRP("node-type", "attribute"),
			types.MRP("name", attr.Name()),
			types.MRP("value", attr.Value()),
		)
	case helium.NamespaceDeclNode, helium.NamespaceNode:
		ns, ok := n.(interface {
			Prefix() string
			URI() string
		})
		if !ok {
			return types.NewRow(
				types.MRP("file", file),
				types.MRP("node-type", "namespace"),
				types.MRP("value", fmt.Sprintf("%T", n)),
			)
		}
		name := "xmlns"
		if ns.Prefix() != "" {
			name = "xmlns:" + ns.Prefix()
		}
		return types.NewRow(
			types.MRP("file", file),
			types.MRP("node-type", "namespace"),
			types.MRP("name", name),
			types.MRP("value", ns.URI()),
		)
	default:
		// Serialize the node as XML
		s, err := helium.WriteString(n)
		if err != nil {
			return types.NewRow(
				types.MRP("file", file),
				types.MRP("node-type", n.Type().String()),
				types.MRP("value", fmt.Sprintf("serialization error: %s", err)),
			)
		}
		return types.NewRow(
			types.MRP("file", file),
			types.MRP("node-type", n.Type().String()),
			types.MRP("value", strings.TrimRight(s, "\n")),
		)
	}
}

func formatAtomic(v xpath3.AtomicValue) string {
	switch v.TypeName {
	case xpath3.TypeBoolean:
		if v.BooleanVal() {
			return "true"
		}
		return "false"
	case xpath3.TypeString, xpath3.TypeAnyURI, xpath3.TypeUntypedAtomic,
		xpath3.TypeNormalizedString, xpath3.TypeToken, xpath3.TypeLanguage,
		xpath3.TypeName, xpath3.TypeNCName, xpath3.TypeNMTOKEN,
		xpath3.TypeENTITY, xpath3.TypeID, xpath3.TypeIDREF:
		return v.StringVal()
	case xpath3.TypeDecimal:
		return xpath3.DecimalToString(v.BigRat())
	case xpath3.TypeDouble, xpath3.TypeFloat:
		return fmt.Sprintf("%g", v.ToFloat64())
	}

	if v.IsNumeric() {
		return v.BigInt().String()
	}
	return v.String()
}

// Register adds the xpath command to the root cobra command.
func Register(root *cobra.Command) error {
	cmd, err := NewXPathCommand()
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
