package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Merge attempts to fast-forward the current branch to the specified branch.
// Only fast-forward merges are supported; no merge commits are created.
func Merge(branchToMerge string) error {

	// Ensure we are inside a repository before performing destructive operations.
	if _, err := os.Stat(RepoDir); os.IsNotExist(err) {
		return errors.New("not a kitkat repository (run `kitkat init`)")
	}

	// Abort if working directory has local modifications that would be overwritten.
	dirty, err := IsWorkDirDirty()
	if err != nil {
		return fmt.Errorf("failed to check working directory status: %w", err)
	}
	if dirty {
		return fmt.Errorf("error: your local changes would be overwritten by merge. Please commit or stash them")
	}

	// Resolve target branch head.
	branchPath := filepath.Join(HeadsDir, branchToMerge)
	featureHeadHashBytes, err := os.ReadFile(branchPath)
	if err != nil {
		return fmt.Errorf("branch '%s' not found", branchToMerge)
	}
	featureHeadHash := strings.TrimSpace(string(featureHeadHashBytes))

	// Read current HEAD commit hash.
	currentHeadHash, err := readHead()
	if err != nil {
		return fmt.Errorf("could not read current HEAD: %w", err)
	}

	// Determine ancestry relationship to decide merge type.
	mergeBase, err := storage.FindMergeBase(currentHeadHash, featureHeadHash)
	if err != nil {
		return fmt.Errorf("failed to calculate merge base: %w", err)
	}

	switch mergeBase {
	case currentHeadHash:
		// Fast-forward: current branch is behind target branch.
		fmt.Printf("Updating %s..%s\n", currentHeadHash[:7], featureHeadHash[:7])
		fmt.Println("Fast-forward")

	case featureHeadHash:
		// No-op: target already contained in current history.
		fmt.Println("Already up to date.")
		return nil

	default:
		// Diverged histories are not supported in this implementation.
		return fmt.Errorf(
			"fatal: Not possible to fast-forward, aborting.\n"+
				"Merge commits are not supported. Please rebase '%s' onto the current branch",
			branchToMerge,
		)
	}

	// Move branch pointer forward to target commit.
	if err := UpdateBranchPointer(featureHeadHash); err != nil {
		return fmt.Errorf("failed to update branch pointer: %w", err)
	}

	// Synchronize index and working directory with new HEAD state.
	err = UpdateWorkspaceAndIndex(featureHeadHash)
	if err != nil {
		// Attempt rollback to avoid leaving branch ref advanced while workspace is stale.
		fmt.Printf("UpdateWorkspaceAndIndex failed: %v. Rolling back branch pointer...\n", err)
		if rollbackErr := UpdateBranchPointer(currentHeadHash); rollbackErr != nil {
			return fmt.Errorf("failed to update workspace: %w; additionally failed to rollback branch pointer: %v", err, rollbackErr)
		}
		return fmt.Errorf("failed to update workspace: %w; branch pointer rolled back to %s", err, currentHeadHash)
	}

	return nil
}
