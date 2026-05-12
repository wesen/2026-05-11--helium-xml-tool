package output

import (
	"fmt"
	"io"

	"github.com/go-go-golems/xml/pkg/engine"
)

// WriteGitHubAnnotations writes validation results as GitHub Actions workflow commands.
// See: https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions
func WriteGitHubAnnotations(results []engine.ValidationResult, w io.Writer) error {
	for _, r := range results {
		severity := r.Severity
		if severity == "info" || severity == "note" {
			severity = "notice"
		}

		if r.Line > 0 {
			fmt.Fprintf(w, "::%s file=%s,line=%d,col=%d::%s\n",
				severity, r.File, r.Line, r.Column, r.Message)
		} else {
			fmt.Fprintf(w, "::%s file=%s::%s\n", severity, r.File, r.Message)
		}
	}
	return nil
}
