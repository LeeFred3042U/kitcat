package main

import (
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleInit(args []string) {
	if err := core.Init(); err != nil {
		die("init failed: %v", err)
	}
	fmt.Fprintf(os.Stderr, "Initialized empty %s repository\n", app.Name)
}
