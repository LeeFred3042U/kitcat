package main

import (
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleMerge(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s merge <branch>\n", app.Name)
		os.Exit(exitUsage)
	}
	if err := core.Merge(args[0]); err != nil {
		die("%v", err)
	}
}
