package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lestrrat-go/helium"
)

// InferOptions configures schema inference behavior.
type InferOptions struct {
	InputFiles   []string // XML files to analyze
	OutputType   string   // "xsd" (only XSD supported for now)
	RootElement  string   // force root element name (empty = auto-detect)
	TargetNS     string   // target namespace for generated schema
	MinFrequency float64  // minimum relative frequency (0-1) to include optional elements
	SimpleTypes  bool     // attempt to infer simple types (int, decimal, date, etc.)
	Verbose      bool     // include occurrence counts in documentation
}

// InferredSchema holds the result of schema inference.
type InferredSchema struct {
	TargetNS      string                   `json:"target-namespace"`
	RootElement   string                   `json:"root-element"`
	Elements      map[string]*InferredElem `json:"elements"`
	GlobalAttrs   []InferredAttr           `json:"global-attributes"`
	SourceFiles   int                      `json:"source-files"`
	TotalElements int                      `json:"total-elements"`
}

// InferredElem represents an inferred element declaration.
type InferredElem struct {
	Name       string           `json:"name"`
	Namespace  string           `json:"namespace"`
	MinOccurs  int              `json:"min-occurs"`
	MaxOccurs  int              `json:"max-occurs"` // -1 = unbounded
	Attributes []InferredAttr   `json:"attributes"`
	Children   []InferredChild  `json:"children"`
	TextType   string           `json:"text-type"`   // "string", "integer", "decimal", "date", "boolean", "mixed"
	HasText    bool             `json:"has-text"`
	Count      int              `json:"count"`       // number of times seen
	EnumValues []string         `json:"enum-values"` // unique text values (if few enough)
}

// InferredAttr represents an inferred attribute declaration.
type InferredAttr struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Required  bool     `json:"required"`
	Type      string   `json:"type"`
	Values    []string `json:"values"` // observed values
}

// InferredChild represents an inferred child element reference.
type InferredChild struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	MinOccurs int    `json:"min-occurs"`
	MaxOccurs int    `json:"max-occurs"`
	Count     int    `json:"count"` // times seen in parent context
}

// InferSchema analyzes a set of XML documents and produces an inferred schema.
func InferSchema(ctx context.Context, opts InferOptions) (*InferredSchema, error) {
	if len(opts.InputFiles) == 0 {
		return nil, fmt.Errorf("no input files specified")
	}

	schema := &InferredSchema{
		TargetNS: opts.TargetNS,
		Elements: make(map[string]*InferredElem),
	}

	// Collect all XML files
	var files []string
	for _, path := range opts.InputFiles {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("cannot access %s: %w", path, err)
		}
		if info.IsDir() {
			err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".xml") {
					files = append(files, p)
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			files = append(files, path)
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no XML files found")
	}

	// Parse each document and analyze
	for _, file := range files {
		buf, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", file, err)
		}

		parser := helium.NewParser()
		doc, err := parser.Parse(ctx, buf)
		if err != nil {
			continue // skip malformed documents
		}
		if doc == nil {
			continue
		}

		schema.SourceFiles++

		// Auto-detect root element
		root := doc.DocumentElement()
		if root == nil {
			continue
		}

		if schema.RootElement == "" {
			if opts.RootElement != "" {
				schema.RootElement = opts.RootElement
			} else {
				schema.RootElement = root.LocalName()
			}
		}

		// Walk the document tree
		walkElement(root, schema, opts)
	}

	if schema.SourceFiles == 0 {
		return nil, fmt.Errorf("no valid XML documents found in input files")
	}

	// Finalize: determine cardinalities from counts
	finalizeCardinalities(schema)

	return schema, nil
}

func walkElement(elem *helium.Element, schema *InferredSchema, opts InferOptions) {
	name := elem.LocalName()
	ns := ""
	if elemNs := elem.Namespace(); elemNs != nil {
		ns = elemNs.URI()
	}
	key := elementKey(name, ns)

	ie, ok := schema.Elements[key]
	if !ok {
		ie = &InferredElem{
			Name:      name,
			Namespace: ns,
			TextType:  "string",
		}
		schema.Elements[key] = ie
	}
	ie.Count++
	schema.TotalElements++

	// Analyze text content
	for child := elem.FirstChild(); child != nil; child = child.NextSibling() {
		if text, ok := child.(*helium.Text); ok {
			content := strings.TrimSpace(string(text.Content()))
			if content != "" {
				ie.HasText = true
				if opts.SimpleTypes {
					ie.TextType = inferSimpleType(content, ie.TextType)
				}
				// Track enum values (only if few unique values seen)
				if len(ie.EnumValues) < 50 {
					found := false
					for _, v := range ie.EnumValues {
						if v == content {
							found = true
							break
						}
					}
					if !found {
						ie.EnumValues = append(ie.EnumValues, content)
					}
				}
			}
		}
	}

	// Analyze attributes
	elem.ForEachAttribute(func(attr *helium.Attribute) bool {
		attrName := attr.LocalName()
		attrNS := attr.URI()
		attrVal := attr.Value()

		found := false
		for i := range ie.Attributes {
			if ie.Attributes[i].Name == attrName && ie.Attributes[i].Namespace == attrNS {
				// Update observed values
				if len(ie.Attributes[i].Values) < 50 {
					vFound := false
					for _, v := range ie.Attributes[i].Values {
						if v == attrVal {
							vFound = true
							break
						}
					}
					if !vFound {
						ie.Attributes[i].Values = append(ie.Attributes[i].Values, attrVal)
					}
				}
				found = true
				break
			}
		}
		if !found {
			ia := InferredAttr{
				Name:      attrName,
				Namespace: attrNS,
				Required:  true, // assume required until proven optional
				Type:      "string",
			}
			if attrVal != "" {
				ia.Values = append(ia.Values, attrVal)
			}
			ie.Attributes = append(ie.Attributes, ia)
		}
		return true
	})

	// Analyze child elements
	childCounts := map[string]int{}
	for child := elem.FirstChild(); child != nil; child = child.NextSibling() {
		if childElem, ok := child.(*helium.Element); ok {
			childName := childElem.LocalName()
			childNS := ""
			if childNs := childElem.Namespace(); childNs != nil {
				childNS = childNs.URI()
			}
			childCounts[elementKey(childName, childNS)]++
			walkElement(childElem, schema, opts)
		}
	}

	// Update child info
	for childKey, count := range childCounts {
		found := false
		for i := range ie.Children {
			if elementKey(ie.Children[i].Name, ie.Children[i].Namespace) == childKey {
				ie.Children[i].Count += count
				if count > ie.Children[i].MaxOccurs {
					ie.Children[i].MaxOccurs = count
				}
				found = true
				break
			}
		}
		if !found {
			cName, cNS := splitElementKey(childKey)
			ie.Children = append(ie.Children, InferredChild{
				Name:      cName,
				Namespace: cNS,
				MinOccurs: 1,
				MaxOccurs: count,
				Count:     count,
			})
		}
	}
}

func finalizeCardinalities(schema *InferredSchema) {
	for _, ie := range schema.Elements {
		// If enum values are > 10, don't treat as enumeration
		if len(ie.EnumValues) > 10 {
			ie.EnumValues = nil
		} else if len(ie.EnumValues) > 0 {
			sort.Strings(ie.EnumValues)
		}

		// Determine attribute optionality
		for i := range ie.Attributes {
			attrObs := len(ie.Attributes[i].Values)
			if ie.Count > 1 && attrObs < ie.Count {
				ie.Attributes[i].Required = false
			}
		}

		// Finalize children cardinality
		for i := range ie.Children {
			child := &ie.Children[i]
			if child.Count < ie.Count {
				child.MinOccurs = 0
			}
			if child.MaxOccurs > 1 {
				child.MaxOccurs = -1 // unbounded
			}
		}
	}
}

func inferSimpleType(value, currentType string) string {
	if currentType == "string" {
		return "string"
	}

	// Try boolean
	if currentType == "" || currentType == "boolean" {
		if value == "true" || value == "false" || value == "1" || value == "0" {
			if currentType == "" {
				return "boolean"
			}
			return currentType
		}
		if currentType == "boolean" {
			currentType = "integer"
		}
	}

	// Try integer
	if currentType == "" || currentType == "integer" {
		isInt := true
		for i, c := range value {
			if !((c >= '0' && c <= '9') || (i == 0 && c == '-')) {
				isInt = false
				break
			}
		}
		if isInt && len(value) > 0 {
			if currentType == "" {
				return "integer"
			}
			return currentType
		}
		if currentType == "integer" {
			currentType = "decimal"
		}
	}

	// Try decimal
	if currentType == "" || currentType == "decimal" {
		isDecimal := true
		hasDot := false
		for i, c := range value {
			if c == '.' && !hasDot {
				hasDot = true
				continue
			}
			if !((c >= '0' && c <= '9') || (i == 0 && c == '-')) {
				isDecimal = false
				break
			}
		}
		if isDecimal && len(value) > 0 {
			if currentType == "" {
				return "decimal"
			}
			return currentType
		}
		if currentType == "decimal" {
			currentType = "string"
		}
	}

	return "string"
}

// InferredSchemaToXSD converts an InferredSchema to an XSD document string.
func InferredSchemaToXSD(schema *InferredSchema) string {
	var b strings.Builder

	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString("\n")

	if schema.TargetNS != "" {
		fmt.Fprintf(&b, "<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\"\n")
		fmt.Fprintf(&b, "  targetNamespace=\"%s\"\n", schema.TargetNS)
		fmt.Fprintf(&b, "  xmlns:tns=\"%s\"\n", schema.TargetNS)
		fmt.Fprintf(&b, "  elementFormDefault=\"qualified\">\n\n")
	} else {
		b.WriteString("<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n\n")
	}

	// Root element
	if schema.RootElement != "" {
		rootKey := elementKey(schema.RootElement, schema.TargetNS)
		if _, ok := schema.Elements[rootKey]; ok {
			if schema.TargetNS != "" {
				fmt.Fprintf(&b, "  <xs:element name=\"%s\" type=\"tns:%sType\"/>\n\n",
					schema.RootElement, schema.RootElement)
			} else {
				fmt.Fprintf(&b, "  <xs:element name=\"%s\" type=\"%sType\"/>\n\n",
					schema.RootElement, schema.RootElement)
			}
		}
	}

	// Emit each element as a complex type
	for _, key := range SortedElementKeys(schema.Elements) {
		ie := schema.Elements[key]
		emitInferredElementType(&b, ie, schema)
	}

	b.WriteString("</xs:schema>\n")
	return b.String()
}

func emitInferredElementType(b *strings.Builder, ie *InferredElem, schema *InferredSchema) {
	fmt.Fprintf(b, "  <xs:complexType name=\"%sType\">\n", ie.Name)

	if len(ie.Children) > 0 || ie.HasText {
		if ie.HasText && len(ie.Children) > 0 {
			b.WriteString("    <xs:complexContent>\n")
			b.WriteString("      <xs:extension base=\"xs:anyType\">\n")
		}

		if len(ie.Children) > 0 {
			b.WriteString("    <xs:sequence>\n")
			for _, child := range ie.Children {
				min := fmt.Sprintf(" minOccurs=\"%d\"", child.MinOccurs)
				max := ""
				if child.MaxOccurs == -1 {
					max = " maxOccurs=\"unbounded\""
				} else if child.MaxOccurs > 1 {
					max = fmt.Sprintf(" maxOccurs=\"%d\"", child.MaxOccurs)
				}
				fmt.Fprintf(b, "      <xs:element name=\"%s\"%s%s/>\n", child.Name, min, max)
			}
			b.WriteString("    </xs:sequence>\n")
		}

		if ie.HasText && len(ie.Children) == 0 {
			fmt.Fprintf(b, "    <xs:simpleContent>\n")
			fmt.Fprintf(b, "      <xs:extension base=\"xs:%s\"/>\n", mapToXSDType(ie.TextType))
			fmt.Fprintf(b, "    </xs:simpleContent>\n")
		}

		if ie.HasText && len(ie.Children) > 0 {
			b.WriteString("      </xs:extension>\n")
			b.WriteString("    </xs:complexContent>\n")
		}
	}

	// Attributes
	for _, attr := range ie.Attributes {
		use := ""
		if !attr.Required {
			use = " use=\"optional\""
		}
		if len(attr.Values) > 0 && len(attr.Values) <= 10 {
			fmt.Fprintf(b, "    <xs:attribute name=\"%s\"%s>\n", attr.Name, use)
			b.WriteString("      <xs:simpleType>\n")
			b.WriteString("        <xs:restriction base=\"xs:string\">\n")
			for _, v := range attr.Values {
				fmt.Fprintf(b, "          <xs:enumeration value=\"%s\"/>\n", escapeXML(v))
			}
			b.WriteString("        </xs:restriction>\n")
			b.WriteString("      </xs:simpleType>\n")
			b.WriteString("    </xs:attribute>\n")
		} else {
			fmt.Fprintf(b, "    <xs:attribute name=\"%s\" type=\"xs:string\"%s/>\n",
				attr.Name, use)
		}
	}

	b.WriteString("  </xs:complexType>\n\n")
}

func mapToXSDType(t string) string {
	switch t {
	case "boolean":
		return "boolean"
	case "integer":
		return "integer"
	case "decimal":
		return "decimal"
	default:
		return "string"
	}
}

func escapeXML(s string) string {
	return strings.ReplaceAll(s, `"`, "&quot;")
}

func elementKey(name, ns string) string {
	if ns == "" {
		return name
	}
	return "{" + ns + "}" + name
}

func splitElementKey(key string) (string, string) {
	if strings.HasPrefix(key, "{") {
		idx := strings.Index(key, "}")
		if idx >= 0 {
			return key[idx+1:], key[1:idx]
		}
	}
	return key, ""
}

// SortedElementKeys returns the keys of the inferred elements map in sorted order.
func SortedElementKeys(m map[string]*InferredElem) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
