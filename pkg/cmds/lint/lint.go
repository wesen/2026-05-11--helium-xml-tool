package lint

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/c14n"
	"github.com/lestrrat-go/helium/xinclude"
	"github.com/spf13/cobra"

	"github.com/go-go-golems/xml/pkg/engine"
)

// LintCommand implements the `xml lint` command.
type LintCommand struct {
	*cmds.CommandDescription
}

// LintSettings maps flags to typed Go values.
type LintSettings struct {
	Recover      bool   `glazed:"recover"`
	Noent        bool   `glazed:"noent"`
	LoadDTD      bool   `glazed:"load-dtd"`
	ValidateDTD  bool   `glazed:"valid"`
	NoWarning    bool   `glazed:"nowarning"`
	NoBlanks     bool   `glazed:"noblanks"`
	CleanNS      bool   `glazed:"nsclean"`
	MergeCDATA   bool   `glazed:"nocdata"`
	NoNetwork    bool   `glazed:"nonet"`
	Huge         bool   `glazed:"huge"`
	NoOut        bool   `glazed:"noout"`
	Format       bool   `glazed:"format"`
	OutputFile  string `glazed:"xml-output"`
	Encode       string `glazed:"encode"`
	C14n         bool   `glazed:"c14n"`
	C14n11       bool   `glazed:"c14n11"`
	ExcC14n      bool   `glazed:"exc-c14n"`
	XInclude     bool   `glazed:"xinclude"`
	NoXIncNode  bool   `glazed:"noxincludenode"`
	NoBaseFixup bool   `glazed:"nofixup-base-uris"`
	DropDTD     bool   `glazed:"dropdtd"`
	Timing       bool   `glazed:"timing"`
	Catalogs     string `glazed:"catalogs"`
	Files        string `glazed:"files"`
}

var _ cmds.GlazeCommand = &LintCommand{}

func NewLintCommand() (*LintCommand, error) {
	glazedSection, err := settings.NewGlazedSchema()
	if err != nil {
		return nil, err
	}

	cmdSettingsSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	cmdDesc := cmds.NewCommandDescription(
		"lint",
		cmds.WithShort("Parse, format, and lint XML documents"),
		cmds.WithLong(`
Parse XML documents and output the result. Can also format, canonicalize,
and process XIncludes. Similar to xmllint but with structured output.

Examples:
  xml lint doc.xml                          # Parse and re-serialize
  xml lint doc.xml --format                 # Pretty-print
  xml lint doc.xml --noout                  # Just check well-formedness
  xml lint doc.xml --c14n                   # Canonical XML output
  xml lint doc.xml --xinclude               # Process XIncludes
  xml lint doc.xml --valid                   # Validate against DTD
  xml lint doc.xml --output json            # Structured parse errors
`),
		cmds.WithFlags(
			fields.New("recover", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Output what was parsable on broken documents"),
			),
			fields.New("noent", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Substitute entity references by their value"),
			),
			fields.New("load-dtd", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Fetch external DTD"),
			),
			fields.New("valid", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Validate document with DTD"),
			),
			fields.New("nowarning", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Suppress parser warnings"),
			),
			fields.New("noblanks", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Drop ignorable blank spaces"),
			),
			fields.New("nsclean", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Remove redundant namespace declarations"),
			),
			fields.New("nocdata", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Merge CDATA sections into text nodes"),
			),
			fields.New("nonet", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Refuse to fetch DTDs over network"),
			),
			fields.New("huge", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Remove internal parser size limits"),
			),
			fields.New("noout", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Suppress result output (just check well-formedness)"),
			),
			fields.New("format", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Reformat/reindent output"),
			),
			fields.New("xml-output", fields.TypeString,
				fields.WithHelp("Write XML output to FILE"),
			),
			fields.New("encode", fields.TypeString,
				fields.WithHelp("Output encoding"),
			),
			fields.New("c14n", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("W3C Canonical XML v1.0"),
			),
			fields.New("c14n11", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("W3C Canonical XML v1.1"),
			),
			fields.New("exc-c14n", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("W3C Exclusive Canonical XML"),
			),
			fields.New("xinclude", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Process XIncludes"),
			),
			fields.New("noxincludenode", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Do not generate XInclude start/end nodes"),
			),
			fields.New("nofixup-base-uris", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Do not fixup xml:base URIs in XInclude"),
			),
			fields.New("dropdtd", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Remove DOCTYPE from result"),
			),
			fields.New("timing", fields.TypeBool,
				fields.WithDefault(false),
				fields.WithHelp("Print timing information"),
			),
			fields.New("catalogs", fields.TypeString,
				fields.WithHelp("Comma-separated list of OASIS XML Catalog files"),
			),
		),
		cmds.WithArguments(
			fields.New("files", fields.TypeString,
				fields.WithHelp("XML files to lint"),
				fields.WithIsArgument(true),
			),
		),
		cmds.WithSections(glazedSection, cmdSettingsSection),
	)

	return &LintCommand{CommandDescription: cmdDesc}, nil
}

func (c *LintCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	s := &LintSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
		return err
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
		return fmt.Errorf("no XML files to lint")
	}

	var catalogFiles []string
	if s.Catalogs != "" {
		catalogFiles = strings.Split(s.Catalogs, ",")
	}

	hasErrors := false

	for _, file := range files {
		parseOpts := engine.ParseOptions{
			BaseURI:       file,
			Recover:       s.Recover,
			SubstituteEnt: s.Noent,
			LoadDTD:       s.LoadDTD,
			ValidateDTD:   s.ValidateDTD,
			StripBlanks:   s.NoBlanks,
			CleanNS:       s.CleanNS,
			MergeCDATA:    s.MergeCDATA,
			NoNetwork:     s.NoNetwork,
			RelaxLimits:   s.Huge,
			CatalogFiles:  catalogFiles,
		}

		parser, _, err := engine.NewParser(parseOpts)
		if err != nil {
			return err
		}

		doc, dur, err := engine.ParseDocument(ctx, parser, file, s.Timing)
		if s.Timing && dur > 0 {
			fmt.Fprintf(os.Stderr, "Parsing %s took %s\n", file, dur)
		}
		if err != nil {
			row := types.NewRow(
				types.MRP("file", file),
				types.MRP("severity", "error"),
				types.MRP("message", err.Error()),
				types.MRP("schema-type", "well-formedness"),
			)
			_ = gp.AddRow(ctx, row)
			hasErrors = true
			if doc == nil {
				continue
			}
		}

		// Process XIncludes if requested
		if s.XInclude && doc != nil {
			var t0 time.Time
			if s.Timing {
				t0 = time.Now()
			}
			xiProc := xinclude.NewProcessor()
			if s.NoXIncNode {
				xiProc = xiProc.NoXIncludeMarkers()
			}
			if s.NoBaseFixup {
				xiProc = xiProc.NoBaseFixup()
			}
			if file != "-" {
				xiProc = xiProc.BaseURI(file)
			}
			_, xiErr := xiProc.Process(ctx, doc)
			if s.Timing {
				fmt.Fprintf(os.Stderr, "XInclude took %s\n", time.Since(t0))
			}
			if xiErr != nil {
				row := types.NewRow(
					types.MRP("file", file),
					types.MRP("severity", "error"),
					types.MRP("message", xiErr.Error()),
					types.MRP("schema-type", "xinclude"),
				)
				_ = gp.AddRow(ctx, row)
				hasErrors = true
			}
		}

		if s.NoOut || doc == nil {
			continue
		}

		// Determine output destination
		out := os.Stdout
		if s.OutputFile != "" {
			f, err := os.Create(s.OutputFile)
			if err != nil {
				return fmt.Errorf("creating output file: %w", err)
			}
			defer f.Close()
			out = f
		}

		// Canonicalization if requested
		if s.C14n || s.C14n11 || s.ExcC14n {
			var mode c14n.Mode
			switch {
			case s.C14n11:
				mode = c14n.C14N11
			case s.ExcC14n:
				mode = c14n.ExclusiveC14N10
			default:
				mode = c14n.C14N10
			}
			if err := c14n.NewCanonicalizer(mode).Comments().Canonicalize(doc, out); err != nil {
				row := types.NewRow(
					types.MRP("file", file),
					types.MRP("severity", "error"),
					types.MRP("message", err.Error()),
					types.MRP("schema-type", "c14n"),
				)
				_ = gp.AddRow(ctx, row)
				hasErrors = true
			}
			continue
		}

		// Normal serialization
		writer := helium.NewWriter()
		if s.Format {
			writer = writer.Format(true).IndentString("  ")
		}
		if s.DropDTD {
			writer = writer.IncludeDTD(false)
		}
		if err := writer.WriteTo(out, doc); err != nil {
			row := types.NewRow(
				types.MRP("file", file),
				types.MRP("severity", "error"),
				types.MRP("message", err.Error()),
			)
			_ = gp.AddRow(ctx, row)
			hasErrors = true
		}
	}

	if hasErrors {
		return fmt.Errorf("one or more files failed lint checks")
	}
	return nil
}

// Register adds the lint command to the root cobra command.
func Register(root *cobra.Command) error {
	cmd, err := NewLintCommand()
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
