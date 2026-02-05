package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

const headsDir string = ".kitcat/refs/heads"

// IsValidRefName checks if the branch or tag name is safe and valid
func IsValidRefName(name string) bool {
	if !IsSafePath(name) {
		return false
	}
	if strings.ContainsAny(name, "/\\") {
		return false
	}
	// Additional branch-specific checks (e.g., no spaces)
	if strings.Contains(name, " ") {
		return false
	}
	return true
}

// Resolves the current commit hash by following the HEAD reference
func readHEAD() (string, error) {
	headData, err := os.ReadFile(".kitcat/HEAD")
	if err != nil {
		return "", err
	}
	ref := strings.TrimSpace(string(headData))
	if !strings.HasPrefix(ref, "ref: ") {
		return "", fmt.Errorf("invalid HEAD format")
	}
	return strings.TrimPrefix(ref, "ref: "), nil
}

// readCommitHash reads the commit hash from the reference path
func readCommitHash(referencePath string) (string, error) {
	commitHash, err := os.ReadFile(filepath.Join(".kitcat", referencePath))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(commitHash)), nil
}

// Create a new branch pointing to the current HEAD commit
func CreateBranch(name string) error {
	if !IsValidRefName(name) {
		return fmt.Errorf("invalid branch name '%s'", name)
	}
	if IsBranch(name) {
		return fmt.Errorf("branch '%s' already exists", name)
	}
	head, err := readHEAD()
	if err != nil {
		return err
	}
	commitHash, err := readCommitHash(head)
	if err != nil {
		// If HEAD can't be read, maybe there are no commits yet
		lastCommit, err := storage.GetLastCommit()
		if err != nil {
			return errors.New("cannot create branch: no commits yet")
		}
		commitHash = lastCommit.ID
	}

	if err := os.MkdirAll(headsDir, 0o755); err != nil {
		return err
	}

	branchPath := filepath.Join(headsDir, name)
	return os.WriteFile(branchPath, []byte(strings.TrimSpace(commitHash)), 0o644)
}

// Checks if a branch with the given name exists.
func IsBranch(name string) bool {
	branchPath := filepath.Join(headsDir, name)
	if _, err := os.Stat(branchPath); err == nil {
		return true
	}
	return false
}

// ListBranches lists all local branches and highlights the current one
func ListBranches() error {
	currentBranch, err := GetHeadState()
	if err != nil {
		// It's possible to be in a detached HEAD state.
		if strings.Contains(err.Error(), "invalid HEAD format") {
			// In a real git, it would show the hash, while we just note it
			currentBranch = "HEAD (detached)"
		} else {
			return err
		}
	}

	// Read all files in the refs/heads directory
	// Each file is a branch
	branches, err := os.ReadDir(headsDir)
	if err != nil {
		return err
	}

	for _, b := range branches {
		if b.Name() == currentBranch {
			// Print the current branch with a '*' and in color.
			fmt.Printf("* %s%s%s\n", colorGreen, b.Name(), colorReset)
		} else {
			fmt.Printf("  %s\n", b.Name())
		}
	}

	return nil
}

func RenameCurrentBranch(newName string) error {
	if !IsValidRefName(newName) {
		return fmt.Errorf("invalid branch name '%s'", newName)
	}

	headPath := ".kitcat/HEAD"
	headContent, err := os.ReadFile(headPath)
	if err != nil {
		return err
	}

	headStr := strings.TrimSpace(string(headContent))
	const refPrefix = "ref: refs/heads/"
	if !strings.HasPrefix(headStr, refPrefix) {
		return errors.New("HEAD is not pointing to a branch")
	}

	oldName := strings.TrimPrefix(headStr, refPrefix)
	oldRef := filepath.Join(".kitcat", "refs", "heads", oldName)
	newRef := filepath.Join(".kitcat", "refs", "heads", newName)

	if _, err := os.Stat(newRef); err == nil {
		return fmt.Errorf("branch '%s' already exists", newName)
	}

	commitHash, err := os.ReadFile(oldRef)
	if err != nil {
		return err
	}

	if err := os.WriteFile(newRef, commitHash, 0o644); err != nil {
		return err
	}

	newHeadContent := []byte(refPrefix + newName + "\n")
	if err := os.WriteFile(headPath, newHeadContent, 0o644); err != nil {
		return err
	}

	if err := os.Remove(oldRef); err != nil {
		return err
	}

	return nil
}

// DeleteBranch deletes the branch
// throws error if the branch is equal to HEAD
func DeleteBranch(name string) error {
	head, err := readHEAD()
	if err != nil {
		return err
	}

	// Checks if the branch is set to HEAD
	if head == "refs/heads/"+name {
		return fmt.Errorf(
			"branch `%s` is currently active, switch to another branch and then try to delete again",
			name,
		)
	}

	if err := os.Remove(filepath.Join(headsDir, name)); err != nil {
		return fmt.Errorf("branch `%s` doesn't exist", name)
	}

	return nil
}
