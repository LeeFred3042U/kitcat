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

	// Search for the "--" separator in the post-flag-parse arguments only.
	// Using raw `args` here would include flag tokens (e.g. --hard) in the
	// paths slice when they appear after "--", causing UnstageFile to receive
	// flag strings as file paths.
	for i, arg := range fsArgs {
		if arg == "--" {
			paths := fsArgs[i+1:]

			commit := "HEAD"
			if i > 0 {
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
