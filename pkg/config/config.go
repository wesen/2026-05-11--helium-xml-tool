package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-go-golems/xml/pkg/engine"
	"github.com/pelletier/go-toml/v2"
)

// Config represents the project-level xml.toml configuration.
type Config struct {
	XML       map[string]interface{}           `toml:"xml,omitempty"`
	Catalog   CatalogConfig                    `toml:"catalog,omitempty"`
	Profiles  map[string]*ValidationProfile     `toml:"validation,omitempty"`
}

// CatalogConfig configures OASIS XML Catalog usage.
type CatalogConfig struct {
	Files []string `toml:"files"`
}

// ValidationProfile is a named, multi-step validation pipeline definition.
type ValidationProfile struct {
	Description string               `toml:"description"`
	Files      string               `toml:"files"`
	Steps      []engine.ValidationStep `toml:"steps"`
}

// LoadFromDir searches for xml.toml in the given directory and loads it.
// Returns nil (no error) if no config file is found.
func LoadFromDir(dir string) (*Config, error) {
	path := filepath.Join(dir, "xml.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading xml.toml: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing xml.toml: %w", err)
	}

	return &cfg, nil
}

// GetProfile returns the validation steps for a named profile.
func (c *Config) GetProfile(name string) ([]engine.ValidationStep, error) {
	if c == nil || c.Profiles == nil {
		return nil, fmt.Errorf("no validation profiles defined in xml.toml")
	}
	profile, ok := c.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("validation profile %q not found in xml.toml", name)
	}
	return profile.Steps, nil
}

// CatalogFiles returns the list of catalog files from the config.
func (c *Config) CatalogFiles() []string {
	if c == nil {
		return nil
	}
	return c.Catalog.Files
}
