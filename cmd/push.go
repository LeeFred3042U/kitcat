package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/core"
	"github.com/LeeFred3042U/kitcat/internal/remote"
)

func handlePush(args []string) {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	branch := fs.String("b", "", "Branch to push (default: current branch)")
	forceFlag := fs.Bool("force", false, "Force push (skip fast-forward check)")
	fs.Bool("f", false, "Shorthand for --force") // registered so -f is accepted
	username := fs.String("u", "", "HTTP username (or set KITCAT_USER env var)")
	password := fs.String("p", "", "HTTP password / token (or set KITCAT_TOKEN env var)")

	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	// Support -f as an alias for --force.
	fShort, _ := fs.Lookup("f"), false
	force := *forceFlag || (fShort != nil && fShort.Value.String() == "true")

	if !core.IsRepoInitialized() {
		die("not a kitcat repository")
	}

	// Positional argument: optional remote name (default "origin").
	remoteName := "origin"
	if fs.NArg() >= 1 {
		remoteName = fs.Arg(0)
	}

	// Allow credentials via env vars so they are not visible in process lists.
	user := *username
	if user == "" {
		user = os.Getenv("KITCAT_USER")
	}
	token := *password
	if token == "" {
		token = os.Getenv("KITCAT_TOKEN")
	}

	if user == "" || token == "" {
		fmt.Fprintf(os.Stderr,
			"hint: set -u / KITCAT_USER and -p / KITCAT_TOKEN for authenticated pushes\n")
	}

	var auth *remote.Auth
	if user != "" || token != "" {
		auth = &remote.Auth{Username: user, Password: token}
	}

	opts := remote.PushOptions{
		RemoteName: remoteName,
		Branch:     *branch,
		Auth:       auth,
		Force:      force,
	}

	if err := remote.Push(opts); err != nil {
		die("push: %v", err)
	}
}
