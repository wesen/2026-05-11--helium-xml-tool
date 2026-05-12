package output

import (
	"encoding/json"
	"io"

	"github.com/go-go-golems/xml/pkg/engine"
)

// SARIF represents the Static Analysis Results Interchange Format.
type SARIF struct {
	Version string `json:"version"`
	Schema  string `json:"$schema"`
	Runs    []Run  `json:"runs"`
}

// Run represents a single run of a tool.
type Run struct {
	Tool    Tool     `json:"tool"`
	Results []Result `json:"results"`
}

// Tool describes the analysis tool.
type Tool struct {
	Driver Driver `json:"driver"`
}

// Driver describes the tool driver.
type Driver struct {
	Name           string `json:"name"`
	Version        string `json:"version,omitempty"`
	InformationURI string `json:"informationUri"`
}

// Result represents a single result (finding).
type Result struct {
	RuleID    string     `json:"ruleId"`
	Level     string     `json:"level"`
	Message   Message    `json:"message"`
	Locations []Location `json:"locations,omitempty"`
}

// Message holds result message text.
type Message struct {
	Text string `json:"text"`
}

// Location describes where the result was found.
type Location struct {
	PhysicalLocation PhysicalLocation `json:"physicalLocation"`
}

// PhysicalLocation describes a physical location in a file.
type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Region           *Region          `json:"region,omitempty"`
}

// ArtifactLocation identifies a file.
type ArtifactLocation struct {
	URI string `json:"uri"`
}

// Region describes a region within a file.
type Region struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
}

// WriteSARIF writes validation results in SARIF 2.1.0 format.
func WriteSARIF(results []engine.ValidationResult, toolName, toolVersion string, w io.Writer) error {
	sarif := SARIF{
		Version: "2.1.0",
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json",
		Runs: []Run{{
			Tool: Tool{
				Driver: Driver{
					Name:           toolName,
					Version:        toolVersion,
					InformationURI: "https://github.com/go-go-golems/xml",
				},
			},
			Results: convertSARIFResults(results),
		}},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(sarif)
}

func convertSARIFResults(results []engine.ValidationResult) []Result {
	out := make([]Result, 0, len(results))
	for _, r := range results {
		level := "error"
		if r.Severity == "warning" {
			level = "warning"
		} else if r.Severity == "info" || r.Severity == "note" {
			level = "note"
		}

		ruleID := r.SchemaType
		if r.Rule != "" {
			ruleID = r.Rule
		}
		if r.RawCode != "" {
			ruleID = r.RawCode
		}

		sr := Result{
			RuleID: ruleID,
			Level:  level,
			Message: Message{Text: r.Message},
		}

		if r.File != "" {
			loc := Location{
				PhysicalLocation: PhysicalLocation{
					ArtifactLocation: ArtifactLocation{URI: r.File},
				},
			}
			if r.Line > 0 {
				loc.PhysicalLocation.Region = &Region{
					StartLine:   r.Line,
					StartColumn: r.Column,
				}
			}
			sr.Locations = []Location{loc}
		}

		out = append(out, sr)
	}
	return out
}
