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

// Merge integrates the specified branch into the current branch
// Supports fast-forward and true 3-way merges
// Fails if the working directory is dirty
func Merge(branchToMerge string) error {
	// Verify repository exists
	if _, err := os.Stat(repo.Dir); os.IsNotExist(err) {
		return fmt.Errorf("not a %s repository (run `%s init`)", app.Name, app.Name)
	}

	// Ensure working tree is clean before merge
	dirty, err := IsWorkDirDirty()
	if err != nil {
		return fmt.Errorf("failed to check working directory status: %w", err)
	}
	if dirty {
		return fmt.Errorf("error: your local changes would be overwritten by merge. Please commit or stash them")
	}

	// Resolve target branch HEAD
	branchPath := filepath.Join(repo.HeadsDir, branchToMerge)
	featureHeadHashBytes, err := os.ReadFile(branchPath)
	if err != nil {
		return fmt.Errorf("branch '%s' not found", branchToMerge)
	}
	featureHeadHash := strings.TrimSpace(string(featureHeadHashBytes))

	// Read current HEAD
	currentHeadHash, err := readHead()
	if err != nil {
		return fmt.Errorf("could not read current HEAD: %w", err)
	}

	// Compute merge base to determine strategy
	mergeBases, err := storage.FindMergeBases(currentHeadHash, featureHeadHash)
	if err != nil {
		return fmt.Errorf("failed to calculate merge base: %w", err)
	}

	mergeBase, err := selectBestMergeBase(mergeBases)
	if err != nil {
		return err
	}

	// Fast-forward: current HEAD is behind target
	if mergeBase == currentHeadHash {
		fmt.Printf("Updating %s..%s\n", currentHeadHash[:7], featureHeadHash[:7])
		fmt.Println("Fast-forward")

		// Apply tree to index and working directory first
		// Ref update is last to avoid pointing to an unmaterialized tree
		if err := UpdateWorkspaceAndIndex(featureHeadHash); err != nil {
			return fmt.Errorf("failed to update workspace: %w", err)
		}

		// Move branch pointer after successful materialization
		if err := UpdateBranchPointer(featureHeadHash); err != nil {
			return fmt.Errorf("failed to update branch pointer: %w", err)
		}

		return nil
	}

	// No-op: target already contained in current branch
	if mergeBase == featureHeadHash {
		fmt.Println("Already up to date.")
		return nil
	}

	// Perform 3-way merge
	fmt.Printf("Auto-merging %s\n", branchToMerge)

	baseCommit, err := storage.FindCommit(mergeBase)
	if err != nil {
		return fmt.Errorf("failed to load base commit %s: %w", mergeBase, err)
	}

	oursCommit, err := storage.FindCommit(currentHeadHash)
	if err != nil {
		return fmt.Errorf("failed to load ours commit %s: %w", currentHeadHash, err)
	}

	theirsCommit, err := storage.FindCommit(featureHeadHash)
	if err != nil {
		return fmt.Errorf("failed to load theirs commit %s: %w", featureHeadHash, err)
	}

	baseTree, err := storage.ParseTree(baseCommit.TreeHash)
	if err != nil {
		return fmt.Errorf("failed to parse base tree (%s): %w", baseCommit.TreeHash, err)
	}

	oursTree, err := storage.ParseTree(oursCommit.TreeHash)
	if err != nil {
		return fmt.Errorf("failed to parse ours tree (%s): %w", oursCommit.TreeHash, err)
	}

	theirsTree, err := storage.ParseTree(theirsCommit.TreeHash)
	if err != nil {
		return fmt.Errorf("failed to parse theirs tree (%s): %w", theirsCommit.TreeHash, err)
	}

	// Compute merge plan from three trees
	plan := merge.MergeTrees(baseTree, oursTree, theirsTree)

	// Apply merge result to index and working directory
	if err := merge.ApplyMergePlan(plan); err != nil {
		return fmt.Errorf("failed to apply merge plan: %w", err)
	}

	// Persist merge metadata for subsequent commit
	SafeWrite(filepath.Join(repo.Dir, "MERGE_HEAD"), []byte(featureHeadHash), 0o644)

	currentBranch, _ := GetHeadState()
	mergeMsg := fmt.Sprintf("Merge branch '%s' into '%s'\n", branchToMerge, currentBranch)
	SafeWrite(filepath.Join(repo.Dir, "MERGE_MSG"), []byte(mergeMsg), 0o644)

	// Report conflicts; user must resolve and commit
	if len(plan.Conflicts) > 0 {
		fmt.Println("CONFLICT (content): Merge conflict in files.")
		for path := range plan.Conflicts {
			fmt.Printf("CONFLICT (content): Merge conflict in %s\n", path)
		}
		return fmt.Errorf("Automatic merge failed; fix conflicts and then commit the result.")
	}

	// Clean merge; commit required to finalize
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
