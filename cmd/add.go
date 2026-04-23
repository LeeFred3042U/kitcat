package main

import (
	"fmt"
	"os"
	"flag"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleAdd(args []string) {
	fs := flag.NewFlagSet("add", flag.ExitOnError)

	all := fs.Bool("A", false, "add all")
	fs.BoolVar(all, "all", false, "add all")

	_ = fs.Parse(args)
	rest := fs.Args()

	if *all || (len(rest) == 1 && rest[0] == ".") {
		if err := core.AddAll(); err != nil {
			die("add all failed: %v", err)
		}
		return
	}

	if len(rest) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s add <pathspec>...\n", app.Name)
		os.Exit(exitUsage)
	}

	hasError := false
	for _, path := range rest {
		if err := core.AddFile(path); err != nil {
			fmt.Fprintf(os.Stderr, "error: adding '%s' failed: %v\n", path, err)
			hasError = true
		}
	}

	if hasError {
		os.Exit(exitFailure)
	}
}
