package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/repo"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Stash saves the current working directory and index state to a temporary storage area.
// It creates a "WIP" commit containing the current index state and then performs a hard
// reset to HEAD, cleaning the workspace. This allows users to switch branches or pull
// updates without losing their work-in-progress.
// This is a convenience wrapper that calls StashPush with an empty message.
func Stash() error {
	return StashPush("")
}

// StashApply applies the stash at the given index (0 = newest) without removing it from the stack.
func StashApply(index int) error {
	if _, err := os.Stat(repo.Dir); os.IsNotExist(err) {
		return fmt.Errorf("fatal: not a %s repository (or any of the parent directories): %s", app.Name, repo.Dir)
	}

	stashes, err := storage.ListStashes()
	if err != nil {
		return fmt.Errorf("failed to list stashes: %w", err)
	}
	if index < 0 || index >= len(stashes) {
		return fmt.Errorf("invalid stash index: %d", index)
	}
	stashHash := stashes[index]

	// Check if working directory is clean to prevent data loss
	isDirty, err := IsWorkDirDirty()
	if err != nil {
		return fmt.Errorf("failed to check working directory status: %w", err)
	}
	if isDirty {
		return fmt.Errorf("error: your local changes would be overwritten by stash apply\nPlease commit your changes or stash them before you apply")
	}

	if err := UpdateWorkspaceAndIndex(stashHash); err != nil {
		return fmt.Errorf("failed to apply stash: %w", err)
	}

	fmt.Printf("Applied refs/stash@{%d} (%s)\n", index, stashHash[:7])
	return nil
}

// StashDrop removes the stash at the given index (0 = newest) from the stack.
func StashDrop(index int) error {
	if _, err := os.Stat(repo.Dir); os.IsNotExist(err) {
		return fmt.Errorf("fatal: not a %s repository (or any of the parent directories): %s", app.Name, repo.Dir)
	}

	stashes, err := storage.ListStashes()
	if err != nil {
		return fmt.Errorf("failed to list stashes: %w", err)
	}
	if index < 0 || index >= len(stashes) {
		return fmt.Errorf("invalid stash index: %d", index)
	}

	// Remove the stash at the given index
	newStashes := make([]string, 0, len(stashes)-1)
	for i, hash := range stashes {
		if i != index {
			newStashes = append(newStashes, hash)
		}
	}

	// Write the new stash list back to the file (preserve order: 0 = newest)
	path := filepath.Join(repo.RefsDir, "stash")
	if err := os.MkdirAll(repo.RefsDir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	for i := 0; i < len(newStashes); i++ {
		if _, err := fmt.Fprintln(f, newStashes[i]); err != nil {
			return err
		}
	}

	fmt.Printf("Dropped refs/stash@{%d} (%s)\n", index, stashes[index][:7])
	return nil
}

// StashPush saves the current working directory and index state to the stash stack.
// It creates a "WIP" commit with an optional custom message and performs a hard reset
// to HEAD, cleaning the workspace. The stash is pushed to the top of the stash stack.
func StashPush(message string) error {
	// Step 1: Validate repository is initialized
	if _, err := os.Stat(repo.Dir); os.IsNotExist(err) {
		return fmt.Errorf("fatal: current directory or any of the parent directories is not a %s repository.", app.Name)
	}

	// Step 2: Get current HEAD commit for parent reference and message
	headCommit, err := GetHeadCommit()
	if err != nil {
		if err == storage.ErrNoCommits || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("cannot stash: no commits yet")
		}
		return fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	// Step 3: Check if there are any changes to stash
	isDirty, err := IsWorkDirDirty()
	if err != nil {
		return fmt.Errorf("failed to check working directory status: %w", err)
	}
	if !isDirty {
		return fmt.Errorf("nothing to stash, working tree clean")
	}

	// Step 4: Get current branch name for WIP message
	branchName, err := GetHeadState()
	if err != nil {
		branchName = "detached HEAD"
	}

	// Step 5: Safely update index with current working directory state
	err = storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {
		for path, entry := range index {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				hashStr, err := storage.HashFile(path)
				if err != nil {
					return fmt.Errorf("failed to hash file %s: %w", path, err)
				}
				hashBytes, _ := storage.HexToHash(hashStr)
				entry.Hash = hashBytes
				index[path] = entry // Write back to the map
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to update index: %w", err)
	}

	// Step 6: Create tree from current index using plumbing
	treeHash, err := plumbing.WriteTree(repo.IndexPath)
	if err != nil {
		return fmt.Errorf("failed to create tree from index: %w", err)
	}

	// Step 7: Get author information
	authorName, _, _ := GetConfig("user.name")
	if authorName == "" {
		authorName = "Unknown"
	}
	authorEmail, _, _ := GetConfig("user.email")
	if authorEmail == "" {
		authorEmail = "unknown@example.com"
	}
	authorStr := fmt.Sprintf("%s <%s>", authorName, authorEmail)

	// Step 8: Create WIP commit message
	var wipMessage string
	if message != "" {
		wipMessage = fmt.Sprintf("WIP on %s: %s", branchName, message)
	} else {
		wipMessage = fmt.Sprintf("WIP on %s: %s", branchName, headCommit.Message)
	}

	// Step 9: Create the stash commit object in the database
	opts := plumbing.CommitOptions{
		Tree:      treeHash,
		Parents:   []string{headCommit.ID},
		Author:    authorStr,
		Committer: authorStr,
		Message:   wipMessage,
	}

	stashCommitHash, err := plumbing.CommitTree(opts)
	if err != nil {
		return fmt.Errorf("failed to save stash commit: %w", err)
	}

	// Step 10: Push the stash hash to the stack
	if err := storage.PushStash(stashCommitHash); err != nil {
		return fmt.Errorf("failed to push stash: %w", err)
	}

	// Step 11: Perform hard reset to HEAD to clean the workspace
	if err := Reset(headCommit.ID, ResetHard); err != nil {
		return fmt.Errorf("failed to reset workspace after stashing: %w", err)
	}

	fmt.Printf("Saved working directory and index state %s\n", wipMessage)
	return nil
}

// StashPop applies the most recent stash to the working directory and removes it.
func StashPop() error {
	if _, err := os.Stat(repo.Dir); os.IsNotExist(err) {
		return fmt.Errorf("fatal: current directory or any of the parent directories is not a %s repository.", app.Name)
	}

	stashHash, err := storage.PopStash()
	if err != nil {
		if err == storage.ErrNoStash {
			return fmt.Errorf("no stash entries found")
		}
		return fmt.Errorf("failed to pop stash: %w", err)
	}

	stashCommit, err := storage.FindCommit(stashHash)
	if err != nil {
		return fmt.Errorf("stash commit not found: %w", err)
	}

	isDirty, err := IsWorkDirDirty()
	if err != nil {
		return fmt.Errorf("failed to check working directory status: %w", err)
	}
	if isDirty {
		// If dirty, push the stash back so data isn't lost
		storage.PushStash(stashHash)
		return fmt.Errorf("error: your local changes would be overwritten by stash pop\nPlease commit your changes or stash them before you pop")
	}

	if err := UpdateWorkspaceAndIndex(stashHash); err != nil {
		storage.PushStash(stashHash) // Revert the pop on failure
		return fmt.Errorf("failed to apply stash: %w", err)
	}

	fmt.Printf("On branch %s\n", getCurrentBranchName())
	fmt.Printf("Dropped refs/stash@{0} (%s)\n", stashCommit.ID[:7])

	return nil
}

// getCurrentBranchName is a helper to get the current branch name
func getCurrentBranchName() string {
	headState, err := GetHeadState()
	if err != nil {
		return "Error: unable to fetch current branch"
	}
	return headState
}

// StashList lists all stashed states in reverse chronological order.
func StashList() error {
	if _, err := os.Stat(repo.Dir); os.IsNotExist(err) {
		return fmt.Errorf("fatal: not a %s repository (or any of the parent directories): %s", app.Name, repo.Dir)
	}

	stashes, err := storage.ListStashes()
	if err != nil {
		return fmt.Errorf("failed to list stashes: %w", err)
	}

	for i, hash := range stashes {
		commit, err := storage.FindCommit(hash)
		if err != nil {
			return fmt.Errorf("failed to find commit for stash %s: %w", hash, err)
		}
		fmt.Printf("stash@{%d}: %s\n", i, commit.Message)
	}

	return nil
}

// StashClear removes all stash entries from the stash stack.
func StashClear() error {
	if _, err := os.Stat(repo.Dir); os.IsNotExist(err) {
		return fmt.Errorf("fatal: not a %s repository (or any of the parent directories): %s", app.Name, repo.Dir)
	}

	if err := storage.ClearStash(); err != nil {
		return fmt.Errorf("failed to clear stash: %w", err)
	}

	return nil
}
