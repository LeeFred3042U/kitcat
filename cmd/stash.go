package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleStash(args []string) {
	if len(args) == 0 {
		// default = push
		if err := core.Stash(); err != nil {
			die("%v", err)
		}
		return
	}

	sub := args[0]
	rest := args[1:]

	switch sub {

	case "push":
		fs := flag.NewFlagSet("stash push", flag.ExitOnError)
		msg := fs.String("m", "", "stash message")
		_ = fs.Parse(rest)

		if err := core.StashPush(*msg); err != nil {
			die("%v", err)
		}

	case "pop":
		idx := 0
		if len(rest) > 0 {
			idx = parseStashIndex(rest[0])
		}
		if err := core.StashPop(idx); err != nil {
			die("%v", err)
		}

	case "apply":
		idx := 0
		if len(rest) > 0 {
			idx = parseStashIndex(rest[0])
		}
		if err := core.StashApply(idx); err != nil {
			die("%v", err)
		}

	case "drop":
		idx := 0
		if len(rest) > 0 {
			idx = parseStashIndex(rest[0])
		}
		if err := core.StashDrop(idx); err != nil {
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

	default:
		fmt.Fprintf(os.Stderr,
			"Usage: %s stash [push [-m msg] | pop [stash@{n}] | apply [stash@{n}] | drop [stash@{n}] | list | clear]\n",
			app.Name,
		)
		os.Exit(exitUsage)
	}
}
