package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleRebase(args []string) {
	fs := flag.NewFlagSet("rebase", flag.ExitOnError)

	interactive := fs.Bool("i", false, "interactive rebase")
	fs.BoolVar(interactive, "interactive", false, "interactive rebase")

	abort := fs.Bool("abort", false, "abort rebase")
	cont := fs.Bool("continue", false, "continue rebase")

	_ = fs.Parse(args)
	rest := fs.Args()

	if *abort {
		if err := core.RebaseAbort(); err != nil {
			die("%v", err)
		}
		return
	}

	if *cont {
		if err := core.RebaseContinue(); err != nil {
			die("%v", err)
		}
		return
	}

	if len(rest) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s rebase <branch> [-i]\n", app.Name)
		os.Exit(exitUsage)
	}

	target := rest[0]

	if err := core.Rebase(target, *interactive); err != nil {
		die("%v", err)
	}
}
