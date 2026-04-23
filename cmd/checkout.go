package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleCheckout(args []string) {
	fs := flag.NewFlagSet("checkout", flag.ExitOnError)

	force := fs.Bool("f", false, "force checkout")
	fs.BoolVar(force, "force", false, "force checkout")

	newBranch := fs.String("b", "", "create and switch to new branch")

	_ = fs.Parse(args)
	rest := fs.Args()

	if *newBranch != "" {
		if err := core.CreateBranch(*newBranch); err != nil {
			die("failed to create branch: %v", err)
		}
		if err := core.Checkout(*newBranch, *force); err != nil {
			die("failed to checkout new branch: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Switched to a new branch '%s'\n", *newBranch)
		return
	}

	if len(rest) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s checkout [-f] <branch>\n", app.Name)
		os.Exit(exitUsage)
	}

	target := rest[0]

	if err := core.Checkout(target, *force); err != nil {
		die("%v", err)
	}

	fmt.Fprintf(os.Stderr, "Checked out '%s'\n", target)
}
