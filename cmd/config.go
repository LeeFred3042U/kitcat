package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleConfig(args []string) {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var global bool
	fs.BoolVar(&global, "global", false, "Use global config file")

	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	params := fs.Args()

	if len(params) < 1 {
		if err := core.PrintAllConfig(global); err != nil {
			die("%v", err)
		}
		return
	}

	key := params[0]
	if len(params) == 2 {
		val := params[1]
		if err := core.SetConfig(key, val, global); err != nil {
			die("%v", err)
		}
		return
	}

	val, found, err := core.GetConfig(key)
	if err != nil {
		die("%v", err)
	}
	if found {
		fmt.Println(val)
	} else {
		os.Exit(exitFailure)
	}
}