package main

import (
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleLsFiles(args []string) {
	if err := core.ListFiles(); err != nil {
		die("%v", err)
	}
}
