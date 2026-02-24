
package main

import (
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleAdd(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: kitcat add <pathspec>...")
		os.Exit(exitUsage)
	}

	if len(args) == 1 && (args[0] == "." || args[0] == "-A" || args[0] == "--all") {
		if err := core.AddAll(); err != nil {
			die("add all failed: %v", err)
		}
		return
	}

	hasError := false
	for _, path := range args {
		if err := core.AddFile(path); err != nil {
			fmt.Fprintf(os.Stderr, "error: adding '%s' failed: %v\n", path, err)
			hasError = true
		}
	}

	if hasError {
		os.Exit(exitFailure)
	}
}
