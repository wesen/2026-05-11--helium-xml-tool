---
Title: Output Formats
Slug: output-formats
Short: How to select and use table, JSON, YAML, CSV, SARIF, GitHub, and JUnit output in any xml command.
Topics:
- output
- formats
- json
- sarif
- csv
- table
Commands:
- validate
- lint
- xpath
Flags:
- --output
- --format
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
Order: 4
---

Every `xml` command produces structured rows. The output format controls how those rows are rendered. Two flags control formatting: `--output` selects the general rendering format, and `--format` selects specialized validation output.

## General formats (`--output`)

### table (default)

Human-readable aligned columns in the terminal. Good for interactive use and quick inspection.

```bash
xml validate doc.xml --xsd schema.xsd
```

### json

One JSON object per row. Each row is a flat key-value map. Rows are separated by commas. Good for piping into `jq` or for programmatic consumption.

```bash
xml validate doc.xml --xsd schema.xsd --output json
```

### yaml

YAML block per row. More readable than JSON for nested or multi-line values.

```bash
xml validate doc.xml --xsd schema.xsd --output yaml
```

### csv

Comma-separated values with a header row. Good for spreadsheet import and simple scripting.

```bash
xml validate doc.xml --xsd schema.xsd --output csv
```

### sqlite

Writes rows to a SQLite database file. Use `--output-file results.db` to specify the database path. Good for persistent storage and complex post-hoc queries.

```bash
xml validate doc.xml --xsd schema.xsd --output sqlite --output-file results.db
```

## Specialized formats (`--format`)

These formats are specific to the `validate` command. They produce output in a standard schema for CI systems.

### pretty (default)

Human-readable error list. Each error shows file, line, severity, and message.

### sarif

Static Analysis Results Interchange Format. JSON-based standard for exchanging static analysis results. GitHub Code Scanning consumes SARIF directly.

```bash
xml validate doc.xml --xsd schema.xsd --format sarif
```

The SARIF output includes:
- `$schema` and `version` fields
- One `run` with the tool name and version
- One `result` per validation error with location, message, and rule ID

### github

GitHub Actions workflow commands. Prints `::error file=...,line=...,col=...::message` for each error. GitHub renders these as inline annotations in pull request diffs.

```bash
xml validate doc.xml --xsd schema.xsd --format github
```

### junit

JUnit XML format. Each validation error becomes a test case failure. CI systems like Jenkins, GitLab CI, and CircleCI consume this format for test reporting.

```bash
xml validate doc.xml --xsd schema.xsd --format junit
```

## Combining `--output` and `--format`

The two flags are independent. `--format` controls the validation-specific rendering (what the errors look like), while `--output` controls the general row rendering (what the wrapper looks like). In practice, you use one or the other:

- For **interactive validation**, use `--format pretty` (or no `--format` flag) with the default table output.
- For **CI validation**, use `--format sarif`, `--format github`, or `--format junit` without `--output`.
- For **structured data extraction**, use `--output json` without `--format`.

## Glazed flag quirks

The Glazed framework provides `--output` and `--output-file` flags by default on every command. In commands that also need a file output path (like `xml lint --xml-output` or `xml xsl run --xml-output`), the file output flag is renamed to `--xml-output` to avoid collision.

The Glazed JSON output may include trailing commas between array elements. If you need strict JSON, pipe through `jq .` to normalize.

## See Also

- `xml help validation-pipeline` — How validation errors are collected and classified
- `xml help getting-started` — First steps with output formats
- `xml help user-guide` — Complete flag reference
