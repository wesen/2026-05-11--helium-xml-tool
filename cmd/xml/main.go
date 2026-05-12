package main

import (
	"os"

	"github.com/go-go-golems/xml/pkg/cmds"
)

func main() {
	rootCmd, err := cmds.NewRootCommand()
	if err != nil {
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
