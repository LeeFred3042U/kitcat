package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/core"
	"github.com/LeeFred3042U/kitcat/internal/remote"
)

func handlePull(args []string) {
	fs := flag.NewFlagSet("pull", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	rebase := fs.Bool("rebase", false, "Rebase onto upstream instead of merging")
	ffOnly := fs.Bool("ff-only", false, "Fail if not fast-forwardable")
	noFF := fs.Bool("no-ff", false, "Always create merge commit")
	addCredentialFlags(fs)

	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}
	if !core.IsRepoInitialized() {
		die("not a kitcat repository")
	}

	remoteName := "origin"
	if fs.NArg() >= 1 {
		remoteName = fs.Arg(0)
	}
	branch := ""
	if fs.NArg() >= 2 {
		branch = fs.Arg(1)
	}

	auth := resolveExplicitAuth(fs)
	res, err := remote.Pull(remote.PullOptions{
		RemoteName: remoteName,
		Branch:     branch,
		Auth:       auth,
		Rebase:     *rebase,
		FFOnly:     *ffOnly,
		NoFF:       *noFF,
	})
	if err != nil {
		die("pull: %v", err)
	}

	switch {
	case res.AlreadyUpToDate:
		fmt.Println("Already up to date.")
	case res.FastForwarded:
		fmt.Printf("Updating %s..%s\nFast-forward\n", shortHash(res.LocalSHA), shortHash(res.RemoteSHA))
	case res.Merged:
		fmt.Println("Merge made.")
	case res.Rebased:
		fmt.Println("Successfully rebased.")
	}
}
