package main

import (
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleMerge(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s merge <branch> | --abort\n", app.Name)
		os.Exit(exitUsage)
	}

	arg := args[0]

	if arg == "--abort" {
		if err := core.MergeAbort(); err != nil {
			die("%v", err)
		}
		return
	}

	if err := core.Merge(arg); err != nil {
		die("%v", err)
	}
}
