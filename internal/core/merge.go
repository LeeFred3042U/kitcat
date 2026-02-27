package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/merge"
	"github.com/LeeFred3042U/kitcat/internal/repo"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Merge attempts to merge the specified branch into the current branch.
// It supports fast-forward and 3-way merges.
func Merge(branchToMerge string) error {
	// Ensure we are inside a repository before performing destructive operations.
	if _, err := os.Stat(repo.Dir); os.IsNotExist(err) {
		return fmt.Errorf("not a %s repository (run `%s init`)", app.Name, app.Name)
	}

	// Abort if working directory has local modifications.
	dirty, err := IsWorkDirDirty()
	if err != nil {
		return fmt.Errorf("failed to check working directory status: %w", err)
	}
	if dirty {
		return fmt.Errorf("error: your local changes would be overwritten by merge. Please commit or stash them")
	}

	// Resolve target branch head.
	branchPath := filepath.Join(repo.HeadsDir, branchToMerge)
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

	// --- FAST-FORWARD MERGE ---
	if mergeBase == currentHeadHash {
		fmt.Printf("Updating %s..%s\n", currentHeadHash[:7], featureHeadHash[:7])
		fmt.Println("Fast-forward")
		
		if err := UpdateBranchPointer(featureHeadHash); err != nil {
			return fmt.Errorf("failed to update branch pointer: %w", err)
		}
		if err := UpdateWorkspaceAndIndex(featureHeadHash); err != nil {
			_ = UpdateBranchPointer(currentHeadHash) // Rollback on failure
			return fmt.Errorf("failed to update workspace: %w", err)
		}
		return nil
	}

	// --- ALREADY UP-TO-DATE ---
	if mergeBase == featureHeadHash {
		fmt.Println("Already up to date.")
		return nil
	}

	// --- 3-WAY MERGE ---
	fmt.Printf("Auto-merging %s\n", branchToMerge)

	baseCommit, _ := storage.FindCommit(mergeBase)
	oursCommit, _ := storage.FindCommit(currentHeadHash)
	theirsCommit, _ := storage.FindCommit(featureHeadHash)

	baseTree, _ := storage.ParseTree(baseCommit.TreeHash)
	oursTree, _ := storage.ParseTree(oursCommit.TreeHash)
	theirsTree, _ := storage.ParseTree(theirsCommit.TreeHash)

	// 1. Calculate Pure Merge Plan (Layer 1)
	plan := merge.MergeTrees(baseTree, oursTree, theirsTree)

	// 2. Apply the Plan to Workspace & Index (Layer 2)
	if err := merge.ApplyMergePlan(plan); err != nil {
		return fmt.Errorf("failed to apply merge plan: %w", err)
	}

	// 3. Write Merge State for the upcoming commit
	SafeWrite(filepath.Join(repo.Dir, "MERGE_HEAD"), []byte(featureHeadHash), 0644)
	
	currentBranch, _ := GetHeadState()
	mergeMsg := fmt.Sprintf("Merge branch '%s' into '%s'\n", branchToMerge, currentBranch)
	SafeWrite(filepath.Join(repo.Dir, "MERGE_MSG"), []byte(mergeMsg), 0644)

	// 4. Handle Conflicts UX
	if len(plan.Conflicts) > 0 {
		fmt.Println("CONFLICT (content): Merge conflict in files.")
		for path := range plan.Conflicts {
			fmt.Printf("CONFLICT (content): Merge conflict in %s\n", path)
		}
		return fmt.Errorf("Automatic merge failed; fix conflicts and then commit the result.")
	}

	// 5. Clean Merge UX
	fmt.Printf("Merge successful. Run `%s commit` to finalize the merge commit.\n", app.Name)
	return nil
}

// MergeAbort cancels an active merge conflict state, restores the original HEAD,
// and cleans up the merge state files.
func MergeAbort() error {
	mergeHeadPath := filepath.Join(repo.Dir, "MERGE_HEAD")
	if _, err := os.Stat(mergeHeadPath); os.IsNotExist(err) {
		return fmt.Errorf("fatal: There is no merge to abort (MERGE_HEAD missing).")
	}

	headCommit, err := GetHeadCommit()
	if err != nil {
		return fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	// Hard reset to the current HEAD to wipe out the dirty working tree and index
	if err := Reset(headCommit.ID, ResetHard); err != nil {
		return fmt.Errorf("failed to restore original HEAD state: %w", err)
	}

	// Scrub the state files
	os.Remove(mergeHeadPath)
	os.Remove(filepath.Join(repo.Dir, "MERGE_MSG"))

	return nil
}
