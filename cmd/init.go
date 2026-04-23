package main

import (
	"flag"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	quiet := addQuietFlag(fs)

	_ = fs.Parse(args)

	if err := core.Init(); err != nil {
		die("init failed: %v", err)
	}

	printIfNotQuiet(*quiet, "Initialized empty %s repository\n", app.Name)
}
