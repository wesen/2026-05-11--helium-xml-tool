package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/xml/pkg/engine"
)

func TestLoadFromDir_Valid(t *testing.T) {
	dir := t.TempDir()
	tomlContent := `
[xml]
version = 1

[catalog]
files = ["catalog.xml"]

[validation.invoice]
description = "Invoice validation"
files = "invoices/**/*.xml"

[[validation.invoice.steps]]
type = "xsd"
schema = "schemas/invoice.xsd"

[[validation.invoice.steps]]
type = "schematron"
schema = "rules/invoice-rules.sch"
`
	if err := os.WriteFile(filepath.Join(dir, "xml.toml"), []byte(tomlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if cfg == nil {
		t.Fatal("cfg is nil")
	}
	if len(cfg.Profiles) != 1 {
		t.Errorf("expected 1 profile, got %d", len(cfg.Profiles))
	}
	profile, ok := cfg.Profiles["invoice"]
	if !ok {
		t.Fatal("invoice profile not found")
	}
	if profile.Description != "Invoice validation" {
		t.Errorf("Description = %q", profile.Description)
	}
	if len(profile.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(profile.Steps))
	}
	if profile.Steps[0].Type != "xsd" {
		t.Errorf("step[0].Type = %q, want xsd", profile.Steps[0].Type)
	}
	if profile.Steps[1].Type != "schematron" {
		t.Errorf("step[1].Type = %q, want schematron", profile.Steps[1].Type)
	}
}

func TestLoadFromDir_NoFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir with no file: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil config when no xml.toml exists")
	}
}

func TestLoadFromDir_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "xml.toml"), []byte("not = valid toml {{{"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFromDir(dir)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}

func TestConfig_GetProfile(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]*ValidationProfile{
			"test": {
				Description: "Test",
				Steps: []engine.ValidationStep{
					{Type: "xsd", SchemaFile: "test.xsd"},
				},
			},
		},
	}

	steps, err := cfg.GetProfile("test")
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if len(steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Type != "xsd" {
		t.Errorf("step type = %q, want xsd", steps[0].Type)
	}
}

func TestConfig_GetProfile_NotFound(t *testing.T) {
	cfg := &Config{Profiles: map[string]*ValidationProfile{}}
	_, err := cfg.GetProfile("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

func TestConfig_GetProfile_NilConfig(t *testing.T) {
	var cfg *Config
	_, err := cfg.GetProfile("anything")
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestConfig_CatalogFiles(t *testing.T) {
	cfg := &Config{
		Catalog: CatalogConfig{Files: []string{"a.xml", "b.xml"}},
	}
	files := cfg.CatalogFiles()
	if len(files) != 2 {
		t.Errorf("expected 2 catalog files, got %d", len(files))
	}
}

func TestConfig_CatalogFiles_Nil(t *testing.T) {
	var cfg *Config
	files := cfg.CatalogFiles()
	if files != nil {
		t.Errorf("expected nil for nil config, got %v", files)
	}
}

func TestLoadFromDir_MultiProfile(t *testing.T) {
	dir := t.TempDir()
	tomlContent := `
[validation.docbook]
description = "DocBook docs"
files = "docs/**/*.xml"

[[validation.docbook.steps]]
type = "rng"
schema = "schemas/docbook.rng"

[validation.invoice]
description = "Invoice pipeline"
files = "invoices/**/*.xml"

[[validation.invoice.steps]]
type = "xsd"
schema = "schemas/invoice.xsd"
`
	if err := os.WriteFile(filepath.Join(dir, "xml.toml"), []byte(tomlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if len(cfg.Profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(cfg.Profiles))
	}
	if _, ok := cfg.Profiles["docbook"]; !ok {
		t.Error("docbook profile missing")
	}
	if _, ok := cfg.Profiles["invoice"]; !ok {
		t.Error("invoice profile missing")
	}
}
