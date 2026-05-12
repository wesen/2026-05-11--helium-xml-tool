# xml — Modern XML Toolkit for Developers

A unified CLI for XML validation, schema authoring, Schematron workflows, XSLT development, and more. Built on [helium](https://github.com/lestrrat-go/helium) (native Go XML engine) and [Glazed](https://github.com/go-go-golems/glazed) (structured output framework).

**Single binary. No JVM. No cgo. No libxml2.**

## Install

```bash
go install github.com/go-go-golems/xml/cmd/xml@latest
```

Or build from source:

```bash
git clone https://github.com/go-go-golems/xml.git
cd xml && make build
```

## Getting Help

The `xml` CLI ships with built-in documentation you can read in the terminal or browse in a web browser.

```bash
# List all help topics
xml help

# Read a specific topic
xml help getting-started
xml help user-guide
xml help validation-pipeline
xml help output-formats

# Browse help as a web application (with search, filtering, and a React UI)
xml serve-help
xml serve-help --address :9090
```

The `getting-started` tutorial walks you through your first validation, lint, and XPath query in under 5 minutes:

```bash
xml help getting-started
```

## Quick Start

```bash
# Validate against XSD
xml validate doc.xml --xsd schema.xsd

# Validate against Schematron business rules
xml validate doc.xml --sch rules.sch

# Chain XSD + Schematron in one pass
xml validate doc.xml --xsd schema.xsd --sch rules.sch

# Pretty-print an XML document
xml lint doc.xml --format

# Run an XSLT transformation
xml xsl run --stylesheet transform.xsl --input doc.xml

# Evaluate XPath
xml xpath '//book/title' doc.xml

# XPath 1.0 mode
xml xpath 'count(//item)' doc.xml --engine 1

# Explain a cryptic validation error
xml explain-error --code cvc-complex-type.2.4.a

# Generate Mermaid dependency graph from an XSD schema
xml schema graph --schema schema.xsd --graph-format mermaid

# Diff two schema versions for breaking changes
xml schema diff --old schema-v1.xsd --new schema-v2.xsd

# Find unused templates in an XSLT stylesheet
xml xsl unused --stylesheet transform.xsl

# Detect billion laughs attacks in a DTD
xml dtd entities --dtd legacy.dtd
```

## Commands

| Command | Description |
|---------|-------------|
| `xml validate` | Validate against XSD, RELAX NG, Schematron, or DTD |
| `xml lint` | Parse, format, canonicalize, XInclude, DTD validation |
| `xml xpath` | Evaluate XPath 1.0 or 3.1 expressions |
| `xml catalog` | OASIS XML Catalog management (init, add, resolve, check) |
| `xml explain-error` | Translate W3C error codes to human prose |
| `xml schema` | Schema workbench: explain, graph, lint, refs, infer, diff, breakage, list |
| `xml dtd` | DTD analysis: inspect, flatten, entities, audit |
| `xml sch` | Schematron: validate, compile, test, report, coverage |
| `xml xsl` | XSLT: run, list, refs, unused, graph, deps |
| `xml serve-help` | Browse help docs in a web browser |

### Schema Workbench (`xml schema`)

8 subcommands for XSD introspection and authoring:

```bash
xml schema explain --schema book.xsd --name BookType   # Describe a type in prose
xml schema graph  --schema book.xsd                     # Mermaid/DOT dependency graph
xml schema lint   --schema book.xsd                     # Find unused types, deep nesting
xml schema refs   --schema book.xsd --name GenreType    # Find all references
xml schema infer  --input doc1.xml --input doc2.xml     # Generate XSD from examples
xml schema diff   --old v1.xsd --new v2.xsd             # Semantic diff (breaking/safe/warning)
xml schema breakage --old v1.xsd --new v2.xsd --corpus  # Corpus-based impact analysis
xml schema list   --schema book.xsd                     # List all named components
```

### Schematron (`xml sch`)

5 subcommands for rule-based validation:

```bash
xml sch validate --schema rules.sch doc.xml    # Structured validation output
xml sch compile  --schema rules.sch            # Verify schema compiles
xml sch test     --schema rules.sch doc.xml     # Pass/fail per document
xml sch report   --schema rules.sch doc.xml     # Human-readable pattern/rule/message
xml sch coverage --schema rules.sch --corpus    # Rule coverage across corpus
```

### XSLT (`xml xsl`)

6 subcommands for transformation and static analysis:

```bash
xml xsl run    --stylesheet transform.xsl --input doc.xml   # Execute transformation
xml xsl list   --stylesheet transform.xsl                    # List templates/functions/variables
xml xsl refs   --stylesheet transform.xsl --name main        # Find references
xml xsl unused --stylesheet transform.xsl                    # Detect dead templates
xml xsl graph  --stylesheet transform.xsl --graph-format dot # Dependency visualization
xml xsl deps   --stylesheet transform.xsl                    # Import/include dependencies
```

## Structured Output

Every command supports multiple output formats through the Glazed framework:

```bash
# Table (default)
xml validate doc.xml --xsd schema.xsd

# JSON
xml validate doc.xml --xsd schema.xsd --output json

# YAML
xml validate doc.xml --xsd schema.xsd --output yaml

# CSV
xml validate doc.xml --xsd schema.xsd --output csv

# SARIF (for GitHub Code Scanning)
xml validate doc.xml --xsd schema.xsd --format sarif

# GitHub Actions annotations
xml validate doc.xml --xsd schema.xsd --format github

# JUnit XML (for CI pipelines)
xml validate doc.xml --xsd schema.xsd --format junit
```

## Validation Pipeline

Chain multiple schema types in a single invocation:

```bash
# XSD structural validation + Schematron business rules
xml validate doc.xml --xsd schema.xsd --sch rules.sch

# Use a named profile from config
xml validate doc.xml --profile docbook

# Auto-detect schema type from file extension
xml validate doc.xml --xsd schema.xsd --rng schema.rng
```

The pipeline runs each stage sequentially. XSD and RELAX NG validate document structure; Schematron validates business rules; DTD validates against a document type declaration. Results from all stages flow into a single structured output stream.

## Configuration

The `xml` CLI reads optional configuration from `xml.toml` in the current directory or any parent:

```toml
[profiles.docbook]
xsd = "schemas/docbook.xsd"
schematron = ["rules/docbook-rules.sch"]
format = "json"

[catalogs.main]
path = "catalog.xml"
```

## Architecture

```
┌─────────────────────────────────────────┐
│  CLI Layer (pkg/cmds/)                   │
│  Cobra commands + Glazed flags/rows     │
├─────────────────────────────────────────┤
│  Engine Layer (pkg/engine/)             │
│  Compile, validate, diff, infer, walk   │
├─────────────────────────────────────────┤
│  helium (github.com/lestrrat-go/helium) │
│  Native Go XML: parse, XSD, RNG, SCH,   │
│  XSLT, XPath, c14n, catalog, XInclude   │
└─────────────────────────────────────────┘
```

- **CLI layer**: Three-struct Glazed pattern (CommandDescription + Settings + RunIntoGlazeProcessor). Produces structured rows that flow through table/JSON/YAML/CSV/SARIF formatters.
- **Engine layer**: Pure Go functions wrapping helium APIs. No CLI dependencies. Testable without Cobra.
- **helium**: Native Go XML stack. No cgo, no JVM, no libxml2.

## Test Suite

195 tests across 4 layers:

| Layer | Count | Purpose |
|-------|-------|---------|
| Package unit tests | 69 | Engine functions, error codes, output formatters |
| CLI integration tests | 55 | Build binary, run commands, check output |
| Scenario tests | 4 | Multi-stage validation pipelines |
| Adversarial tests | 12 | Malformed XML, billion laughs, huge documents |
| Phase 2 unit tests | 28 | Schema introspection, diff, inference, DTD |
| Phase 3 unit tests | 19 | Schematron, XSLT compilation, static analysis |

```bash
make test
```

## Project Status

| Phase | Scope | Status |
|-------|-------|--------|
| 1 | Validation, lint, XPath, catalog, explain-error | ✅ Complete |
| 2 | Schema workbench (8 commands), DTD analysis (4 commands) | ✅ Complete |
| 3 | Schematron workflows (5 commands), XSLT analysis (6 commands) | ✅ Complete |
| 4 | TUI, LSP, HTML reports, generation | 🔜 Planned |

## License

MIT
