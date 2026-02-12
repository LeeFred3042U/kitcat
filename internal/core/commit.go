package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

func Commit(message string) (string, error) {
	name, _, _ := GetConfig("user.name")
	email, _, _ := GetConfig("user.email")
	if name == "" {
		name = "Unknown"
	}
	if email == "" {
		email = "unknown@example.com"
	}
	authorStr := fmt.Sprintf("%s <%s>", name, email)

	treeHash, err := plumbing.WriteTree(storage.IndexPath)
	if err != nil {
		return "", fmt.Errorf("failed to write tree: %w", err)
	}

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

func CommitAll(message string) (string, error) {
	if err := AddAll(); err != nil {
		return "", err
	}
	return Commit(message)
}

func AmendCommit(message string) (string, error) {
	head, err := storage.GetLastCommit()
	if err != nil {
		return "", fmt.Errorf("nothing to amend")
	}

	// Use empty message as signal to keep original
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

func updateHead(commitHash string) error {
	headData, _ := os.ReadFile(".kitcat/HEAD")
	refPath := strings.TrimSpace(strings.TrimPrefix(string(headData), "ref: "))
	if refPath == "" {
		refPath = "refs/heads/master"
	}

	if err := os.MkdirAll(filepath.Dir(".kitcat/"+refPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(".kitcat/"+refPath, []byte(commitHash), 0644)
}
