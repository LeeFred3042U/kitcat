package core

import (
	"fmt"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

const (
	ResetMixed = 0
	ResetSoft  = 1
	ResetHard  = 2
)

// Reset moves HEAD to the specified commit and conditionally updates
// index and working directory depending on reset mode semantics.
func Reset(commitStr string, mode int) error {
	// Resolve target commit first to ensure HEAD is not moved to an invalid state.
	commit, err := storage.FindCommit(commitStr)
	if err != nil {
		return fmt.Errorf("invalid commit %s: %w", commitStr, err)
	}

	// Update HEAD or branch ref before mutating index/worktree to match Git behavior.
	if err := UpdateBranchPointer(commit.ID); err != nil {
		return err
	}

	// --soft: only move HEAD; index and working tree remain unchanged.
	if mode == ResetSoft {
		return nil
	}

	// Load tree snapshot from target commit; used to rebuild index state.
	tree, err := storage.ParseTree(commit.TreeHash)
	if err != nil {
		return err
	}

	// Replacing the index aligns staged state with the target commit.
	if err := storage.WriteIndexFromTree(tree); err != nil {
		return err
	}

	// --mixed: HEAD + index updated; working directory left untouched.
	if mode == ResetMixed {
		return nil
	}

	// --hard: destructive sync of working directory with commit tree.
	// Files absent from the commit may be deleted.
	return UpdateWorkspaceAndIndex(commit.ID)
}
