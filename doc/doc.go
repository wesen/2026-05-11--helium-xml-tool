package doc

import (
	"embed"

	"github.com/go-go-golems/glazed/pkg/help"
)

//go:embed help/*.md
var HelpFS embed.FS

func AddDocsToHelpSystem(hs *help.HelpSystem) error {
	return hs.LoadSectionsFromFS(HelpFS, "help")
}
