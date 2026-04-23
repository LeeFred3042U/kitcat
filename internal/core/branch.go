package core

import (
	"path/filepath"
	"strings"
	"errors"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// headsDir is the directory containing local branch reference files.
// Each file stores the commit hash that the branch currently points to.
const headsDir string = ".kitcat/refs/heads"

// ResolveHead resolves the commit currently referenced by HEAD.
//
// HEAD may point directly to a commit or to a symbolic reference such as
// "refs/heads/main". Resolution is delegated to storage.GetLastCommit,
// which handles symbolic references and detached states.
func ResolveHead() (string, error) {
	c, err := storage.GetLastCommit()
	if err != nil {
		return "", err
	}
	return c.ID, nil
}

// IsValidRefName validates a branch or tag name.
//
// The validation prevents unsafe filesystem paths or malformed names
// from being used as references.
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

// readHEAD reads the symbolic reference stored in the HEAD file.
//
// The function expects HEAD to contain a symbolic reference such as:
//
//	ref: refs/heads/main
//
// Detached HEAD states are treated as invalid for callers expecting
// a branch reference.
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

// readCommitHash reads the commit hash stored in a reference file.
func readCommitHash(referencePath string) (string, error) {
	commitHash, err := os.ReadFile(filepath.Join(".kitcat", referencePath))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(commitHash)), nil
}

// CreateBranch creates a new branch pointing to the current HEAD commit.
//
// If the symbolic HEAD reference cannot be resolved, the function falls
// back to the most recent commit retrieved via storage.GetLastCommit.
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
		// Detached or missing reference: fall back to last commit.
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

// IsBranch reports whether a branch reference file exists.
func IsBranch(name string) bool {
	branchPath := filepath.Join(headsDir, name)
	if _, err := os.Stat(branchPath); err == nil {
		return true
	}
	return false
}

func ListBranches(verbose bool) error {
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
		name := b.Name()

		isCurrent := name == currentBranch

		// Non-verbose
		if !verbose {
			if isCurrent {
				fmt.Printf("* %s%s%s\n", colorGreen, name, colorReset)
			} else {
				fmt.Printf("  %s\n", name)
			}
			continue
		}

		// verbose mode
		hash, err := storage.GetRef(filepath.Join("refs/heads", name))
		if err != nil {
			return err
		}

		commit, err := storage.FindCommit(hash)
		if err != nil {
			return err
		}

		short := hash
		if len(short) > 7 {
			short = short[:7]
		}

		if isCurrent {
			fmt.Printf("* %s%s%s %s %s\n", colorGreen, name, colorReset, short, commit.Message)
		} else {
			fmt.Printf("  %s %s %s\n", name, short, commit.Message)
		}
	}

	return nil
}

// RenameCurrentBranch renames the currently checked-out branch.
//
// The operation is performed by copying the reference to the new name,
// updating HEAD, and then removing the old reference.
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

	// Create the new reference before modifying HEAD.
	if err := SafeWrite(newRef, commitHash, 0o644); err != nil {
		return err
	}

	newHeadContent := []byte(refPrefix + newName + "\n")
	if err := SafeWrite(headPath, newHeadContent, 0o644); err != nil {
		return err
	}

	// Record rename events in reflogs.
	hashStr := strings.TrimSpace(string(commitHash))
	ReflogAppend("refs/heads/"+newName, "", hashStr, "branch: renamed from "+oldName)
	ReflogAppend("HEAD", hashStr, hashStr, "checkout: moving from "+oldName+" to "+newName)

	return os.Remove(oldRef)
}

// DeleteBranch removes the reference file for the specified branch.
//
// The operation is rejected if the branch is currently checked out.
func DeleteBranch(name string) error {
	head, err := readHEAD()
	if err != nil {
		return err
	}

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
