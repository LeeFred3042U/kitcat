package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleBranch(args []string) {
	fs := flag.NewFlagSet("branch", flag.ExitOnError)

	del := fs.Bool("d", false, "delete branch")
	fs.BoolVar(del, "delete", false, "delete branch")

	move := fs.Bool("m", false, "rename branch")
	fs.BoolVar(move, "move", false, "rename branch")

	verbose := fs.Bool("v", false, "show last commit")
	forceDel := fs.Bool("D", false, "force delete branch")

	_ = fs.Parse(args)
	rest := fs.Args()

	// list branches
	if !*del && !*move && len(rest) == 0 {
		if err := core.ListBranches(*verbose); err != nil {
			die("%v", err)
		}
		return
	}

	// delete branch
	if *del || *forceDel {
		if len(rest) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: %s branch -d <branchname>\n", app.Name)
			os.Exit(exitUsage)
		}
		if err := core.DeleteBranch(rest[0]); err != nil {
			die("%v", err)
		}

		if *forceDel {
			fmt.Fprintf(os.Stderr, "Deleted branch %s (forced)\n", rest[0])
		} else {
			fmt.Fprintf(os.Stderr, "Deleted branch %s\n", rest[0])
		}
		return
	}

	// rename current branch
	if *move {
		if len(rest) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: %s branch -m <newname>\n", app.Name)
			os.Exit(exitUsage)
		}
		if err := core.RenameCurrentBranch(rest[0]); err != nil {
			die("%v", err)
		}
		fmt.Fprintf(os.Stderr, "Renamed current branch to %s\n", rest[0])
		return
	}

	// create branch
	if len(rest) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s branch <branchname>\n", app.Name)
		os.Exit(exitUsage)
	}

	if err := core.CreateBranch(rest[0]); err != nil {
		die("%v", err)
	}
	fmt.Fprintf(os.Stderr, "Created branch %s\n", rest[0])
}
