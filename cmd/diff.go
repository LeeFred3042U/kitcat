package main

import (
	"flag"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleDiff(args []string) {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)

	staged := fs.Bool("staged", false, "diff staged changes")
	fs.BoolVar(staged, "cached", false, "diff staged changes")

	_ = fs.Parse(args)

	if err := core.Diff(*staged); err != nil {
		die("%v", err)
	}
}
