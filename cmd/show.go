package main

import (
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleShowObject(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s show-object <hash>\n", app.Name)
		os.Exit(exitUsage)
	}
	if err := core.ShowObject(args[0]); err != nil {
		die("%v", err)
	}
}
