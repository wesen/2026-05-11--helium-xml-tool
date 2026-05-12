//go:build ignore

// Script: debug-serve-help-package-name.go
// Purpose: Reproduce the bug where xml serve-help SPA shows 0 sections
//          because SetDefaultPackage("xml") doesn't persist through
//          to the server handler.
//
// Run:  cd /home/manuel/code/wesen/2026-05-11--helium-xml-tool && go run ttmp/.../scripts/debug-serve-help-package-name.go
//
// What it tests:
//   1. Create HelpSystem, load docs, call SetDefaultPackage("xml")
//   2. Verify sections have packageName="xml" directly via Store.Find
//   3. Start HTTP server with helpserver.NewServeHandler
//   4. Hit /api/packages and /api/sections?package=xml
//   5. Check if the SPA filter finds the sections

package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-go-golems/glazed/pkg/help"
	helpserver "github.com/go-go-golems/glazed/pkg/help/server"
	"github.com/go-go-golems/glazed/pkg/help/store"
	"github.com/go-go-golems/glazed/pkg/web"
	"github.com/go-go-golems/xml/doc"
)

const addr = ":19884"

func main() {
	hs := help.NewHelpSystem()
	if err := doc.AddDocsToHelpSystem(hs); err != nil {
		fmt.Printf("ERROR AddDocsToHelpSystem: %v\n", err)
		return
	}

	// Set default package — this is what root.go does
	if err := hs.Store.SetDefaultPackage(context.Background(), "xml", ""); err != nil {
		fmt.Printf("ERROR SetDefaultPackage: %v\n", err)
		return
	}

	// Verify directly
	sections, _ := hs.Store.Find(context.Background(), func(qc *store.QueryCompiler) {})
	fmt.Printf("=== Direct store query (%d sections) ===\n", len(sections))
	for _, s := range sections {
		fmt.Printf("  slug=%-20s packageName=%q\n", s.Slug, s.PackageName)
	}

	// Start HTTP server
	spaHandler, err := web.NewSPAHandler()
	if err != nil {
		fmt.Printf("ERROR NewSPAHandler: %v\n", err)
		return
	}
	deps := helpserver.HandlerDeps{Store: hs.Store}
	handler := helpserver.NewServeHandler(deps, spaHandler)

	go func() {
		_ = http.ListenAndServe(addr, handler)
	}()

	time.Sleep(500 * time.Millisecond)

	// Test API endpoints
	fmt.Println("\n=== HTTP API tests ===")

	packages := fetch("/api/packages")
	fmt.Printf("GET /api/packages: %s\n", packages)

	allSections := fetch("/api/sections")
	fmt.Printf("GET /api/sections: %s\n", trunc(allSections, 200))

	xmlSections := fetch("/api/sections?package=xml")
	fmt.Printf("GET /api/sections?package=xml: %s\n", trunc(xmlSections, 200))

	defaultSections := fetch("/api/sections?package=default")
	fmt.Printf("GET /api/sections?package=default: %s\n", trunc(defaultSections, 200))

	_ = sections // silence unused warning
}

func fetch(path string) string {
	resp, err := http.Get("http://localhost" + addr + path)
	if err != nil {
		return fmt.Sprintf("ERROR: %v", err)
	}
	defer resp.Body.Close()
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	return string(buf[:n])
}

func trunc(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
