---
Title: Getting Started
Slug: getting-started
Short: Install the xml CLI and run your first validation, lint, and XPath query in under 5 minutes.
Topics:
- getting-started
- installation
- first-steps
Commands:
- validate
- lint
- xpath
Flags:
- --xsd
- --sch
- --output
IsTopLevel: true
ShowPerDefault: true
SectionType: Tutorial
Order: 1
---

## What you will learn

This tutorial walks you through installing the `xml` CLI and running four operations: validating a document against an XSD schema, linting and formatting XML, evaluating an XPath expression, and explaining a cryptic validation error. By the end, you will have the tool installed and understand the common command patterns that every other command builds on.

## Install

Build from source (requires Go 1.22+):

```bash
go install github.com/go-go-golems/xml/cmd/xml@latest
```

Verify the installation:

```bash
xml version
# xml version 0.1.0-dev
```

If you see a version string, the tool is ready. If not, check that `$GOPATH/bin` or `$HOME/go/bin` is on your `PATH`.

## Step 1: Create a test document

Save this as `book.xml`:

```xml
<?xml version="1.0"?>
<book>
  <title>XML Toolkit</title>
  <author>Developer</author>
  <price>29.99</price>
</book>
```

## Step 2: Validate against an XSD schema

Save this as `book.xsd`:

```xml
<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="book">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="title" type="xs:string"/>
        <xs:element name="author" type="xs:string"/>
        <xs:element name="price" type="xs:decimal"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>
```

Run validation:

```bash
xml validate book.xml --xsd book.xsd
```

For a valid document, the command exits with code 0 and produces no error output. For an invalid document — say, if you remove the `<title>` element — the command prints the error and exits with code 1.

Change the output format to JSON to see the structured result:

```bash
xml validate book.xml --xsd book.xsd --output json
```

The JSON output shows one row per error, with fields for the file, line, error code, and message. This format works well for piping into `jq` or other downstream tools.

## Step 3: Lint and format

The `lint` command parses an XML document and re-serializes it. With the `--format` flag, it pretty-prints:

```bash
xml lint book.xml --format
```

Output:

```xml
<?xml version="1.0"?>
<book>
  <title>XML Toolkit</title>
  <author>Developer</author>
  <price>29.99</price>
</book>
```

The `lint` command also supports canonicalization (`--c14n`), XInclude processing (`--xinclude`), and DTD validation (`--valid`). These are the same operations that `xmllint` provides, but with structured output.

## Step 4: Evaluate XPath

Find all element names in the document:

```bash
xml xpath '//*/local-name()' book.xml
```

Or select specific elements:

```bash
xml xpath '//book/title' book.xml --output json
```

The XPath command supports both XPath 1.0 and XPath 3.1. XPath 3.1 is the default; use `--engine 1` for XPath 1.0. XPath 1.0 is useful for compatibility with tools that expect XPath 1.0 semantics. XPath 3.1 provides additional functions like `array`, `map`, `sort`, and `string-join` that are not available in XPath 1.0.

## Step 5: Explain a validation error

When validation fails, the error output includes a W3C error code like `cvc-complex-type.2.4.a`. These codes are precise but not human-readable. The `explain-error` command translates them:

```bash
xml explain-error --code cvc-complex-type.2.4.a
```

Output includes the code, a plain-English explanation, likely causes, and suggested follow-up commands.

## What to explore next

- **Schematron validation**: `xml validate doc.xml --sch rules.sch` — business rule validation with assert/report semantics.
- **Schema workbench**: `xml schema explain --schema book.xsd --name book` — describe a schema type in prose.
- **XSLT transformation**: `xml xsl run --stylesheet transform.xsl --input doc.xml` — execute an XSLT 3.0 stylesheet.
- **Structured output**: Add `--output json`, `--output yaml`, or `--output csv` to any command.
- **CI integration**: Use `--format sarif` for GitHub Code Scanning or `--format junit` for CI pipelines.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| `xml: command not found` | Binary not on PATH | Add `$GOPATH/bin` to your PATH, or use the full path to the binary |
| `cannot access schema: stat ...: no such file or directory` | Schema file path is wrong | Use an absolute path, or run from the directory containing the schema |
| `compiling XSD: ...` compilation error | Schema has syntax errors | Check the schema with `xmllint --schema` or fix the reported error |
| Exit code 1 with no output | Document is valid but `--format` is set | Some formats (SARIF, GitHub, JUnit) suppress normal output; use `--output json` for diagnostics |

## See Also

- `xml help user-guide` — Complete reference for all commands and output formats
- `xml help validation-pipeline` — How multi-stage validation works
- `xml help output-formats` — Detailed guide to table, JSON, YAML, CSV, SARIF, GitHub, and JUnit output
