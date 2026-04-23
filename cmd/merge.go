package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleMerge(args []string) {
	fs := flag.NewFlagSet("merge", flag.ExitOnError)
	quiet := addQuietFlag(fs)

	rest := fs.Args()

	if len(rest) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s merge <branch> | --abort\n", app.Name)
		os.Exit(exitUsage)
	}

	arg := rest[0]

	if arg == "--abort" {
		if err := core.MergeAbort(); err != nil {
			die("%v", err)
		}
		return
	}

	if err := core.Merge(arg); err != nil {
		die("%v", err)
	}

	printIfNotQuiet(*quiet, "Merge completed\n")
}
