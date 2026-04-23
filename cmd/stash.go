package main

import (
	
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleStash(args []string) {
	if len(args) == 0 {
		if err := core.Stash(); err != nil {
			die("%v", err)
		}
		return
	}

	subCmd := args[0]

	switch subCmd {
	case "pop":
		if err := core.StashPop(); err != nil {
			die("%v", err)
		}

	case "list":
		if err := core.StashList(); err != nil {
			die("%v", err)
		}

	case "clear":
		if err := core.StashClear(); err != nil {
			die("%v", err)
		}

	case "apply":
		idx := 0
		if len(args) > 1 {
			idx = parseStashIndex(args[1])
		}
		if err := core.StashApply(idx); err != nil {
			die("%v", err)
		}

	case "drop":
		idx := 0
		if len(args) > 1 {
			idx = parseStashIndex(args[1])
		}
		if err := core.StashDrop(idx); err != nil {
			die("%v", err)
		}

	default:
		fmt.Fprintf(os.Stderr, "Usage: %s stash [apply|drop|pop|list|clear]\n", app.Name)
		os.Exit(exitUsage)
	}
}
