package main

import (
	"path/filepath"
	"strings"
	"os/exec"
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/core"
	"github.com/LeeFred3042U/kitcat/internal/repo"
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

// Add this helper function to the bottom of the file
func captureMessageViaEditor() (string, error) {
	editor := os.Getenv("GIT_EDITOR")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi" // Fallback to vi, standard Unix behavior
	}

	msgFilePath := filepath.Join(repo.Dir, "COMMIT_EDITMSG")
	initialContent := "\n# Please enter the commit message for your changes. Lines starting\n# with '#' will be ignored, and an empty message aborts the commit.\n"
	if err := os.WriteFile(msgFilePath, []byte(initialContent), 0644); err != nil {
		return "", err
	}
	defer os.Remove(msgFilePath) // Clean up after

	cmd := exec.Command(editor, msgFilePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor aborted")
	}

	content, err := os.ReadFile(msgFilePath)
	if err != nil {
		return "", err
	}

	// Strip comments and trim whitespace
	var finalMsg []string
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "#") {
			finalMsg = append(finalMsg, line)
		}
	}

	cleanedMsg := strings.TrimSpace(strings.Join(finalMsg, "\n"))
	if cleanedMsg == "" {
		return "", fmt.Errorf("aborting commit due to empty commit message")
	}

	return cleanedMsg, nil
}
