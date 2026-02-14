package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Commit creates a new commit object from the current index state.
// It resolves author identity from config and links the previous HEAD
// as parent when available.
func Commit(message string) (string, error) {
	name, _, _ := GetConfig("user.name")
	email, _, _ := GetConfig("user.email")

	// Fallback identity ensures commit creation never fails due to missing config.
	if name == "" {
		name = "Unknown"
	}
	if email == "" {
		email = "unknown@example.com"
	}
	authorStr := fmt.Sprintf("%s <%s>", name, email)

	// Tree is generated from index; index ordering must be deterministic upstream.
	treeHash, err := plumbing.WriteTree(storage.IndexPath)
	if err != nil {
		return "", fmt.Errorf("failed to write tree: %w", err)
	}

	// Attach previous commit as parent if HEAD exists.
	parents := []string{}
	headCommit, err := storage.GetLastCommit()
	if err == nil {
		parents = append(parents, headCommit.ID)
	}

	opts := plumbing.CommitOptions{
		Tree:      treeHash,
		Parents:   parents,
		Author:    authorStr,
		Committer: authorStr,
		Message:   message,
	}

	commitHash, err := plumbing.CommitTree(opts)
	if err != nil {
		return "", err
	}

	return commitHash, updateHead(commitHash)
}

// CommitAll stages tracked changes before creating a commit.
// Delegates staging behavior to AddAll.
func CommitAll(message string) (string, error) {
	if err := AddAll(); err != nil {
		return "", err
	}
	return Commit(message)
}

// AmendCommit replaces the current HEAD commit with a new commit object.
// Parent chain is preserved while tree/message may change.
func AmendCommit(message string) (string, error) {
	head, err := storage.GetLastCommit()
	if err != nil {
		return "", fmt.Errorf("nothing to amend")
	}

	// Empty message means reuse original commit message.
	if message == "" {
		message = head.Message
	}

	treeHash, err := plumbing.WriteTree(storage.IndexPath)
	if err != nil {
		return "", err
	}

	parents := []string{}
	if head.Parent != "" {
		parents = append(parents, head.Parent)
	}

	name, _, _ := GetConfig("user.name")
	email, _, _ := GetConfig("user.email")
	authorStr := fmt.Sprintf("%s <%s>", name, email)

	opts := plumbing.CommitOptions{
		Tree:      treeHash,
		Parents:   parents,
		Author:    authorStr,
		Committer: authorStr,
		Message:   message,
	}

	commitHash, err := plumbing.CommitTree(opts)
	if err != nil {
		return "", err
	}
	return commitHash, updateHead(commitHash)
}

// updateHead updates the branch reference pointed to by HEAD.
// If HEAD is detached or malformed, defaults to refs/heads/master.
func updateHead(commitHash string) error {
	headData, _ := os.ReadFile(".kitcat/HEAD")
	refPath := strings.TrimSpace(strings.TrimPrefix(string(headData), "ref: "))
	if refPath == "" {
		refPath = "refs/heads/master"
	}

	// Ensure branch directory exists before writing new ref value.
	if err := os.MkdirAll(filepath.Dir(".kitcat/"+refPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(".kitcat/"+refPath, []byte(commitHash), 0644)
}
