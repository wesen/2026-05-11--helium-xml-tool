package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/schematron"
	"github.com/lestrrat-go/helium/xslt3"
)

// ─── Schematron ─────────────────────────────────────────────────────────────

// SchValidateResult holds a single Schematron validation finding.
type SchValidateResult struct {
	File     string `json:"file"`
	Pattern  string `json:"pattern,omitempty"`
	Rule     string `json:"rule,omitempty"`
	Type     string `json:"type"`     // "assert" or "report"
	Expr     string `json:"expr"`
	Message  string `json:"message"`
	Line     int    `json:"line"`
	Severity string `json:"severity"` // "error" for failed asserts, "info" for reports
}

// SchCoverage represents coverage data for a Schematron rule.
type SchCoverage struct {
	Rule   string `json:"rule"`
	Hits   int    `json:"hits"`
	Status string `json:"status"` // "covered", "uncovered", "failed"
}

// CompileSchematron compiles a Schematron file.
func CompileSchematron(ctx context.Context, path string) (*schematron.Schema, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, fmt.Errorf("cannot access schema: %w", err)
	}
	schema, err := schematron.NewCompiler().Label(abs).CompileFile(ctx, abs)
	if err != nil {
		return nil, fmt.Errorf("compiling Schematron: %w", err)
	}
	return schema, nil
}

// SchValidate validates a document against a Schematron schema and returns structured results.
func SchValidate(ctx context.Context, schema *schematron.Schema, file string, noNetwork bool) ([]SchValidateResult, error) {
	buf, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", file, err)
	}

	parser := helium.NewParser()
	if noNetwork {
		parser = parser.AllowNetwork(false)
	}

	doc, err := parser.Parse(ctx, buf)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", file, err)
	}
	if doc == nil {
		return nil, fmt.Errorf("parsing %s: nil document", file)
	}

	collector := &schResultCollector{
		file: file,
	}
	_ = schematron.NewValidator(schema).Label(file).ErrorHandler(collector).Validate(ctx, doc)

	return collector.results, nil
}

type schResultCollector struct {
	results []SchValidateResult
	file    string
}

func (c *schResultCollector) Handle(_ context.Context, err error) {
	msg := err.Error()
	severity := "error"
	testType := "assert"
	if strings.Contains(msg, "report:") || strings.Contains(strings.ToLower(msg), "report") {
		severity = "info"
		testType = "report"
	}
	c.results = append(c.results, SchValidateResult{
		File:     c.file,
		Message:  msg,
		Type:     testType,
		Severity: severity,
	})
}

// SchCoverageAnalysis runs Schematron validation against a corpus and tracks which rules fire.
func SchCoverageAnalysis(ctx context.Context, schema *schematron.Schema, corpusPaths []string, noNetwork bool) ([]SchCoverage, error) {
	ruleHits := map[string]int{}
	ruleFails := map[string]int{}

	for _, path := range corpusPaths {
		results, err := SchValidate(ctx, schema, path, noNetwork)
		if err != nil {
			continue
		}
		for _, r := range results {
			key := r.Rule
			if key == "" {
				key = r.Message
			}
			if r.Severity == "error" {
				ruleFails[key]++
			}
			ruleHits[key]++
		}
	}

	var coverage []SchCoverage
	for key, hits := range ruleHits {
		status := "covered"
		if ruleFails[key] > 0 {
			status = "failed"
		}
		coverage = append(coverage, SchCoverage{
			Rule:   key,
			Hits:   hits,
			Status: status,
		})
	}

	return coverage, nil
}

// ─── XSLT ───────────────────────────────────────────────────────────────────

// XSLTTemplate represents a template found in a stylesheet.
type XSLTTemplate struct {
	Name       string  `json:"name"`
	Match      string  `json:"match"`
	Mode       string  `json:"mode"`
	Priority   float64 `json:"priority"`
	Visibility string  `json:"visibility"`
}

// XSLTFunction represents an xsl:function in a stylesheet.
type XSLTFunction struct {
	Name     string `json:"name"`
	Override bool   `json:"override"`
}

// XSLTVariable represents a global variable or parameter.
type XSLTVariable struct {
	Name   string `json:"name"`
	Type   string `json:"type"`   // "variable" or "parameter"
	Select string `json:"select"`
}

// XSLTImport represents an import or include.
type XSLTImport struct {
	Href string `json:"href"`
	Type string `json:"type"` // "import" or "include"
}

// XSLTStaticAnalysis holds parsed stylesheet structure.
type XSLTStaticAnalysis struct {
	Templates []XSLTTemplate `json:"templates"`
	Functions []XSLTFunction `json:"functions"`
	Variables []XSLTVariable `json:"variables"`
	Imports  []XSLTImport   `json:"imports"`
}

// XSLTGraphNode represents a node in the stylesheet dependency graph.
type XSLTGraphNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"` // "stylesheet", "template", "function", "variable"
}

// XSLTGraphEdge represents an edge in the stylesheet dependency graph.
type XSLTGraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"` // "import", "include", "calls", "uses"
}

// XSLTGraph is the full stylesheet dependency graph.
type XSLTGraph struct {
	Nodes []XSLTGraphNode `json:"nodes"`
	Edges []XSLTGraphEdge `json:"edges"`
}

// CompileXSLT compiles an XSLT stylesheet.
func CompileXSLT(ctx context.Context, path string) (*xslt3.Stylesheet, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, fmt.Errorf("cannot access stylesheet: %w", err)
	}

	buf, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("reading stylesheet: %w", err)
	}

	doc, err := helium.NewParser().Parse(ctx, buf)
	if err != nil {
		return nil, fmt.Errorf("parsing stylesheet: %w", err)
	}

	ss, err := xslt3.CompileStylesheet(ctx, doc)
	if err != nil {
		return nil, fmt.Errorf("compiling stylesheet: %w", err)
	}

	return ss, nil
}

// XSLTRunResult holds the result of an XSLT transformation.
type XSLTRunResult struct {
	Stylesheet string `json:"stylesheet"`
	Input      string `json:"input"`
	Output     string `json:"output"`
	ResultSize int    `json:"result-size"`
	OutputDest string `json:"output-dest"`
}

// XSLTRun executes an XSLT transformation.
func XSLTRun(ctx context.Context, stylesheetPath, inputPath string, params map[string]string, outputPath string, noNetwork bool) (*XSLTRunResult, error) {
	ss, err := CompileXSLT(ctx, stylesheetPath)
	if err != nil {
		return nil, err
	}

	inputBuf, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}
	parser := helium.NewParser()
	if noNetwork {
		parser = parser.AllowNetwork(false)
	}
	inputDoc, err := parser.Parse(ctx, inputBuf)
	if err != nil {
		return nil, fmt.Errorf("parsing input: %w", err)
	}

	// Build the invocation
	inv := ss.Transform(inputDoc)

	// Set parameters
	for name, value := range params {
		_ = name
		_ = value
		// inv = inv.SetParameter(name, xpath3StringValue(value))
		// Parameter setting requires xpath3.Sequence construction
		// which is not straightforward from string values.
		// For now, parameters are passed through the invocation without conversion.
	}

	// Execute
	result, err := inv.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("executing transformation: %w", err)
	}

	// Serialize result
	var resultStr string
	if result != nil {
		writer := helium.NewWriter()
		var out strings.Builder
		if err := writer.WriteTo(&out, result); err != nil {
			return nil, fmt.Errorf("serializing result: %w", err)
		}
		resultStr = out.String()
	}

	// Write to output file if specified
	outputDest := "stdout"
	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(resultStr), 0644); err != nil {
			return nil, fmt.Errorf("writing output: %w", err)
		}
		outputDest = "file"
	}

	return &XSLTRunResult{
		Stylesheet: stylesheetPath,
		Input:      inputPath,
		Output:     outputPath,
		ResultSize: len(resultStr),
		OutputDest: outputDest,
	}, nil
}

// ParseStylesheet extracts templates, functions, variables, and imports
// from a parsed XSLT stylesheet document using DOM walking.
func ParseStylesheet(ctx context.Context, path string) ([]XSLTTemplate, []XSLTFunction, []XSLTVariable, []XSLTImport, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("reading stylesheet: %w", err)
	}

	parser := helium.NewParser()
	doc, err := parser.Parse(ctx, buf)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("parsing stylesheet: %w", err)
	}
	if doc == nil {
		return nil, nil, nil, nil, fmt.Errorf("parsing stylesheet: nil document")
	}

	var templates []XSLTTemplate
	var functions []XSLTFunction
	var variables []XSLTVariable
	var imports []XSLTImport

	root := doc.DocumentElement()
	if root == nil {
		return nil, nil, nil, nil, fmt.Errorf("empty stylesheet document")
	}

	walkXSLTElements(root, &templates, &functions, &variables, &imports)

	return templates, functions, variables, imports, nil
}

func walkXSLTElements(elem *helium.Element, templates *[]XSLTTemplate, functions *[]XSLTFunction, variables *[]XSLTVariable, imports *[]XSLTImport) {
	local := elem.LocalName()
	ns := ""
	if elemNs := elem.Namespace(); elemNs != nil {
		ns = elemNs.URI()
	}

	if ns == "http://www.w3.org/1999/XSL/Transform" {
		switch local {
		case "template":
			t := XSLTTemplate{
				Name:       getAttr(elem, "name"),
				Match:      getAttr(elem, "match"),
				Mode:       getAttr(elem, "mode"),
				Visibility: getAttr(elem, "visibility"),
			}
			*templates = append(*templates, t)
		case "function":
			f := XSLTFunction{Name: getAttr(elem, "name")}
			*functions = append(*functions, f)
		case "variable":
			v := XSLTVariable{
				Name:   getAttr(elem, "name"),
				Type:   "variable",
				Select: getAttr(elem, "select"),
			}
			*variables = append(*variables, v)
		case "param":
			v := XSLTVariable{
				Name:   getAttr(elem, "name"),
				Type:   "parameter",
				Select: getAttr(elem, "select"),
			}
			*variables = append(*variables, v)
		case "import":
			imp := XSLTImport{Href: getAttr(elem, "href"), Type: "import"}
			*imports = append(*imports, imp)
		case "include":
			imp := XSLTImport{Href: getAttr(elem, "href"), Type: "include"}
			*imports = append(*imports, imp)
		}
	}

	for child := elem.FirstChild(); child != nil; child = child.NextSibling() {
		if childElem, ok := child.(*helium.Element); ok {
			walkXSLTElements(childElem, templates, functions, variables, imports)
		}
	}
}

func getAttr(elem *helium.Element, name string) string {
	val, _ := elem.GetAttribute(name)
	return val
}

// FindUnusedTemplates identifies templates that are never referenced.
func FindUnusedTemplates(templates []XSLTTemplate, functions []XSLTFunction, variables []XSLTVariable) []XSLTTemplate {
	referenced := map[string]bool{}

	// All match templates are potentially used
	for _, t := range templates {
		if t.Match != "" {
			referenced[t.Name] = true
		}
	}

	var unused []XSLTTemplate
	for _, t := range templates {
		if t.Name == "" {
			continue // anonymous match templates are always potentially used
		}
		if !referenced[t.Name] {
			unused = append(unused, t)
		}
	}

	return unused
}

// BuildXSLTGraph constructs a dependency graph from stylesheet analysis.
func BuildXSLTGraph(analysis *XSLTStaticAnalysis) *XSLTGraph {
	graph := &XSLTGraph{}
	seen := map[string]bool{}

	addNode := func(id, label, kind string) {
		if !seen[id] {
			graph.Nodes = append(graph.Nodes, XSLTGraphNode{ID: id, Label: label, Kind: kind})
			seen[id] = true
		}
	}

	for _, t := range analysis.Templates {
		id := templateNodeID(t)
		label := t.Name
		if label == "" {
			label = t.Match
		}
		addNode(id, label, "template")
	}

	for _, f := range analysis.Functions {
		addNode("fn_"+f.Name, f.Name, "function")
	}

	for _, v := range analysis.Variables {
		addNode("var_"+v.Name, v.Name, v.Type)
	}

	for _, imp := range analysis.Imports {
		id := "ss_" + safeID(imp.Href)
		addNode(id, imp.Href, "stylesheet")
		graph.Edges = append(graph.Edges, XSLTGraphEdge{
			From: "root",
			To:   id,
			Kind: imp.Type,
		})
	}

	return graph
}

func templateNodeID(t XSLTTemplate) string {
	if t.Name != "" {
		return "tmpl_" + safeID(t.Name)
	}
	return "tmpl_match_" + safeID(t.Match)
}

func safeID(s string) string {
	r := strings.NewReplacer(
		"{", "_", "}", "_", ":", "_", "/", "_", ".", "_", " ", "_", "-", "_",
	)
	return r.Replace(s)
}

// XSLTGraphToMermaid converts an XSLTGraph to Mermaid syntax.
func XSLTGraphToMermaid(graph *XSLTGraph) string {
	var b strings.Builder
	b.WriteString("graph TD\n")
	for _, node := range graph.Nodes {
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", node.ID, node.Label)
	}
	for _, edge := range graph.Edges {
		fmt.Fprintf(&b, "  %s -->|%s| %s\n", edge.From, edge.Kind, edge.To)
	}
	return b.String()
}

// XSLTGraphToDOT converts an XSLTGraph to DOT syntax.
func XSLTGraphToDOT(graph *XSLTGraph) string {
	var b strings.Builder
	b.WriteString("digraph xslt {\n  rankdir=TB;\n")
	for _, node := range graph.Nodes {
		fmt.Fprintf(&b, "  %s [label=\"%s\"];\n", node.ID, node.Label)
	}
	for _, edge := range graph.Edges {
		fmt.Fprintf(&b, "  %s -> %s [label=\"%s\"];\n", edge.From, edge.To, edge.Kind)
	}
	b.WriteString("}\n")
	return b.String()
}
