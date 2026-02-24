package main

import (
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleMerge(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: kitcat merge <branch>")
		os.Exit(exitUsage)
	}
	if err := core.Merge(args[0]); err != nil {
		die("%v", err)
	}
}
