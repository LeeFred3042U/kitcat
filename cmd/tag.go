package main

import (
	"flag"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleTag(args []string) {
	fs := flag.NewFlagSet("tag", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	annotated := fs.Bool("a", false, "Make an unsigned, annotated tag object")
	message := fs.String("m", "", "Tag message")

	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	if fs.NArg() == 0 {
		if err := core.PrintTags(); err != nil {
			die("%v", err)
		}
		return
	}

	tagName := fs.Arg(0)
	commit := "HEAD"
	if fs.NArg() > 1 {
		commit = fs.Arg(1)
	}

	// Resolve "HEAD" to the actual commit hash
	if commit == "HEAD" {
		headHash, err := core.ResolveHead()
		if err != nil {
			die("cannot resolve HEAD: %v", err)
		}
		if headHash == "" {
			die("cannot resolve HEAD: ref is empty or invalid")
		}
		commit = headHash
	}

	// Route based on the -a flag
	if *annotated {
		msg := *message
		if msg == "" {
			// If no message provided, use a default (or we could open an editor here)
			msg = "Annotated tag " + tagName
		}
		if err := core.CreateAnnotatedTag(tagName, commit, msg); err != nil {
			die("%v", err)
		}
	} else {
		if err := core.CreateTag(tagName, commit); err != nil {
			die("%v", err)
		}
	}
}
