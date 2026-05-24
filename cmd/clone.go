package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/remote"
)

func handleClone(args []string) {
	fs := flag.NewFlagSet("clone", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	dir := fs.String("dir", "", "Local directory name (default: inferred from URL)")
	branch := fs.String("b", "", "Branch to check out (default: remote HEAD)")
	addCredentialFlags(fs)

	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: kitcat clone <url> [--dir <name>] [-b <branch>] [-u <user>] [-p <token>]\n")
		os.Exit(exitUsage)
	}

	remoteURL := fs.Arg(0)

	auth := resolveExplicitAuth(fs)

	opts := remote.CloneOptions{
		RemoteURL: remoteURL,
		Dir:       *dir,
		Branch:    *branch,
		Auth:      auth,
	}

	if err := remote.Clone(opts); err != nil {
		die("clone: %v", err)
	}
}
