---
Title: User Guide
Slug: user-guide
Short: Complete reference for all xml CLI commands, flags, and output formats.
Topics:
- reference
- commands
- output-formats
- configuration
Commands:
- validate
- lint
- xpath
- catalog
- explain-error
- schema
- dtd
- sch
- xsl
Flags:
- --output
- --format
- --xsd
- --rng
- --sch
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
Order: 2
---

The `xml` CLI provides 9 top-level commands covering the XML development lifecycle: validation, parsing, querying, catalog management, error translation, schema authoring, DTD analysis, Schematron workflows, and XSLT execution. Every command supports the same output format flags, the same error handling, and the same exit code conventions.

## Output formats

Every command accepts the `--output` flag to select the rendering format and the `--format` flag for specialized output (SARIF, GitHub, JUnit). The available formats are:

| `--output` value | Description | When to use |
|------------------|-------------|-------------|
| `table` | Aligned columns in the terminal (default) | Interactive use |
| `json` | One JSON object per row | Piping to `jq`, programmatic consumption |
| `yaml` | YAML block per row | Configuration files, human-readable structured data |
| `csv` | Comma-separated values | Spreadsheet import, simple scripting |
| `sqlite` | Write rows to a SQLite database | Persistent storage, complex queries |

The `--format` flag controls specialized validation output:

| `--format` value | Description | When to use |
|-------------------|-------------|-------------|
| `pretty` | Human-readable error list (default) | Interactive validation |
| `sarif` | SARIF JSON for GitHub Code Scanning | CI with Code Scanning alerts |
| `github` | GitHub Actions workflow commands | CI with `::error::` annotations |
| `junit` | JUnit XML | CI pipelines (Jenkins, GitLab, etc.) |

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success — no errors found |
| 1 | Failure — validation errors, parse errors, or command errors |
| 2 | Usage error — invalid flags or missing required arguments |

The exit code convention is consistent across all commands. A validation that finds errors exits with 1, regardless of the output format. This allows shell scripts to use `xml validate` in conditionals: `if xml validate doc.xml --xsd schema.xsd; then echo "valid"; fi`.

## Command reference

### xml validate

Validates XML documents against one or more schema types. Supports XSD, RELAX NG, Schematron, and DTD validation in a single pass.

```bash
xml validate [files...] --xsd <file> [--rng <file>] [--sch <file>] [--dtd]
              [--profile <name>] [--format pretty|sarif|github|junit]
              [--output table|json|yaml|csv] [--no-network]
```

Key flags:

| Flag | Purpose |
|------|---------|
| `--xsd` | XSD schema file |
| `--rng` | RELAX NG schema file |
| `--sch` | Schematron schema file |
| `--dtd` | Validate against DTD in the document |
| `--profile` | Named validation profile from `xml.toml` |
| `--format` | Output format: pretty, sarif, github, junit |
| `--no-network` | Block network access (no DTD/schema download) |

Multiple schema flags can be combined. The validation pipeline runs each stage in order and collects all errors before producing output.

### xml lint

Parses, formats, canonicalizes, and processes XML documents. Similar to `xmllint` but with structured output.

```bash
xml lint <file> [--format] [--c14n] [--c14n-version 1.0|1.1|exclusive]
         [--xinclude] [--valid] [--noout] [--xml-output <path>]
         [--output table|json|yaml|csv]
```

Key flags:

| Flag | Purpose |
|------|---------|
| `--format` | Pretty-print the output |
| `--c14n` | Canonical XML output |
| `--c14n-version` | Canonical XML version: 1.0, 1.1, or exclusive |
| `--xinclude` | Process XInclude directives |
| `--valid` | Validate against the document's DTD |
| `--noout` | Suppress output (check well-formedness only) |
| `--xml-output` | Write output to a file (renamed to avoid conflict with Glazed's `--output`) |

### xml xpath

Evaluates XPath expressions against an XML document. Supports XPath 1.0 and XPath 3.1.

```bash
xml xpath <expression> <file> [--engine 1|3] [--output table|json|yaml|csv]
```

Key flags:

| Flag | Purpose |
|------|---------|
| `--engine` | XPath engine version: `1` for XPath 1.0, `3` for XPath 3.1 (default: `3`) |

The expression is a positional argument (not a flag). This means you write `xml xpath '//book/title' doc.xml`, not `xml xpath --expr '//book/title' doc.xml`.

XPath 3.1 is the default. It provides additional functions (`array`, `map`, `sort`, `string-join`, `for-each`) and a richer type system. Use `--engine 1` for XPath 1.0 compatibility. The output includes the result type, value, and XPath type classification.

### xml catalog

Manages OASIS XML Catalog files for resolving public identifiers and system identifiers.

```bash
xml catalog init   [--catalog <path>]              # Create a new catalog
xml catalog add    --catalog <path> --uri <uri> --resolve <file>  # Add a mapping
xml catalog resolve --catalog <path> --uri <uri>    # Resolve a URI
xml catalog check  --catalog <path>                 # Validate catalog integrity
```

Catalogs map external identifiers (public IDs, system IDs, URIs) to local files. This is essential for offline or air-gapped environments where schemas and DTDs cannot be fetched from the internet.

### xml explain-error

Translates W3C XML Schema validation error codes into human-readable prose with likely causes and suggested commands.

```bash
xml explain-error --code <cvc-code>
```

Supported codes include the 15 most common `cvc-*` codes: `cvc-complex-type.2.4.a` (unexpected child element), `cvc-datatype-valid.1.2.1` (invalid value for type), `cvc-enumeration-valid` (value not in enumeration), and more. The output includes the code, a plain-English explanation, a list of likely causes, and suggested `xml` commands for further investigation.

### xml schema

8 subcommands for XSD schema introspection and authoring.

```bash
xml schema explain  --schema <file> --name <type> [--namespace <ns>]  # Describe a type in prose
xml schema graph   --schema <file> [--graph-format mermaid|dot]       # Dependency graph
xml schema lint    --schema <file>                                     # Find unused types, deep nesting
xml schema refs    --schema <file> --name <type>                      # Find all references
xml schema infer   --input <file> [--input <file>...] [--simple-types] # Generate XSD from examples
xml schema diff    --old <file> --new <file>                          # Semantic diff
xml schema breakage --old <file> --new <file> [--corpus <dir>]        # Breaking change analysis
xml schema list    --schema <file>                                     # List all named components
```

The `explain` command walks the compiled schema type tree and produces a prose description including content type, base type, derivation, child elements, attributes, and facets. The `diff` command classifies changes as `breaking`, `safe`, or `warning`. The `breakage` command extends diff with corpus validation to measure real-world impact. The `infer` command generates a starting XSD from example XML documents.

### xml dtd

4 subcommands for DTD analysis and safety checking.

```bash
xml dtd inspect  --dtd <file>   # List elements, attributes, entities, notations
xml dtd flatten  --dtd <file>   # Resolve parameter entity includes
xml dtd entities --dtd <file>   # List entities with expansion attack detection
xml dtd audit    --dtd <file>   # Comprehensive health check
```

The `entities` command detects billion laughs attacks by counting entity references within each entity's value. Entities with more than 5 references are flagged as potential expansion vectors. External entities referencing local files are flagged as security concerns.

### xml sch

5 subcommands for Schematron validation and coverage analysis.

```bash
xml sch validate --schema <file> <xml-file>  # Validate with structured output
xml sch compile  --schema <file>             # Compile and verify schema
xml sch test     --schema <file> <xml-file>  # Pass/fail per document
xml sch report   --schema <file> <xml-file>  # Pattern/rule/message report
xml sch coverage --schema <file> --corpus <dir>  # Rule coverage across corpus
```

Schematron validates business rules using XPath assertions. A failed `<assert>` produces an error; a fired `<report>` produces informational output. The `coverage` command measures which rules fire against a corpus of documents, identifying rules that are never exercised.

### xml xsl

6 subcommands for XSLT 3.0 execution and static analysis.

```bash
xml xsl run    --stylesheet <file> --input <file> [--xml-output <path>] [--params k=v]  # Execute transformation
xml xsl list   --stylesheet <file>   # List templates, functions, variables
xml xsl refs   --stylesheet <file> --name <name>   # Find references
xml xsl unused --stylesheet <file>   # Detect unused named templates
xml xsl graph  --stylesheet <file> [--graph-format mermaid|dot]  # Dependency graph
xml xsl deps   --stylesheet <file>   # Import/include dependencies
```

The `run` command compiles and executes an XSLT stylesheet against an input document. The `list` and `unused` commands perform static analysis by walking the stylesheet DOM (not the compiled representation). The `unused` command flags named templates that have no `match` attribute and no name collision with a match-only template.

## Configuration

The `xml` CLI reads optional configuration from `xml.toml` files. It searches the current directory and all parent directories, using the first file found.

### Validation profiles

A profile defines a reusable set of schema files and format options:

```toml
[profiles.docbook]
xsd = "schemas/docbook.xsd"
schematron = ["rules/docbook-rules.sch"]
format = "json"

[profiles.tei]
xsd = "schemas/tei.xsd"
no-network = true
```

Use a profile with `--profile`:

```bash
xml validate doc.xml --profile docbook
```

### Catalog configuration

```toml
[catalogs.main]
path = "catalog.xml"
```

When a catalog is configured, the validation engine uses it to resolve external identifiers before attempting network resolution.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| No output from validate on valid documents | Valid documents produce zero error rows | Use `--output json` to confirm empty result, or test with an invalid document |
| CSV output is empty on validation failure | `ErrValidationFailed` propagates before Glazed flush | Use `--output json` or combine stdout+stderr |
| `--output` flag conflict in `xml lint` and `xml xsl run` | Glazed reserves `--output` for format selection | Use `--xml-output` for file output in lint and xsl run |
| Schematron coverage shows no results | Corpus documents pass all rules (no fires = no coverage) | Add documents that intentionally violate rules, or add `<report>` elements |
| `xml xsl unused` flags templates that are called | Static analysis does not trace `call-template` inside bodies | Known limitation — verify manually before removing |

## See Also

- `xml help getting-started` — Install and run your first commands
- `xml help validation-pipeline` — How multi-stage validation works internally
- `xml help output-formats` — Detailed guide to all output formats
