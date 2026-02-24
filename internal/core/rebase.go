package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/merge"
	"github.com/LeeFred3042U/kitcat/internal/models"
	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/storage"
	"github.com/LeeFred3042U/kitcat/internal/constant"
)

// RebaseAbort cancels an active rebase, restores the original HEAD, 
// and cleans up the sequencer state.
func RebaseAbort() error {
	if !IsRebaseInProgress() {
		return fmt.Errorf("fatal: No rebase in progress")
	}

	state, err := LoadRebaseState()
	if err != nil {
		return fmt.Errorf("failed to read rebase state: %w", err)
	}

	fmt.Printf("Aborting rebase; restoring HEAD to %s\n", state.OrigHead[:7])
	
	if err := hardResetTo(state.OrigHead); err != nil {
		return fmt.Errorf("failed to restore original HEAD: %w", err)
	}

	if err := ClearRebaseState(); err != nil {
		return fmt.Errorf("failed to clear rebase state: %w", err)
	}

	return nil
}

// RebaseContinue reads the sequencer state and replays the next commits
// using the 3-way merge engine. It handles pausing and resuming for conflicts.
func RebaseContinue() error {
	if !IsRebaseInProgress() {
		return fmt.Errorf("fatal: No rebase in progress")
	}

	state, err := LoadRebaseState()
	if err != nil {
		return fmt.Errorf("failed to read rebase state: %w", err)
	}

	// 1. PREFLIGHT: Check if we are resuming from a paused conflict
	stoppedShaPath := filepath.Join(constant.RepoDir, "rebase-merge", "stopped-sha")
	if stoppedHashBytes, err := os.ReadFile(stoppedShaPath); err == nil {
		// We are resuming! Check if the user actually resolved the conflicts in the index.
		index, _ := storage.LoadIndex()
		for path, entry := range index {
			if entry.Stage != 0 {
				return fmt.Errorf("you still have unmerged files in '%s'.\nFix them, run 'kitcat add', and then 'kitcat rebase --continue'", path)
			}
		}

		// Conflicts are resolved! Commit the paused step before continuing.
		stoppedHash := strings.TrimSpace(string(stoppedHashBytes))
		commitToApply, _ := storage.FindCommit(stoppedHash)
		
		if _, err := commitRebaseStep(commitToApply); err != nil {
			return fmt.Errorf("failed to commit resolved rebase step: %w", err)
		}

		os.Remove(stoppedShaPath) // Clear the pause marker
		state.CurrentStep++       // Move to the next step
		SaveRebaseState(*state)
	}

	// 2. THE REPLAY ENGINE LOOP
	for state.CurrentStep < len(state.TodoSteps) {
		stepLine := strings.TrimSpace(state.TodoSteps[state.CurrentStep])
		if stepLine == "" || strings.HasPrefix(stepLine, "#") {
			state.CurrentStep++
			continue
		}

		parts := strings.SplitN(stepLine, " ", 3)
		if len(parts) < 2 {
			return fmt.Errorf("invalid todo format: %s", stepLine)
		}
		action, hash := parts[0], parts[1]

		if action != "pick" && action != "p" {
			return fmt.Errorf("unsupported action '%s' (only 'pick' is currently supported)", action)
		}

		fmt.Printf("Rebasing (%d/%d): %s\n", state.CurrentStep+1, len(state.TodoSteps), stepLine)

		commitToApply, err := storage.FindCommit(hash)
		if err != nil {
			return fmt.Errorf("failed to find commit %s: %w", hash, err)
		}

		// --- 3-WAY MERGE PREPARATION ---
		baseTree := make(map[string]storage.TreeEntry)
		if commitToApply.Parent != "" {
			if baseCommit, err := storage.FindCommit(commitToApply.Parent); err == nil {
				baseTree, _ = storage.ParseTree(baseCommit.TreeHash)
			}
		}

		oursCommit, _ := storage.GetLastCommit()
		oursTree, _ := storage.ParseTree(oursCommit.TreeHash)
		theirsTree, _ := storage.ParseTree(commitToApply.TreeHash)

		// --- CALCULATE & APPLY MERGE ---
		plan := merge.MergeTrees(baseTree, oursTree, theirsTree)
		if err := merge.ApplyMergePlan(plan); err != nil {
			return fmt.Errorf("failed to apply rebase patch: %w", err)
		}

		// --- CONFLICT HANDLING ---
		if len(plan.Conflicts) > 0 {
			// Pause the sequencer by writing the stopped-sha
			os.WriteFile(stoppedShaPath, []byte(hash), 0644)
			
			fmt.Println("CONFLICT (content): Merge conflict in files:")
			for path := range plan.Conflicts {
				fmt.Printf("  - %s\n", path)
			}
			return fmt.Errorf("could not apply %s.\nFix conflicts, run 'kitcat add', and then 'kitcat rebase --continue'", hash[:7])
		}

		// --- CLEAN APPLY: COMMIT IMMEDIATELY ---
		if _, err := commitRebaseStep(commitToApply); err != nil {
			return fmt.Errorf("failed to commit %s: %w", hash, err)
		}

		state.CurrentStep++
		SaveRebaseState(*state)
	}

	// 3. REBASE COMPLETE! CLEANUP
	branchRef := strings.TrimPrefix(state.HeadName, "refs/heads/")
	// Matches canonical git output
	fmt.Printf("Successfully rebased and updated refs/heads/%s.\n", branchRef)
	return ClearRebaseState()
}

// commitRebaseStep replicates a commit during a rebase.
// It bypasses the standard Commit function to preserve the original Author
// and ensure single-parent ancestry (no accidental merge commits!).
func commitRebaseStep(original models.Commit) (string, error) {
	treeHash, err := plumbing.WriteTree(storage.IndexPath)
	if err != nil {
		return "", err
	}

	headCommit, err := storage.GetLastCommit()
	var parents []string
	if err == nil {
		parents = append(parents, headCommit.ID)
	}

	// Preserve the original author (The person who wrote the code)
	authorStr := fmt.Sprintf("%s <%s>", original.AuthorName, original.AuthorEmail)

	// Update the committer to the current user (The person running the rebase)
	name, _, _ := GetConfig("user.name")
	email, _, _ := GetConfig("user.email")
	if name == "" { name = "Unknown" }
	if email == "" { email = "unknown@example.com" }
	committerStr := fmt.Sprintf("%s <%s>", name, email)

	opts := plumbing.CommitOptions{
		Tree:      treeHash,
		Parents:   parents,
		Author:    authorStr,
		Committer: committerStr,
		Message:   original.Message,
	}

	newCommitHash, err := plumbing.CommitTree(opts)
	if err != nil {
		return "", err
	}

	// Safely update HEAD pointer using your existing unexported function in commit.go
	if err := updateHead(newCommitHash); err != nil {
		return "", err
	}

	return newCommitHash, nil
}

// GetCurrentBranch resolves HEAD and returns the active branch name.
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

// Rebase initializes the sequencer. It calculates the commits to replay,
// saves them to the todo list, resets the working directory to the target base,
// and then kicks off RebaseContinue.
func Rebase(targetBranch string, interactive bool) error {
	if IsRebaseInProgress() {
		return fmt.Errorf("fatal: A rebase is already in progress. Use --continue or --abort")
	}

	dirty, _ := IsWorkDirDirty()
	if dirty {
		return fmt.Errorf("fatal: cannot rebase: you have unstaged changes.\nPlease commit or stash them")
	}

	currentBranch, err := GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	headCommit, err := storage.GetLastCommit()
	if err != nil {
		return fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	targetHash, err := storage.GetRef("refs/heads/" + targetBranch)
	if err != nil {
		return fmt.Errorf("fatal: invalid branch '%s'", targetBranch)
	}

	mergeBase, err := storage.FindMergeBase(headCommit.ID, targetHash)
	if err != nil {
		return fmt.Errorf("failed to find merge base: %w", err)
	}

	// 1. Collect commits from HEAD down to mergeBase
	var commitsToRebase []models.Commit
	curr := headCommit.ID
	for curr != mergeBase && curr != "" {
		c, err := storage.FindCommit(curr)
		if err != nil {
			return err
		}
		// Prepend to array because we traverse backwards, but need to replay forwards!
		commitsToRebase = append([]models.Commit{c}, commitsToRebase...)
		curr = c.Parent
	}

	if len(commitsToRebase) == 0 {
		fmt.Println("Current branch is already up to date.")
		return nil
	}

	// 2. Generate the Git-standard "todo" list
	var todoSteps []string
	for _, c := range commitsToRebase {
		// Format: action hash message
		step := fmt.Sprintf("pick %s %s", c.ID, c.Message)
		todoSteps = append(todoSteps, step)
	}

	// 3. Interactive prompt (Allows modifying the todoSteps array)
	if interactive {
		todoSteps, err = promptInteractiveRebase(todoSteps)
		if err != nil {
			return fmt.Errorf("rebase aborted: %w", err)
		}
		if len(todoSteps) == 0 {
			fmt.Println("Successfully rebased and updated (nothing to do).")
			return nil
		}
	}

	fmt.Printf("Rebasing %s onto %s...\n", currentBranch, targetBranch)

	// 4. Save the sequencer state to disk
	state := RebaseState{
		HeadName:    "refs/heads/" + currentBranch,
		Onto:        targetHash,
		OrigHead:    headCommit.ID,
		TodoSteps:   todoSteps,
		CurrentStep: 0,
		Message:     "",
	}
	
	if err := SaveRebaseState(state); err != nil {
		return fmt.Errorf("failed to initialize rebase state: %w", err)
	}

	// 5. Checkout the target branch (the "onto" base)
	if err := hardResetTo(targetHash); err != nil {
		return fmt.Errorf("failed to checkout base commit: %w", err)
	}

	// 6. Start the replay engine
	return RebaseContinue()
}

// hardResetTo updates branch pointer then forces index and working tree
// to match the provided commit hash.
func hardResetTo(hash string) error {
	currentBranch, _ := GetCurrentBranch()
	refPath := ".kitcat/refs/heads/" + currentBranch
	
	// Atomic write
	if err := SafeWrite(refPath, []byte(hash), 0644); err != nil {
		return err
	}
	return Reset(hash, ResetHard)
}

// promptInteractiveRebase opens the todo list in the user's default editor,
// just like git rebase -i. The user can remove lines or change actions to 'drop'.
func promptInteractiveRebase(steps []string) ([]string, error) {
	var sb strings.Builder
	for _, step := range steps {
		sb.WriteString(step + "\n")
	}

	sb.WriteString("\n# Rebase interactive commands:\n")
	sb.WriteString("# p, pick <commit> = use commit\n")
	sb.WriteString("# d, drop <commit> = remove commit\n")
	sb.WriteString("#\n# These lines can be re-ordered; they are executed from top to bottom.\n")
	sb.WriteString("# If you remove a line here THAT COMMIT WILL BE LOST.\n")

	// Trigger the editor!
	editedData, err := CaptureViaEditor("git-rebase-todo", sb.String())
	if err != nil {
		return nil, fmt.Errorf("interactive rebase aborted: %w", err)
	}

	if strings.TrimSpace(editedData) == "" {
		return nil, fmt.Errorf("aborting rebase due to empty todo list")
	}

	var kept []string
	for _, line := range strings.Split(editedData, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) > 0 {
			action := parts[0]
			// Handle 'drop' natively
			if action == "drop" || action == "d" {
				continue
			}
			kept = append(kept, line)
		}
	}

	return kept, nil
}