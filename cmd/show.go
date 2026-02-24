package main

import (
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleShowObject(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: kitcat show-object <hash>")
		os.Exit(exitUsage)
	}
	if err := core.ShowObject(args[0]); err != nil {
		die("%v", err)
	}
}
