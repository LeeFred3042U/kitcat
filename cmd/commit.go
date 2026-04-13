package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

func handleCommit(args []string) {
	var cleanArgs []string
	for _, a := range args {
		if strings.HasPrefix(a, "-am=") {
			cleanArgs = append(cleanArgs, "-a", "-m", strings.TrimPrefix(a, "-am="))
			continue
		}
		if strings.HasPrefix(a, "-am") && a != "-a" && a != "-m" {
			cleanArgs = append(cleanArgs, "-a", "-m")
			if len(a) > 3 {
				cleanArgs = append(cleanArgs, a[3:])
			}
			continue
		}
		cleanArgs = append(cleanArgs, a)
	}

	fs := flag.NewFlagSet("commit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	msg := fs.String("m", "", "Commit message")
	amend := fs.Bool("amend", false, "Amend the last commit")
	all := fs.Bool("a", false, "Stage all modified/deleted files")

	if err := fs.Parse(cleanArgs); err != nil {
		os.Exit(exitUsage)
	}

	if *amend && *msg == "" {
		if head, err := core.GetHeadCommit(); err == nil {
			*msg = head.Message
		}
	}

	var hash string
	var err error

	if *amend {
		hash, err = core.AmendCommit(*msg)
	} else if *all {
		hash, err = core.CommitAll(*msg)
	} else {
		hash, err = core.Commit(*msg)
	}

	if err != nil {
		if err.Error() == "nothing to commit, working tree clean" {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(exitFailure)
		}
		die("commit failed: %v", err)
	}

	if len(hash) >= 7 {
		fmt.Printf("[%s] %s\n", hash[:7], *msg)
	} else {
		fmt.Printf("[%s] %s\n", hash, *msg)
	}
}
