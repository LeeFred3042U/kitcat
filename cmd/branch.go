package main

import (
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleBranch(args []string) {
	if len(args) == 0 {
		if err := core.ListBranches(); err != nil {
			die("%v", err)
		}
		return
	}

	if args[0] == "-d" || args[0] == "--delete" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: kitcat branch -d <branchname>")
			os.Exit(exitUsage)
		}
		if err := core.DeleteBranch(args[1]); err != nil {
			die("%v", err)
		}
		fmt.Fprintf(os.Stderr, "Deleted branch %s\n", args[1])
		return
	}

	if args[0] == "-m" || args[0] == "--move" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: kitcat branch -m <newname>")
			os.Exit(exitUsage)
		}
		if err := core.RenameCurrentBranch(args[1]); err != nil {
			die("%v", err)
		}
		fmt.Fprintf(os.Stderr, "Renamed current branch to %s\n", args[1])
		return
	}

	if err := core.CreateBranch(args[0]); err != nil {
		die("%v", err)
	}
	fmt.Fprintf(os.Stderr, "Created branch %s\n", args[0])
}
