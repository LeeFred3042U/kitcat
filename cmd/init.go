package main

import (
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleInit(args []string) {
	if err := core.Init(); err != nil {
		die("init failed: %v", err)
	}
	fmt.Fprintln(os.Stderr, "Initialized empty kitcat repository")
}
