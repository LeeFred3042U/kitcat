package core

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/LeeFred3042U/kitcat/internal/models"
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
	if !IsRepoInitialized() {
		return fmt.Errorf("fatal: not a kitcat repository (or any of the parent directories): .kitcat")
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
	if !IsRepoInitialized() {
		return fmt.Errorf("fatal: not a kitcat repository (or any of the parent directories): .kitcat")
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
	path := ".kitcat/refs/stash"
	if err := os.MkdirAll(".kitcat/refs", 0o755); err != nil {
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
// If message is empty, uses default format: "WIP on <branch>: <latest_commit_message>"
// If message is provided, uses format: "WIP on <branch>: <custom_message>"
func StashPush(message string) error {
	// Step 1: Validate repository is initialized
	if !IsRepoInitialized() {
		return fmt.Errorf("fatal: not a kitcat repository (or any of the parent directories): .kitcat")
	}

	// Step 2: Get current HEAD commit for parent reference and message
	// This must be done before checking dirty state because IsWorkDirDirty needs commits to exist
	headCommit, err := GetHeadCommit()
	if err != nil {
		// Check if the error is because there are no commits
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

	// Step 5: Update index with current working directory state for tracked files
	// This ensures unstaged changes are included in the stash tree
	index, err := storage.LoadIndex()
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	for path := range index {
		// If file exists in working directory, hash it and update index
		if _, err := os.Stat(path); err == nil {
			hash, err := storage.HashAndStoreFile(path)
			if err != nil {
				return fmt.Errorf("failed to hash file %s: %w", path, err)
			}
			index[path] = hash
		}
	}

	// Write updated index to disk so CreateTree uses the current state
	if err := storage.WriteIndex(index); err != nil {
		return fmt.Errorf("failed to write updated index: %w", err)
	}

	// Step 6: Create tree from current index
	treeHash, err := storage.CreateTree()
	if err != nil {
		return fmt.Errorf("failed to create tree from index: %w", err)
	}

	// Step 6: Get author information
	authorName, _, _ := GetConfig("user.name")
	if authorName == "" {
		authorName = "Unknown"
	}
	authorEmail, _, _ := GetConfig("user.email")
	if authorEmail == "" {
		authorEmail = "unknown@example.com"
	}

	// Step 7: Create WIP commit message
	// If custom message provided, use it; otherwise use default format
	var wipMessage string
	if message != "" {
		wipMessage = fmt.Sprintf("WIP on %s: %s", branchName, message)
	} else {
		wipMessage = fmt.Sprintf("WIP on %s: %s", branchName, headCommit.Message)
	}

	// Step 8: Create the stash commit
	stashCommit := models.Commit{
		Parent:      headCommit.ID,
		Message:     wipMessage,
		Timestamp:   time.Now().UTC(),
		TreeHash:    treeHash,
		AuthorName:  authorName,
		AuthorEmail: authorEmail,
	}
	stashCommit.ID = hashCommit(stashCommit)

	// Step 9: Save the stash commit to commits.log
	if err := storage.AppendCommit(stashCommit); err != nil {
		return fmt.Errorf("failed to save stash commit: %w", err)
	}

	// Step 10: Push the stash to the stack
	if err := storage.PushStash(stashCommit.ID); err != nil {
		return fmt.Errorf("failed to push stash: %w", err)
	}

	// Step 11: Perform hard reset to HEAD to clean the workspace
	if err := ResetHard(headCommit.ID); err != nil {
		// Note: We don't clean up the stash on failure as it's already in the stack
		// The user can manually pop it if needed
		return fmt.Errorf("failed to reset workspace after stashing: %w", err)
	}

	return nil
}

// StashPop applies the most recent stash to the working directory and removes it.
// It reads the stash commit, applies it to the workspace, and deletes the stash reference.
// This operation will fail if the working directory has uncommitted changes to prevent data loss.
func StashPop() error {
	// Step 1: Validate repository is initialized
	if !IsRepoInitialized() {
		return fmt.Errorf("fatal: not a kitcat repository (or any of the parent directories): .kitcat")
	}

	// Step 2: Pop the most recent stash from the stack
	stashHash, err := storage.PopStash()
	if err != nil {
		if err == storage.ErrNoStash {
			return fmt.Errorf("no stash entries found")
		}
		return fmt.Errorf("failed to pop stash: %w", err)
	}

	// Step 3: Verify the stash commit exists
	stashCommit, err := storage.FindCommit(stashHash)
	if err != nil {
		return fmt.Errorf("stash commit not found: %w", err)
	}

	// Step 4: Check if working directory is clean to prevent data loss
	isDirty, err := IsWorkDirDirty()
	if err != nil {
		return fmt.Errorf("failed to check working directory status: %w", err)
	}
	if isDirty {
		return fmt.Errorf("error: your local changes would be overwritten by stash pop\nPlease commit your changes or stash them before you pop")
	}

	// Step 5: Apply the stash commit to the working directory
	if err := UpdateWorkspaceAndIndex(stashHash); err != nil {
		return fmt.Errorf("failed to apply stash: %w", err)
	}

	// Step 6: Print success message with commit info
	fmt.Printf("On branch %s\n", getCurrentBranchName())
	fmt.Printf("Dropped refs/stash@{0} (%s)\n", stashCommit.ID[:7])

	return nil
}

// getCurrentBranchName is a helper to get the current branch name
func getCurrentBranchName() string {
	headState, err := GetHeadState()
	if err != nil {
		return "unknown"
	}
	return headState
}

// StashList lists all stashed states in reverse chronological order.
func StashList() error {
	if !IsRepoInitialized() {
		return fmt.Errorf("fatal: not a kitcat repository (or any of the parent directories): .kitcat")
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
// It truncates the stash file to size 0, effectively clearing all stashed changes.
func StashClear() error {
	if !IsRepoInitialized() {
		return fmt.Errorf("fatal: not a kitcat repository (or any of the parent directories): .kitcat")
	}

	if err := storage.ClearStash(); err != nil {
		return fmt.Errorf("failed to clear stash: %w", err)
	}

	return nil
}
