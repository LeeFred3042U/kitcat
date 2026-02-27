package main

import (
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleCheckout(args []string) {
	force := false
	var cleanArgs []string

	for _, arg := range args {
		if arg == "-f" || arg == "--force" {
			force = true
		} else {
			cleanArgs = append(cleanArgs, arg)
		}
	}

	if len(cleanArgs) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s checkout [-f] <branch> | [-f] -b <new_branch>\n", app.Name)
		os.Exit(exitUsage)
	}

	if cleanArgs[0] == "-b" {
		if len(cleanArgs) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: %s checkout [-f] -b <new_branch>\n", app.Name)
			os.Exit(exitUsage)
		}
		newBranch := cleanArgs[1]
		if err := core.CreateBranch(newBranch); err != nil {
			die("failed to create branch: %v", err)
		}
		
		if err := core.Checkout(newBranch, force); err != nil {
			die("failed to checkout new branch: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Switched to a new branch '%s'\n", newBranch)
		return
	}

	arg := cleanArgs[0]
	if err := core.Checkout(arg, force); err != nil {
		die("%v", err)
	}

	fmt.Fprintf(os.Stderr, "Checked out '%s'\n", arg)
}
