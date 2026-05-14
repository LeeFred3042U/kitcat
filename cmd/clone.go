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
	username := fs.String("u", "", "HTTP username (or set KITCAT_USER env var)")
	password := fs.String("p", "", "HTTP password / token (or set KITCAT_TOKEN env var)")

	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: kitcat clone <url> [--dir <name>] [-b <branch>] [-u <user>] [-p <token>]\n")
		os.Exit(exitUsage)
	}

	remoteURL := fs.Arg(0)

	// Allow credentials via env vars so they are not visible in process lists
	user := *username
	if user == "" {
		user = os.Getenv("KITCAT_USER")
	}
	token := *password
	if token == "" {
		token = os.Getenv("KITCAT_TOKEN")
	}

	var auth *remote.Auth
	if user != "" || token != "" {
		auth = &remote.Auth{Username: user, Password: token}
	}

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
