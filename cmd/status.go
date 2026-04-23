package main

import (
	"flag"
	"fmt"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	_ = fs.Parse(args)

	out, err := core.Status()
	if err != nil {
		die("status failed: %v", err)
	}
	fmt.Println(out)
}
