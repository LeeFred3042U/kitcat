package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/remote"
)

func addQuietFlag(fs *flag.FlagSet) *bool {
	q := fs.Bool("q", false, "suppress output")
	fs.BoolVar(q, "quiet", false, "suppress output")
	return q
}

func printIfNotQuiet(q bool, format string, a ...any) {
	if q {
		return
	}
	fmt.Printf(format, a...)
}

func shortHash(h string) string {
	if len(h) >= 7 {
		return h[:7]
	}
	return h
}

func parseStashIndex(arg string) int {
	var n int
	_, err := fmt.Sscanf(arg, "stash@{%d}", &n)
	if err != nil {
		die("invalid stash reference: %s", arg)
	}
	return n
}

func addCredentialFlags(fs *flag.FlagSet) {
	fs.String("u", "", "HTTP username (or set KITCAT_USER env var)")
	fs.String("p", "", "HTTP password / token (or set KITCAT_TOKEN env var)")
}

func resolveExplicitAuth(fs *flag.FlagSet) *remote.Auth {
	u := ""
	p := ""

	if f := fs.Lookup("u"); f != nil {
		u = f.Value.String()
	}
	if f := fs.Lookup("p"); f != nil {
		p = f.Value.String()
	}

	if u == "" {
		u = os.Getenv("KITCAT_USER")
	}
	if p == "" {
		p = os.Getenv("KITCAT_TOKEN")
	}

	if u == "" && p == "" {
		return nil
	}
	// Only treat as explicit credentials if both are present.
	// Partial credentials should fall back to other auth sources (keychain/prompt).
	if u == "" || p == "" {
		return nil
	}
	return &remote.Auth{Username: u, Password: p}
}
