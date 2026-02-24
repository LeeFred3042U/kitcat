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

// ResolveHead returns the commit hash currently referenced by HEAD.
// Delegates resolution to storage.GetLastCommit which handles symbolic refs.
func ResolveHead() (string, error) {
	c, err := storage.GetLastCommit()
	if err != nil {
		return "", err
	}
	return c.ID, nil
}

// IsValidRefName validates a branch/tag name to avoid unsafe filesystem paths.
func IsValidRefName(name string) bool {
	if !IsSafePath(name) {
		return false
	}
	if strings.ContainsAny(name, "/\\") {
		return false
	}
	if strings.Contains(name, " ") {
		return false
	}
	return true
}

// readHEAD reads the symbolic reference stored in HEAD.
// Detached HEADs are treated as invalid for callers that expect branch refs.
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

// readCommitHash reads a commit hash from a reference file path.
func readCommitHash(referencePath string) (string, error) {
	commitHash, err := os.ReadFile(filepath.Join(".kitcat", referencePath))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(commitHash)), nil
}

// CreateBranch creates a new branch pointing to the current HEAD commit.
// Falls back to last commit lookup if HEAD ref cannot be resolved.
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
		// Detached or missing ref: fallback to latest commit.
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
	err = SafeWrite(branchPath, []byte(strings.TrimSpace(commitHash)), 0o644)
	if err == nil {
		ReflogAppend("refs/heads/"+name, "", commitHash, "branch: Created from HEAD")
	}
	return err
}

// IsBranch returns true if a branch reference file exists.
func IsBranch(name string) bool {
	branchPath := filepath.Join(headsDir, name)
	if _, err := os.Stat(branchPath); err == nil {
		return true
	}
	return false
}

// ListBranches prints all branches, highlighting the current one.
// Output ordering depends on filesystem enumeration.
func ListBranches() error {
	currentBranch, err := GetHeadState()
	if err != nil {
		if strings.Contains(err.Error(), "invalid HEAD format") {
			currentBranch = "HEAD (detached)"
		} else {
			return err
		}
	}

	branches, err := os.ReadDir(headsDir)
	if err != nil {
		return err
	}

	for _, b := range branches {
		if b.Name() == currentBranch {
			fmt.Printf("* %s%s%s\n", colorGreen, b.Name(), colorReset)
		} else {
			fmt.Printf("  %s\n", b.Name())
		}
	}

	return nil
}

// RenameCurrentBranch renames the active branch by copying its ref,
// updating HEAD, and deleting the old reference.
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

	// Create new ref atomically before modifying HEAD
	if err := SafeWrite(newRef, commitHash, 0o644); err != nil {
		return err
	}

	newHeadContent := []byte(refPrefix + newName + "\n")
	if err := SafeWrite(headPath, newHeadContent, 0o644); err != nil {
		return err
	}

	// Reflogs for rename
	hashStr := strings.TrimSpace(string(commitHash))
	ReflogAppend("refs/heads/"+newName, "", hashStr, "branch: renamed from "+oldName)
	ReflogAppend("HEAD", hashStr, hashStr, "checkout: moving from "+oldName+" to "+newName)

	return os.Remove(oldRef)
}

// DeleteBranch removes a branch reference file.
// Refuses deletion if the branch is currently checked out.
func DeleteBranch(name string) error {
	head, err := readHEAD()
	if err != nil {
		return err
	}

	if head == "refs/heads/"+name {
		return fmt.Errorf("branch `%s` is currently active, switch to another branch and then try to delete again", name)
	}

	if err := os.Remove(filepath.Join(headsDir, name)); err != nil {
		return fmt.Errorf("branch `%s` doesn't exist", name)
	}

	return nil
}
