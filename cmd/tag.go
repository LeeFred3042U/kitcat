package main

import (
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleTag(args []string) {
	if len(args) == 0 {
		if err := core.PrintTags(); err != nil {
			die("%v", err)
		}
		return
	}

	tagName := args[0]
	commit := "HEAD"
	if len(args) > 1 {
		commit = args[1]
	}

	// Architectural fix: Use core.ResolveHead
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

	if err := core.CreateTag(tagName, commit); err != nil {
		die("%v", err)
	}
}
