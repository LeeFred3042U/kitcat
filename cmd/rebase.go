package main

import (
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleRebase(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: kitcat rebase <branch> [-i | --abort | --continue]")
		os.Exit(exitUsage)
	}

	arg := args[0]

	if arg == "--abort" {
		if err := core.RebaseAbort(); err != nil {
			die("%v", err)
		}
		return
	}
	if arg == "--continue" {
		if err := core.RebaseContinue(); err != nil {
			die("%v", err)
		}
		return
	}

	interactive := false
	target := arg

	if len(args) > 1 && (args[1] == "-i" || args[1] == "--interactive") {
		interactive = true
	}

	if err := core.Rebase(target, interactive); err != nil {
		die("%v", err)
	}
}
