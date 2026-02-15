package core

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LeeFred3042U/kitcat/internal/diff"
	"github.com/LeeFred3042U/kitcat/internal/models"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// note task - Move commit storage from commits.log to individual objects in .kitcat/objects/
// hashCommit creates a unique, content-based SHA-1 hash for a Commit object.
func hashCommit(c models.Commit) string {
	h := sha1.New()
	h.Write([]byte(c.TreeHash))
	h.Write([]byte(c.Parent))
	h.Write([]byte(c.Message))
	h.Write([]byte(c.Timestamp.UTC().Format(time.RFC3339Nano)))
	return hex.EncodeToString(h.Sum(nil))
}

// Commit creates a new snapshot of the repository based on the current state of the index
// It prevents empty commits and returns the full commit object and a formatted summary
func Commit(message string) (models.Commit, string, error) {
	authorName, _, _ := GetConfig("user.name")
	authorEmail, _, _ := GetConfig("user.email")

	if authorName == "" || authorEmail == "" {
		return models.Commit{}, "", fmt.Errorf("author identity not configured. Please set user.name and user.email:\n  kitcat config user.name \"Your Name\"\n  kitcat config user.email \"you@example.com\"")
	}

	treeHash, err := storage.CreateTree()
	if err != nil {
		return models.Commit{}, "", err
	}

	var parentID, parentTreeHash string
	parentCommit, err := GetHeadCommit()
	// If error, we assume root commit (no parent) unless critical system error
	// In strict world, we'd check error type.
	if err == nil {
		parentID = parentCommit.ID
		parentTreeHash = parentCommit.TreeHash
	}

	if treeHash == parentTreeHash {
		return models.Commit{}, "", errors.New("nothing to commit, working tree clean")
	}

	commit := models.Commit{
		Parent:      parentID,
		Message:     message,
		Timestamp:   time.Now().UTC(),
		TreeHash:    treeHash,
		AuthorName:  authorName,
		AuthorEmail: authorEmail,
	}
	commit.ID = hashCommit(commit)

	if err := storage.AppendCommit(commit); err != nil {
		return models.Commit{}, "", err
	}

	refPath, err := getCurrentBranchRefPath()
	if err != nil {
		headData, readErr := os.ReadFile(".kitcat/HEAD")
		if readErr != nil {
			return models.Commit{}, "", fmt.Errorf("could not read HEAD: %w", readErr)
		}
		ref := strings.TrimSpace(string(headData))
		if !strings.HasPrefix(ref, "ref: ") {
			return models.Commit{}, "", fmt.Errorf("cannot commit in detached HEAD state")
		}
		refPath = strings.TrimPrefix(ref, "ref: ")
		if err := os.MkdirAll(filepath.Dir(filepath.Join(".kitcat", refPath)), 0o755); err != nil {
			return models.Commit{}, "", fmt.Errorf("could not create refs directory: %w", err)
		}
	}

	branchFilePath := filepath.Join(".kitcat", refPath)
	if err := SafeWrite(branchFilePath, []byte(commit.ID), 0o644); err != nil {
		return models.Commit{}, "", fmt.Errorf("failed to update branch pointer: %w", err)
	}

	parentTree := make(map[string]string)
	if parentID != "" {
		parentTree, _ = storage.ParseTree(parentCommit.TreeHash)
	}
	newTree, _ := storage.ParseTree(treeHash)
	summary, _ := GenerateCommitSummary(parentTree, newTree)

	return commit, summary, nil
}

// AmendCommit updates the message of the most recent commit without changing files.
// It loads the last commit, updates its message, re-hashes it, and updates the branch pointer.
func AmendCommit(newMessage string) (models.Commit, error) {
	// Get the last commit
	lastCommit, err := storage.GetLastCommit()
	if err != nil {
		if err == storage.ErrNoCommits {
			return models.Commit{}, errors.New("no commits to amend")
		}
		return models.Commit{}, fmt.Errorf("failed to get last commit: %w", err)
	}

	// Create a new commit with the updated message but same tree and parent
	amendedCommit := models.Commit{
		Parent:      lastCommit.Parent,
		Message:     newMessage,
		Timestamp:   lastCommit.Timestamp, // Keep original timestamp
		TreeHash:    lastCommit.TreeHash,  // Same files
		AuthorName:  lastCommit.AuthorName,
		AuthorEmail: lastCommit.AuthorEmail,
	}

	// Re-hash the commit (this generates a new ID)
	amendedCommit.ID = hashCommit(amendedCommit)

	// Save the amended commit
	if err := storage.AppendCommit(amendedCommit); err != nil {
		return models.Commit{}, fmt.Errorf("failed to save amended commit: %w", err)
	}

	// Update the branch pointer to the new commit ID
	refPath, err := getCurrentBranchRefPath()
	if err != nil {
		return models.Commit{}, fmt.Errorf("failed to get current branch: %w", err)
	}

	branchFilePath := filepath.Join(".kitcat", refPath)
	if err := SafeWrite(branchFilePath, []byte(amendedCommit.ID), 0o644); err != nil {
		return models.Commit{}, fmt.Errorf("failed to update branch pointer: %w", err)
	}

	return amendedCommit, nil
}

// CommitAll is a convenience function that implements the `commit -am` shortcut.
func CommitAll(message string) (models.Commit, string, error) {
	if err := AddAll(); err != nil {
		return models.Commit{}, "", fmt.Errorf("failed to stage changes before committing: %w", err)
	}
	return Commit(message)
}

func getCurrentBranchRefPath() (string, error) {
	headData, err := os.ReadFile(".kitcat/HEAD")
	if err != nil {
		return "", err
	}
	ref := strings.TrimSpace(string(headData))
	if !strings.HasPrefix(ref, "ref: ") {
		return "", fmt.Errorf("invalid HEAD format: %s", ref)
	}
	return strings.TrimPrefix(ref, "ref: "), nil
}

// pluralize is a simple helper for the summary string
func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// GenerateCommitSummary compares parent and new trees to create a formatted summary
// of files changed, lines inserted, and lines deleted
func GenerateCommitSummary(parentTree, newTree map[string]string) (string, error) {
	filesChanged, insertions, deletions := 0, 0, 0
	allPaths := make(map[string]bool)
	for path := range parentTree {
		allPaths[path] = true
	}
	for path := range newTree {
		allPaths[path] = true
	}

	for path := range allPaths {
		oldHash, inOld := parentTree[path]
		newHash, inNew := newTree[path]

		if inOld && !inNew {
			filesChanged++
			oldContent, _ := storage.ReadObject(oldHash)
			deletions += len(strings.Split(string(oldContent), "\n"))
		} else if !inOld && inNew {
			filesChanged++
			newContent, _ := storage.ReadObject(newHash)
			insertions += len(strings.Split(string(newContent), "\n"))
		} else if inOld && inNew && oldHash != newHash {
			filesChanged++
			oldContent, _ := storage.ReadObject(oldHash)
			newContent, _ := storage.ReadObject(newHash)
			d := diff.NewMyersDiff(strings.Split(string(oldContent), "\n"), strings.Split(string(newContent), "\n"))
			for _, chk := range d.Diffs() {
				if chk.Operation == diff.INSERT {
					insertions += len(chk.Text)
				}
				if chk.Operation == diff.DELETE {
					deletions += len(chk.Text)
				}
			}
		}
	}

	plural := "s"
	if filesChanged == 1 {
		plural = ""
	}
	return fmt.Sprintf("%d file%s changed, %d insertion%s(+), %d deletion%s(-)",
		filesChanged, plural,
		insertions, pluralize(insertions),
		deletions, pluralize(deletions)), nil
}
