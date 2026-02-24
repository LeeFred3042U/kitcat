package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleClean(args []string) {
	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	dryRun := fs.Bool("n", false, "Dry run")
	force := fs.Bool("f", false, "Force")
	removeDirs := fs.Bool("d", false, "Remove untracked directories in addition to untracked files")
	removeIgnored := fs.Bool("x", false, "Remove ignored files, as well as untracked files")
	onlyIgnored := fs.Bool("X", false, "Remove only files ignored by Git")

	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	if !*force && !*dryRun {
		fmt.Fprintln(os.Stderr, "fatal: clean.requireForce defaults to true and neither -n nor -f given; refusing to clean")
		os.Exit(exitFailure)
	}

	if err := core.Clean(*dryRun, *removeDirs, *removeIgnored, *onlyIgnored); err != nil {
		die("%v", err)
	}
}