package main

import (
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleDiff(args []string) {
	// Check for --staged flag
	staged := false
	if len(args) > 0 && (args[0] == "--staged" || args[0] == "--cached") {
		staged = true
	}
	if err := core.Diff(staged); err != nil { // Pass arg
		die("%v", err)
	}
}
