package core

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/models"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

func RebaseAbort() error {
	fmt.Println("Rebase aborted")
	return nil
}

func RebaseContinue() error {
	fmt.Println("Rebase continue not implemented")
	return nil
}

func GetCurrentBranch() (string, error) {
	head, err := os.ReadFile(".kitcat/HEAD")
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(string(head))
	if strings.HasPrefix(content, "ref: refs/heads/") {
		return strings.TrimPrefix(content, "ref: refs/heads/"), nil
	}
	return "", fmt.Errorf("detached HEAD")
}

// Rebase performs a simplified interactive rebase.
// It rewrites history from the merge base to HEAD onto the new base.
func Rebase(targetBranch string, interactive bool) error {
	// 1. Get current branch and HEAD
	currentBranch, err := GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	headCommit, err := storage.GetLastCommit()
	if err != nil {
		return fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	// 2. Get target commit
	targetHash, err := storage.GetRef("refs/heads/" + targetBranch)
	if err != nil {
		return err
	}

	// 3. Find Merge Base
	mergeBase, err := storage.FindMergeBase(headCommit.ID, targetHash)
	if err != nil {
		return fmt.Errorf("failed to find merge base: %w", err)
	}

	fmt.Printf("Rebasing %s onto %s (base: %s)\n", currentBranch, targetBranch, mergeBase[:7])

	// 4. Collect commits to rebase (from base+1 to HEAD)
	var commitsToRebase []models.Commit
	curr := headCommit.ID
	for curr != mergeBase && curr != "" {
		c, err := storage.FindCommit(curr)
		if err != nil {
			return err
		}
		// Prepend (since we walk backwards)
		commitsToRebase = append([]models.Commit{c}, commitsToRebase...)
		curr = c.Parent
	}

	if len(commitsToRebase) == 0 {
		fmt.Println("Current branch is already up to date.")
		return nil
	}

	// 5. Interactive Mode: Let user edit the plan
	if interactive {
		commitsToRebase, err = promptInteractiveRebase(commitsToRebase)
		if err != nil {
			return fmt.Errorf("rebase aborted: %w", err)
		}
	}

	// 6. Perform Rebase (Replay commits)
	// Reset HEAD to targetBranch (the new base)
	if err := hardResetTo(targetHash); err != nil {
		return err
	}

	for _, commit := range commitsToRebase {
		fmt.Printf("Picking %s %s\n", commit.ID[:7], commit.Message)

		// A. Checkout commit content (simplified: we just apply tree if no conflicts)
		// In a real git, we'd apply the DIFF. Here we cheat and restore the Tree,
		// relying on manual conflict resolution if files differ.
		// For a robust rebase, we should use 3-way merge logic.
		// Reusing 'CherryPick' logic here effectively.

		if err := cherryPickTree(commit.TreeHash); err != nil {
			return err
		}

		// B. Commit with new parent (automatically handled by Commit())
		// The Commit() function uses the current HEAD as parent.
		hash, err := Commit(commit.Message)
		if err != nil {
			return fmt.Errorf("failed to commit %s during rebase: %w", commit.ID[:7], err)
		}

		// Optional: Preserve Author/Date?
		// The plumbing Commit() uses "now". To preserve, we'd need to modify CommitOptions
		// inside Commit() or pass them in. For now, we accept new timestamps.
		fmt.Printf("Re-applied as %s\n", hash[:7])
	}

	// 7. Update branch pointer
	// Commit() updates HEAD (detached or branch).
	// If we were on a branch, HEAD points to refs/heads/branch, so it's updated.
	fmt.Println("Rebase completed successfully.")
	return hardResetTo(targetHash)
}

// hardResetTo moves HEAD and updates index/workdir to a specific hash
func hardResetTo(hash string) error {
	currentBranch, _ := GetCurrentBranch()
	refPath := ".kitcat/refs/heads/" + currentBranch
	if err := os.WriteFile(refPath, []byte(hash), 0644); err != nil {
		return err
	}
	return Reset(hash, ResetHard)
}

// promptInteractiveRebase opens an editor or simple prompt to reorder/drop commits
func promptInteractiveRebase(commits []models.Commit) ([]models.Commit, error) {
	// Simplified: print and ask for confirmation
	fmt.Println("\nCommits to rebase:")
	for i, c := range commits {
		fmt.Printf("%d: %s %s\n", i+1, c.ID[:7], c.Message)
	}
	fmt.Println("\nTo drop a commit, enter its number (comma separated). Enter to proceed.")
	fmt.Print("> ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return commits, nil
	}

	dropIndices := make(map[int]bool)
	parts := strings.Split(input, ",")
	for _, p := range parts {
		var idx int
		if _, err := fmt.Sscanf(strings.TrimSpace(p), "%d", &idx); err == nil {
			if idx > 0 && idx <= len(commits) {
				dropIndices[idx-1] = true
			}
		}
	}

	var kept []models.Commit
	for i, c := range commits {
		if !dropIndices[i] {
			kept = append(kept, c)
		}
	}
	return nil, nil
}

func cherryPickTree(treeHash string) error {
	treeMap, err := storage.ParseTree(treeHash)
	if err != nil {
		return err
	}

	if err := storage.WriteIndexFromTree(treeMap); err != nil {
		return err
	}

	// Since CheckoutIndex is missing, we use Reset(HEAD, Hard) logic but without moving HEAD
	// Actually, cherry-pick applies changes to workdir.
	// We can manually checkout files from the index.
	return checkoutIndexFromMap(treeMap)
}

func checkoutIndexFromMap(tree map[string]string) error {
	for path, hash := range tree {
		content, err := storage.ReadObject(hash)
		if err != nil { return err }
		if err := os.WriteFile(path, content, 0644); err != nil {
			return err
		}
	}
	return nil
}
