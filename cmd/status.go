package main

import (
	"fmt"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleStatus(args []string) {
	out, err := core.Status()
	if err != nil {
		die("status failed: %v", err)
	}
	fmt.Println(out)
}
