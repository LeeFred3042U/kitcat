package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleTag(args []string) {
	fs := flag.NewFlagSet("tag", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	annotated := fs.Bool("a", false, "Make an annotated tag object")
	message := fs.String("m", "", "Tag message")
	del := fs.Bool("d", false, "delete tag")

	force := fs.Bool("f", false, "force overwrite tag")
	fs.BoolVar(force, "force", false, "force overwrite tag")

	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	// delete first (separate path)
	if *del {
		if fs.NArg() < 1 {
			fmt.Fprintf(os.Stderr, "Usage: %s tag -d <tagname>\n", app.Name)
			os.Exit(exitUsage)
		}

		name := fs.Arg(0)

		if err := core.DeleteTag(name); err != nil {
			die("%v", err)
		}

		fmt.Fprintf(os.Stderr, "Deleted tag '%s'\n", name)
		return
	}

	// list tags
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

	// resolve HEAD
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

	// create tag (single path)
	if *annotated {
		msg := *message
		if msg == "" {
			msg = "Annotated tag " + tagName
		}
		if err := core.CreateAnnotatedTag(tagName, commit, msg, *force); err != nil {
			die("%v", err)
		}
	} else {
		if err := core.CreateTag(tagName, commit, *force); err != nil {
			die("%v", err)
		}
	}
}
