//go:build ignore

// Script: debug-serve-help-via-root.go
// Purpose: Test the full command path: NewRootCommand() → xml serve-help
//          to see if SetDefaultPackage persists through the Cobra wiring.
//
// Run:  cd /home/manuel/code/wesen/2026-05-11--helium-xml-tool && go run ttmp/.../scripts/debug-serve-help-via-root.go

package main

import (
	"context"
	"fmt"

	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/go-go-golems/glazed/pkg/help/store"
	"github.com/go-go-golems/xml/doc"
)

func main() {
	hs := help.NewHelpSystem()

	// Step 1: Load docs (same as root.go)
	if err := doc.AddDocsToHelpSystem(hs); err != nil {
		fmt.Printf("ERROR AddDocsToHelpSystem: %v\n", err)
		return
	}
	dump("After AddDocsToHelpSystem", hs)

	// Step 2: Set default package (same as root.go)
	if err := hs.Store.SetDefaultPackage(context.Background(), "xml", ""); err != nil {
		fmt.Printf("ERROR SetDefaultPackage: %v\n", err)
		return
	}
	dump("After SetDefaultPackage", hs)

	// Step 3: Check if SetupCobraRootCommand modifies the store
	// (we can't call it without a real cobra.Command, so we skip)
	fmt.Println("\n(SetupCobraRootCommand not called — cannot call with nil cobra.Command)")
}

func dump(label string, hs *help.HelpSystem) {
	sections, _ := hs.Store.Find(context.Background(), func(qc *store.QueryCompiler) {})
	fmt.Printf("\n=== %s (%d sections) ===\n", label, len(sections))
	for _, s := range sections {
		fmt.Printf("  slug=%-20s packageName=%q\n", s.Slug, s.PackageName)
	}
}
