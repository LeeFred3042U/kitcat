package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/LeeFred3042U/kitcat/internal/remote"
	"golang.org/x/term"
)

func handleCredential(args []string) {
	fs := flag.NewFlagSet("credential", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: kitcat credential <set|clear|list> [hostname]\n")
		os.Exit(exitUsage)
	}

	sub := fs.Arg(0)
	switch sub {
	case "set":
		if fs.NArg() < 2 {
			die("credential set: missing hostname")
		}
		hostname := fs.Arg(1)
		auth, err := promptCredential(hostname)
		if err != nil {
			die("credential set: %v", err)
		}
		if err := remote.SaveToKeychain(hostname, auth); err != nil {
			die("credential set: %v", err)
		}
		fmt.Printf("Saved credentials for %s\n", hostname)

	case "clear":
		if fs.NArg() < 2 {
			die("credential clear: missing hostname")
		}
		hostname := fs.Arg(1)
		if err := remote.DeleteFromKeychain(hostname); err != nil {
			die("credential clear: %v", err)
		}
		fmt.Printf("Cleared credentials for %s\n", hostname)

	case "list":
		hosts, err := remote.ListCredentialHosts()
		if err != nil {
			die("credential list: %v", err)
		}
		for _, h := range hosts {
			fmt.Println(h)
		}

	default:
		die("credential: unknown subcommand %q", sub)
	}
}

func promptCredential(hostname string) (*remote.Auth, error) {
	in := bufio.NewReader(os.Stdin)

	fmt.Printf("Username for '%s': ", hostname)
	user, err := in.ReadString('\n')
	if err != nil {
		return nil, err
	}
	user = strings.TrimSpace(user)
	if user == "" {
		return nil, errors.New("empty username")
	}

	fmt.Printf("Token for '%s': ", hostname)
	pw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(string(pw))
	if token == "" {
		return nil, errors.New("empty token")
	}
	return &remote.Auth{Username: user, Password: token, Source: remote.AuthSourceExplicit}, nil
}
