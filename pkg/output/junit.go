package output

import (
	"encoding/xml"
	"io"

	"github.com/go-go-golems/xml/pkg/engine"
)

// JUnitXML represents the top-level JUnit test report.
type JUnitXML struct {
	XMLName   xml.Name      `xml:"testsuites"`
	TestSuite JUnitTestSuite `xml:"testsuite"`
}

// JUnitTestSuite represents a single test suite.
type JUnitTestSuite struct {
	Name      string         `xml:"name,attr"`
	Tests     int            `xml:"tests,attr"`
	Errors    int            `xml:"errors,attr"`
	Failures  int            `xml:"failures,attr"`
	TestCases []JUnitTestCase `xml:"testcase"`
}

// JUnitTestCase represents a single test case.
type JUnitTestCase struct {
	Name      string    `xml:"name,attr"`
	Classname string    `xml:"classname,attr"`
	Failure   *Failure  `xml:"failure,omitempty"`
}

// Failure represents a test failure.
type Failure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

// WriteJUnit writes validation results in JUnit XML format.
func WriteJUnit(results []engine.ValidationResult, w io.Writer) error {
	suite := JUnitTestSuite{
		Name: "xml-validate",
	}

	errorCount := 0
	failureCount := 0

	for _, r := range results {
		tc := JUnitTestCase{
			Name:      r.File,
			Classname: r.SchemaType,
		}

		if r.Severity == "error" {
			errorCount++
			tc.Failure = &Failure{
				Message: r.Message,
				Type:    r.SchemaType,
				Content: r.Message,
			}
		} else if r.Severity == "warning" {
			failureCount++
			tc.Failure = &Failure{
				Message: r.Message,
				Type:    "warning",
				Content: r.Message,
			}
		}

		suite.TestCases = append(suite.TestCases, tc)
	}

	suite.Tests = len(results)
	suite.Errors = errorCount
	suite.Failures = failureCount

	junit := JUnitXML{TestSuite: suite}

	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(junit); err != nil {
		return err
	}
	_, err := w.Write([]byte("\n"))
	return err
}
