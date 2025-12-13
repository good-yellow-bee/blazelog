// Package main is the entry point for the blazelog CLI tool.
package main

import (
	"os"

	"github.com/good-yellow-bee/blazelog/cmd/blazectl/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
