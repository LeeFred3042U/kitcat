package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/core"
	"github.com/LeeFred3042U/kitcat/internal/remote"
)

func handleFetch(args []string) {
	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	prune := fs.Bool("prune", false, "Delete remote-tracking refs that no longer exist on remote")
	fs.BoolVar(prune, "p", false, "Delete remote-tracking refs that no longer exist on remote")
	all := fs.Bool("all", false, "Fetch all configured remotes (not yet implemented)")
	tags := fs.Bool("tags", false, "Fetch all tags (not yet implemented)")
	noTags := fs.Bool("no-tags", false, "Suppress tag auto-following (not yet implemented)")
	depth := fs.Int("depth", 0, "Deepen a shallow fetch (not yet implemented)")

	addCredentialFlags(fs)

	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}
	if !core.IsRepoInitialized() {
		die("not a kitcat repository")
	}

	if *all || *tags || *noTags || *depth != 0 {
		die("fetch: --all/--tags/--no-tags/--depth are not yet implemented")
	}

	remoteName := "origin"
	if fs.NArg() >= 1 {
		remoteName = fs.Arg(0)
	}

	auth := resolveExplicitAuth(fs)
	res, err := remote.Fetch(remote.FetchOptions{
		RemoteName: remoteName,
		Auth:       auth,
		Prune:      *prune,
	})
	if err != nil {
		die("fetch: %v", err)
	}

	for _, u := range res.Updated {
		if u.OldSHA == "" {
			fmt.Printf(" * [new ref] %s -> %s\n", u.RemoteRef, u.NewSHA[:8])
		} else {
			fmt.Printf("   %s %s..%s\n", u.RemoteRef, shortHash(u.OldSHA), shortHash(u.NewSHA))
		}
	}
	for _, p := range res.Pruned {
		fmt.Printf(" - [deleted] %s\n", p)
	}
}

