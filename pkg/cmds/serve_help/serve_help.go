package serve_help

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	glazedSchema "github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/help"
	helpserver "github.com/go-go-golems/glazed/pkg/help/server"
	"github.com/go-go-golems/glazed/pkg/web"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

const slug = glazedSchema.DefaultSlug

// ServeHelpCommand starts an HTTP server that serves the help documentation
// as a browsable web application with a React SPA and JSON API.
type ServeHelpCommand struct {
	*cmds.CommandDescription
	helpSystem *help.HelpSystem
	spaHandler http.Handler
}

var _ cmds.BareCommand = (*ServeHelpCommand)(nil)

type ServeHelpSettings struct {
	Address string   `glazed:"address"`
	Paths   []string `glazed:"paths"`
}

func NewServeHelpCommand(hs *help.HelpSystem) (*ServeHelpCommand, error) {
	spaHandler, err := web.NewSPAHandler()
	if err != nil {
		return nil, fmt.Errorf("creating SPA handler: %w", err)
	}

	return &ServeHelpCommand{
		CommandDescription: cmds.NewCommandDescription(
			"serve-help",
			cmds.WithShort("Serve help documentation as a web application"),
			cmds.WithLong(`
Start an HTTP server that serves the xml CLI help documentation
as a browsable web application with a React SPA frontend and a JSON API.

By default, serves the embedded documentation (getting-started, user-guide,
validation-pipeline, output-formats). Point at additional markdown files
or directories to include external documentation.

The server provides:
  GET /api/health         — Health check with section count
  GET /api/sections       — List and search help sections
  GET /api/sections/:slug — Full section content
  GET /*                  — React SPA (browser UI)

Examples:
  xml serve-help
  xml serve-help --address :9090
  xml serve-help ./docs
`),
			cmds.WithFlags(
				fields.New("address", fields.TypeString,
					fields.WithDefault(":8088"),
					fields.WithHelp("Address to listen on"),
				),
			),
			cmds.WithArguments(
				fields.New("paths", fields.TypeStringList,
					fields.WithHelp("Additional markdown files or directories to load"),
				),
			),
		),
		helpSystem: hs,
		spaHandler: spaHandler,
	}, nil
}

func (c *ServeHelpCommand) Run(ctx context.Context, parsedValues *values.Values) error {
	s := &ServeHelpSettings{}
	if err := parsedValues.DecodeSectionInto(slug, s); err != nil {
		return fmt.Errorf("decoding settings: %w", err)
	}

	hs := c.helpSystem
	if hs.Store == nil {
		return fmt.Errorf("HelpSystem.Store is nil")
	}

	count, err := hs.Store.Count(ctx)
	if err != nil {
		return fmt.Errorf("counting help sections: %w", err)
	}
	log.Info().Int64("sections", count).Msg("Loaded help sections")

	deps := helpserver.HandlerDeps{Store: hs.Store}
	handler := helpserver.NewServeHandler(deps, c.spaHandler)

	httpSrv := &http.Server{
		Addr:         s.Address,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Info().Str("address", s.Address).Msg("Help browser listening")

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpSrv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return fmt.Errorf("server error: %w", err)
	case sig := <-sigCh:
		log.Info().Str("signal", sig.String()).Msg("Shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	}
}

func RegisterServeHelp(root *cobra.Command, hs *help.HelpSystem) error {
	cmd, err := NewServeHelpCommand(hs)
	if err != nil {
		return err
	}

	cobraCmd, err := cli.BuildCobraCommand(cmd,
		cli.WithParserConfig(cli.CobraParserConfig{
			AppName:           "xml",
			ShortHelpSections: []string{slug},
			MiddlewaresFunc:   cli.CobraCommandDefaultMiddlewares,
		}),
	)
	if err != nil {
		return err
	}

	root.AddCommand(cobraCmd)
	return nil
}
