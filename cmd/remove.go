package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleRm(args []string) {
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	recursive := fs.Bool("r", false, "Allow recursive removal")
	cached := fs.Bool("cached", false, "Only remove from the index")

	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s rm [-r] [--cached] <file>...\n", app.Name)
		os.Exit(exitUsage)
	}

	for _, file := range fs.Args() {
		if err := core.RemoveFile(file, *recursive, *cached); err != nil {
			die("%v", err)
		}
	}
}
