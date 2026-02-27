package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleMv(args []string) {
	fs := flag.NewFlagSet("mv", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	force := fs.Bool("f", false, "Force move/rename")

	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	if fs.NArg() != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s mv <source> <destination>\n", app.Name)
		os.Exit(exitUsage)
	}

	if err := core.MoveFile(fs.Arg(0), fs.Arg(1), *force); err != nil {
		die("%v", err)
	}
	fmt.Fprintf(os.Stderr, "Renamed '%s' to '%s'\n", fs.Arg(0), fs.Arg(1))
}
