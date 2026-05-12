package cmds

import (
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	"github.com/spf13/cobra"

	"github.com/go-go-golems/xml/pkg/cmds/catalog"
	"github.com/go-go-golems/xml/pkg/cmds/dtd"
	"github.com/go-go-golems/xml/pkg/cmds/explain_error"
	"github.com/go-go-golems/xml/pkg/cmds/lint"
	"github.com/go-go-golems/xml/pkg/cmds/schema"
	"github.com/go-go-golems/xml/pkg/cmds/sch"
	"github.com/go-go-golems/xml/pkg/cmds/validate"
	"github.com/go-go-golems/xml/pkg/cmds/xpath"
	"github.com/go-go-golems/xml/pkg/cmds/xsl"
	"github.com/go-go-golems/xml/doc"
)

// Version is set at build time via -ldflags.
var Version = "0.1.0-dev"

func NewRootCommand() (*cobra.Command, error) {
	rootCmd := &cobra.Command{
		Use:   "xml",
		Short: "Modern XML toolkit for developers",
		Long: `A unified CLI for XML validation, schema authoring,
Schematron workflows, XSLT development, and more.

Built on the helium Go XML engine and the Glazed command framework.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return logging.InitLoggerFromCobra(cmd)
		},
	}

	if err := logging.AddLoggingSectionToRootCommand(rootCmd, "xml"); err != nil {
		return nil, err
	}

	helpSystem := help.NewHelpSystem()
	if err := doc.AddDocsToHelpSystem(helpSystem); err != nil {
		return nil, err
	}
	help_cmd.SetupCobraRootCommand(helpSystem, rootCmd)

	// Register command groups
	if err := validate.Register(rootCmd); err != nil {
		return nil, err
	}
	if err := lint.Register(rootCmd); err != nil {
		return nil, err
	}
	if err := xpath.Register(rootCmd); err != nil {
		return nil, err
	}
	if err := catalog.Register(rootCmd); err != nil {
		return nil, err
	}
	if err := explain_error.Register(rootCmd); err != nil {
		return nil, err
	}
	if err := schema.Register(rootCmd); err != nil {
		return nil, err
	}
	if err := dtd.Register(rootCmd); err != nil {
		return nil, err
	}
	if err := sch.Register(rootCmd); err != nil {
		return nil, err
	}
	if err := xsl.Register(rootCmd); err != nil {
		return nil, err
	}

	// Version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("xml version", Version)
		},
	})

	return rootCmd, nil
}
