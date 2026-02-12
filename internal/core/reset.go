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

// Reset moves the current HEAD to the specified commit and updates index/worktree depending on mode.
func Reset(commitStr string, mode int) error {
	// 1. Resolve target commit
	commit, err := storage.FindCommit(commitStr)
	if err != nil {
		return fmt.Errorf("invalid commit %s: %w", commitStr, err)
	}

	// 2. Move HEAD (Soft/Mixed/Hard)
	// We use the helper to safely update HEAD or the Branch Ref
	if err := UpdateBranchPointer(commit.ID); err != nil {
		return err
	}

	// --soft: Move HEAD only. Index and Workdir left alone.
	if mode == ResetSoft {
		return nil
	}

	// 3. Update Index (Mixed & Hard)
	tree, err := storage.ParseTree(commit.TreeHash)
	if err != nil {
		return err
	}

	// Write the target tree into the index.
	// This makes the index match the commit we just reset to.
	if err := storage.WriteIndexFromTree(tree); err != nil {
		return err
	}

	// --mixed: Move HEAD and update Index. Workdir left alone.
	if mode == ResetMixed {
		return nil
	}

	// 4. Update Working Tree (Hard)
	// --hard: Force the working directory to match the commit.
	// This includes deleting files that are not in the commit and restoring files that are.
	// UpdateWorkspaceAndIndex handles the sync of Disk <-> Commit Tree.
	return UpdateWorkspaceAndIndex(commit.ID)
}
