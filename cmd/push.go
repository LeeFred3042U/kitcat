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
	setUpstream := fs.Bool("set-upstream", false, "Set upstream (write branch tracking config)")
	addCredentialFlags(fs)

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

	auth := resolveExplicitAuth(fs)
	if auth == nil || auth.Username == "" || auth.Password == "" {
		fmt.Fprintf(os.Stderr,
			"hint: set -u / KITCAT_USER and -p / KITCAT_TOKEN for authenticated pushes\n")
	}

	opts := remote.PushOptions{
		RemoteName: remoteName,
		Branch:     *branch,
		Auth:       auth,
		Force:      force,
		SetUpstream: *setUpstream,
	}

	if err := remote.Push(opts); err != nil {
		die("push: %v", err)
	}
}
