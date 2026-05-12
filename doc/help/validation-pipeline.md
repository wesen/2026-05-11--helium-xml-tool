---
Title: Validation Pipeline
Slug: validation-pipeline
Short: How xml validate chains XSD, RELAX NG, Schematron, and DTD checks into a single structured stream.
Topics:
- validation
- pipeline
- xsd
- relaxng
- schematron
- dtd
Commands:
- validate
Flags:
- --xsd
- --rng
- --sch
- --dtd
- --profile
- --format
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
Order: 3
---

The `xml validate` command does not run a single check. It builds a pipeline of validation stages and runs them in sequence, collecting errors from each stage into a unified stream. This design lets you chain structural validation (XSD, RELAX NG) with business rule validation (Schematron) and legacy validation (DTD) in one invocation.

## How the pipeline works

When you run:

```bash
xml validate doc.xml --xsd schema.xsd --sch rules.sch
```

The command builds a pipeline with two stages:

1. **XSD stage**: Compile `schema.xsd`, parse `doc.xml`, validate the document against the compiled schema. Collect any `cvc-*` errors.
2. **Schematron stage**: Compile `rules.sch`, validate the same document against the compiled Schematron schema. Collect any failed asserts or fired reports.

Both stages run regardless of whether the first stage produces errors. A document can fail XSD validation but still produce useful Schematron results — for example, a structurally valid document might violate a business rule, and the author wants to see all errors at once rather than fixing them one at a time.

## Stage types

| Stage | Schema flag | Engine | Error style |
|-------|------------|--------|-------------|
| XSD | `--xsd` | `xsd.Compiler` → `xsd.Validator` | W3C `cvc-*` error codes |
| RELAX NG | `--rng` | `relaxng.Compiler` → `relaxng.Validator` | RelaxNG validation errors |
| Schematron | `--sch` | `schematron.Compiler` → `schematron.Validator` | Assert/report messages |
| DTD | `--dtd` | Parser with DTD validation | DTD validity errors |

Each stage compiles its schema independently. Compilation errors are reported immediately and abort the pipeline for that schema. Validation errors are collected and reported together after all stages complete.

## Error handler pattern

Every helium validator delivers errors through the `helium.ErrorHandler` interface:

```go
type ErrorHandler interface {
    Handle(ctx context.Context, err error)
}
```

The validation pipeline creates a collector that implements this interface. Each stage's validator is configured with the same collector. When a stage calls `collector.Handle(ctx, err)`, the error is converted to a `ValidationResult` struct with fields for file, line, column, severity, error code, and message.

This pattern is uniform across all four schema types. The CLI does not need to handle XSD errors differently from Schematron errors — they both flow through the same handler, into the same result type, and out through the same output formatter.

## Profiles

When you find yourself running the same validation command repeatedly, define a profile in `xml.toml`:

```toml
[profiles.docbook]
xsd = "schemas/docbook.xsd"
schematron = ["rules/structure.sch", "rules/style.sch"]
format = "json"
no-network = true
```

Then run:

```bash
xml validate doc.xml --profile docbook
```

The profile expands into the equivalent set of flags. You can override profile values with explicit flags on the command line.

## CI integration

The `--format` flag produces machine-readable output for CI systems:

- **SARIF** (`--format sarif`): JSON format for GitHub Code Scanning. Upload with `github/codeql-action/upload-sarif`.
- **GitHub** (`--format github`): Prints `::error file=...::message` annotations that GitHub Actions renders inline.
- **JUnit** (`--format junit`): XML format for Jenkins, GitLab CI, and other CI systems that consume JUnit test results.

Example GitHub Actions workflow:

```yaml
- name: Validate XML
  run: |
    xml validate src/**/*.xml --xsd schema.xsd --format github
  continue-on-error: true

- name: Upload SARIF
  uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: results.sarif.json
```

## See Also

- `xml help getting-started` — First steps with the xml CLI
- `xml help output-formats` — All available output formats and when to use each
- `xml help user-guide` — Complete command reference
