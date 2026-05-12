package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/catalog"
)

// ParseOptions holds the configuration for constructing a helium.Parser.
type ParseOptions struct {
	BaseURI       string
	CatalogFiles  []string
	NoNetwork     bool
	Recover       bool
	SubstituteEnt bool
	LoadDTD       bool
	ValidateDTD   bool
	StripBlanks   bool
	CleanNS       bool
	MergeCDATA    bool
	RelaxLimits   bool
}

// NewParser constructs a helium.Parser from ParseOptions.
func NewParser(opts ParseOptions) (helium.Parser, *catalog.Catalog, error) {
	p := helium.NewParser()

	if opts.BaseURI != "" {
		p = p.BaseURI(opts.BaseURI)
	}
	if opts.Recover {
		p = p.RecoverOnError(true)
	}
	if opts.SubstituteEnt {
		p = p.SubstituteEntities(true)
	}
	if opts.LoadDTD {
		p = p.LoadExternalDTD(true)
	}
	if opts.ValidateDTD {
		p = p.ValidateDTD(true)
	}
	if opts.StripBlanks {
		p = p.StripBlanks(true)
	}
	if opts.CleanNS {
		p = p.CleanNamespaces(true)
	}
	if opts.MergeCDATA {
		p = p.MergeCDATA(true)
	}
	if opts.NoNetwork {
		p = p.AllowNetwork(false)
	}
	if opts.RelaxLimits {
		p = p.RelaxLimits(true)
	}

	var cat *catalog.Catalog
	if len(opts.CatalogFiles) > 0 {
		loaded, err := LoadCatalogs(context.Background(), opts.CatalogFiles)
		if err != nil {
			return helium.Parser{}, nil, fmt.Errorf("loading catalogs: %w", err)
		}
		cat = loaded
		p = p.Catalog(cat)
	}

	return p, cat, nil
}

// LoadCatalogs loads one or more OASIS XML Catalog files and returns
// the first successfully loaded catalog.
func LoadCatalogs(ctx context.Context, paths []string) (*catalog.Catalog, error) {
	for _, path := range paths {
		abs, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		cat, err := catalog.Load(ctx, abs)
		if err != nil {
			continue
		}
		return cat, nil
	}
	return nil, fmt.Errorf("no valid catalog found in: %v", paths)
}

// ReadInput reads an XML document from a file path or stdin (if path is "-").
func ReadInput(path string) ([]byte, error) {
	if path == "-" {
		return os.ReadFile("/dev/stdin")
	}
	return os.ReadFile(path)
}

// ParseDocument reads and parses an XML document, returning the DOM tree
// and timing information if requested.
func ParseDocument(ctx context.Context, p helium.Parser, path string, timing bool) (*helium.Document, time.Duration, error) {
	buf, err := ReadInput(path)
	if err != nil {
		return nil, 0, err
	}

	var t0 time.Time
	if timing {
		t0 = time.Now()
	}

	doc, err := p.Parse(ctx, buf)

	var dur time.Duration
	if timing {
		dur = time.Since(t0)
	}

	if err != nil {
		return doc, dur, err
	}
	return doc, dur, nil
}

// CollectFiles expands a list of paths that may include directories and "."
// into individual XML file paths. Directories are walked recursively.
// Files are included if they have a .xml extension (case-insensitive).
func CollectFiles(paths []string, all bool) ([]string, error) {
	if !all && len(paths) == 0 {
		return nil, fmt.Errorf("no input files specified (use --all for directory walk)")
	}

	var files []string
	seen := map[string]bool{}

	for _, path := range paths {
		if path == "-" {
			files = append(files, path)
			continue
		}

		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("cannot access %s: %w", path, err)
		}

		if !info.IsDir() {
			if !seen[path] {
				files = append(files, path)
				seen[path] = true
			}
			continue
		}

		// Walk directory for XML files
		err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if strings.HasSuffix(strings.ToLower(d.Name()), ".xml") {
				if !seen[p] {
					files = append(files, p)
					seen[p] = true
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return files, nil
}
