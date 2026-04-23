package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleReset(args []string) {
	fs := flag.NewFlagSet("reset", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	hard := fs.Bool("hard", false, "Reset index and working tree")
	soft := fs.Bool("soft", false, "Reset only HEAD")
	_ = fs.Bool("mixed", false, "Reset HEAD and index (default)")

	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	fsArgs := fs.Args()

	for i, arg := range args {
		if arg == "--" {
			paths := args[i+1:]

			commit := "HEAD"
			if len(fsArgs) > 0 {
				commit = fsArgs[0]
			}

			if err := core.UnstageFile(commit, paths); err != nil {
				die("%v", err)
			}
			return
		}
	}

	if *hard && *soft {
		fmt.Fprintln(os.Stderr, "fatal: --hard and --soft are mutually exclusive")
		os.Exit(exitUsage)
	}

	commit := "HEAD"
	if len(fsArgs) > 0 {
		commit = fsArgs[0]
	}

	mode := core.ResetMixed
	if *hard {
		mode = core.ResetHard
	} else if *soft {
		mode = core.ResetSoft
	}

	if err := core.Reset(commit, mode); err != nil {
		die("%v", err)
	}
}
