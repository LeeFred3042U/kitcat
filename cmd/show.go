package main

import (
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleShowObject(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s show-object <hash>\n", app.Name)
		os.Exit(exitUsage)
	}

	hash := args[0]

	// Architectural fix: Resolve "HEAD" to its underlying commit hash
	if hash == "HEAD" {
		resolvedHash, err := core.ResolveHead()
		if err != nil {
			die("cannot resolve HEAD: %v", err)
		}
		if resolvedHash == "" {
			die("cannot resolve HEAD: ref is empty or invalid")
		}
		hash = resolvedHash
	}

	if err := core.ShowObject(hash); err != nil {
		die("%v", err)
	}
}
