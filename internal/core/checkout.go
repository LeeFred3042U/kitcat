package core

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Restore a file in the working directory to its state in the last commit
func CheckoutFile(filePath string) error {
	// Get the target content (from HEAD/Last Commit)
	lastCommit, err := storage.GetLastCommit()
	if err != nil {
		return err
	}

	tree, err := storage.ParseTree(lastCommit.TreeHash)
	if err != nil {
		return err
	}

	blobHash, ok := tree[filePath]
	if !ok {
		return errors.New("file not found in the last commit")
	}

	// SAFETY CHECK: Prevent overwriting dirty or untracked files
	if _, err := os.Stat(filePath); err == nil {
		// File exists, check if it is safe to overwrite
		currentHash, err := calculateHash(filePath)
		if err != nil {
			return fmt.Errorf("failed to calculate hash for safety check: %v", err)
		}

		// Load index to check if the file is tracked and clean
		index, err := storage.LoadIndex()
		if err != nil {
			return err
		}

		if trackedHash, ok := index[filePath]; ok {
			// File is tracked: fail if local changes exist (Index != Disk)
			if currentHash != trackedHash {
				return fmt.Errorf("error: local changes to '%s' would be overwritten", filePath)
			}
		} else {
			// File exists but is NOT in the index (untracked): fail to prevent data loss
			return fmt.Errorf("error: untracked file '%s' would be overwritten", filePath)
		}
	}

	// Safe to overwrite: Perform the checkout
	content, err := storage.ReadObject(blobHash)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		return err
	}

	// Update the index to reflect the checked-out version
	index, err := storage.LoadIndex()
	if err != nil {
		return err
	}
	index[filePath] = blobHash
	return storage.WriteIndex(index)
}

// Switch the current HEAD to the named branch and updates the working directory.
func CheckoutBranch(name string) error {
	branchPath := filepath.Join(headsDir, name)
	commitHashBytes, err := os.ReadFile(branchPath)
	if err != nil {
		return fmt.Errorf("branch '%s' not found", name)
	}
	commitHash := strings.TrimSpace(string(commitHashBytes))

	// Get the tree of the target commit
	// We need to find the commit object to get its tree hash
	commit, err := storage.FindCommit(commitHash)
	if err != nil {
		return err
	}
	targetTree, err := storage.ParseTree(commit.TreeHash)
	if err != nil {
		return err
	}

	isDirty, err := IsWorkDirDirty()
	if err != nil {
		return fmt.Errorf("could not check for local changes: %w", err)
	}
	if isDirty {
		return errors.New(
			"error: Your local changes to the following files would be overwritten by checkout:\n\tPlease commit your changes or stash them before you switch branches",
		)
	}

	// Before making changes, we should check if the user has unstaged work
	// that would be overwritten
	// So the real Git would abort here
	// For now, this is what i have done

	// SAFETY CHECK: prevent overwriting untracked files that conflict with files in the target branch.
	// This must run before we modify the working directory.
	currentIndex, err := storage.LoadIndex()
	if err != nil {
		return err
	}
	for filePath := range targetTree {
		if _, statErr := os.Stat(filePath); statErr == nil {
			if _, tracked := currentIndex[filePath]; !tracked {
				return fmt.Errorf("error: untracked file '%s' would be overwritten by checkout", filePath)
			}
		}
	}

	// Update the working directory to match the target tree
	// First, delete files that are not in the target tree
	// currentIndex loaded above for safety checks and deletion logic
	for path := range currentIndex {
		if _, existsInTarget := targetTree[path]; !existsInTarget {
			os.Remove(path)
		}
	}

	// Now, write/update files from the target tree
	for path, hash := range targetTree {
		content, err := storage.ReadObject(hash)
		if err != nil {
			return err
		}
		// Ensure directory exists before writing file
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return err
		}
	}

	// Update the index to match the new tree
	if err := storage.WriteIndex(targetTree); err != nil {
		return err
	}

	// Update HEAD to point to the new branch
	newHEADContent := fmt.Sprintf("ref: refs/heads/%s", name)
	return os.WriteFile(".kitcat/HEAD", []byte(newHEADContent), 0o644)
}

// CheckoutCommit moves HEAD to a specific commit and updates the working directory
// This puts the repository in a "detached HEAD" state
func CheckoutCommit(commitHash string) error {
	// Verify the commit actually exists
	_, err := storage.FindCommit(commitHash)
	if err != nil {
		return fmt.Errorf("commit '%s' not found", commitHash)
	}

	if err := UpdateWorkspaceAndIndex(commitHash); err != nil {
		return err
	}

	return os.WriteFile(".kitcat/HEAD", []byte(commitHash), 0o644)
}

func calculateHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
