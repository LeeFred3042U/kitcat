package core

import (
	"crypto/sha1"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/models"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// getEditor returns the user's preferred text editor from the EDITOR environment variable
// or defaults to common editors based on the OS
func getEditor() (string, []string, error) {
	if envEditor := os.Getenv("EDITOR"); envEditor != "" {
		return envEditor, []string{}, nil
	}

	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("code"); err == nil {
			return "code", []string{"--wait"}, nil
		}
		return "notepad", []string{}, nil
	}

	// Order of preference for Unix-like systems
	editors := []string{"code", "nano", "micro", "vim"}
	for _, e := range editors {
		if _, err := exec.LookPath(e); err == nil {
			if e == "code" {
				return e, []string{"--wait"}, nil
			}
			return e, []string{}, nil
		}
	}

	return "", nil, fmt.Errorf("no suitable editor found (checked code, nano, micro, vim)")
}

// RebaseInteractive starts an interactive rebase onto the specified commit
// returns an error if any operation fails
func RebaseInteractive(commitHash string) error {
	if !IsRepoInitialized() {
		return fmt.Errorf("not a kitcat repository")
	}

	isDirty, err := IsWorkDirDirty()
	if err != nil {
		return fmt.Errorf("failed to check working directory status: %w", err)
	}
	if isDirty {
		return fmt.Errorf("cannot rebase: you have unstaged changes")
	}

	ontoCommit, err := storage.FindCommit(commitHash)
	if err != nil {
		return fmt.Errorf("invalid base commit '%s': %w", commitHash, err)
	}

	headState, err := GetHeadState()
	if err != nil {
		return err
	}
	headHash, err := readHead()
	if err != nil {
		return err
	}
	rebaseHeadNameVal := ""
	if !strings.HasPrefix(headState, "HEAD") {
		rebaseHeadNameVal = "refs/heads/" + headState
	}

	commitsToRebase, err := getCommitsBetween(ontoCommit.ID, headHash)
	if err != nil {
		return err
	}
	if len(commitsToRebase) == 0 {
		fmt.Println("No commits to rebase.")
		return nil
	}

	todoPath := filepath.Join(RepoDir, "rebase-todo")
	todoContent := generateTodo(commitsToRebase)
	if err := os.WriteFile(todoPath, []byte(todoContent), 0o644); err != nil {
		return err
	}

	editor, editorArgs, err := getEditor()
	if err != nil {
		return err
	}

	cmdArgs := append(editorArgs, todoPath)
	cmd := exec.Command(editor, cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("Opening editor (%s) to modify rebase todo list...\n", editor)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run editor: %w", err)
	}

	newTodoContent, err := os.ReadFile(todoPath)
	if err != nil {
		return err
	}
	steps := parseTodo(string(newTodoContent))
	if len(steps) == 0 {
		fmt.Println("Nothing to do.")
		return nil
	}

	state := RebaseState{
		HeadName:    rebaseHeadNameVal,
		Onto:        ontoCommit.ID,
		OrigHead:    headHash,
		TodoSteps:   steps,
		CurrentStep: 0,
	}
	if err := SaveRebaseState(state); err != nil {
		return err
	}

	// Create temporary branch at ontoCommit
	// This branch will be used as the new HEAD during the rebase
	// It will be deleted after the rebase completes or is aborted
	tmpBranch := "kitcat-rebase-tmp"
	tmpBranchPath := filepath.Join(".kitcat", "refs", "heads", tmpBranch)
	if err := os.MkdirAll(filepath.Dir(tmpBranchPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(tmpBranchPath, []byte(ontoCommit.ID), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(".kitcat/HEAD", []byte("ref: refs/heads/"+tmpBranch), 0o644); err != nil {
		return fmt.Errorf("failed to update HEAD: %w", err)
	}
	if err := UpdateWorkspaceAndIndex(ontoCommit.ID); err != nil {
		return fmt.Errorf("failed to checkout base: %w", err)
	}

	return RunRebaseLoop()
}

// RebaseContinue continues the ongoing rebase process after conflicts are resolved
// returns an error if no rebase is in progress or if any operation fails
func RebaseContinue() error {
	if !IsRebaseInProgress() {
		return fmt.Errorf("no rebase in progress")
	}

	state, err := LoadRebaseState()
	if err != nil {
		return err
	}

	if state.CurrentStep >= len(state.TodoSteps) {
		return fmt.Errorf("no steps remaining")
	}

	currentCmdLine := state.TodoSteps[state.CurrentStep]
	parts := strings.Fields(currentCmdLine)
	cmd := parts[0]

	if len(parts) < 2 {
		return AdvanceRebaseStep(state)
	}
	originalHash := parts[1]
	originalCommit, _ := storage.FindCommit(originalHash)

	switch cmd {
	case "pick", "reword":
		msg := originalCommit.Message
		_, _, err := Commit(msg)
		if err != nil {
			if strings.Contains(err.Error(), "nothing to commit") {
				fmt.Println("Nothing to commit. Skipping step.")
			} else {
				return err
			}
		}

		if cmd == "reword" {
			head, _ := readHead()
			newMsg := promptForMessage(msg)
			if newMsg != msg {
				if err := amendCommitMessage(head, newMsg); err != nil {
					return err
				}
			}
		}

	case "squash":
		prevHead, _ := GetHeadCommit()
		newMsg := prevHead.Message + "\n\n" + originalCommit.Message
		err := amendCommit(prevHead, newMsg)
		if err != nil {
			return err
		}
	}

	if err := AdvanceRebaseStep(state); err != nil {
		return err
	}
	return RunRebaseLoop()
}

// RebaseAbort aborts the ongoing rebase and restores the original HEAD and working directory
// returns an error if no rebase is in progress or if any operation fails
func RebaseAbort() error {
	if !IsRebaseInProgress() {
		return fmt.Errorf("no rebase in progress")
	}
	state, err := LoadRebaseState()
	if err != nil {
		return err
	}

	fmt.Printf("Aborting rebase. restoring HEAD to %s\n", state.OrigHead[:7])

	if state.HeadName != "" {
		if err := os.WriteFile(".kitcat/HEAD", []byte("ref: "+state.HeadName), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(".kitcat", state.HeadName), []byte(state.OrigHead), 0o644); err != nil {
			return err
		}
		if err := UpdateWorkspaceAndIndex(state.OrigHead); err != nil {
			return err
		}
	} else {
		if err := ResetHard(state.OrigHead); err != nil {
			return err
		}
	}

	os.Remove(filepath.Join(".kitcat", "refs", "heads", "kitcat-rebase-tmp"))
	return ClearRebaseState()
}

// RunRebaseLoop processes the rebase steps in a loop until completion or conflict
// returns an error if any operation fails
func RunRebaseLoop() error {
	for {
		cmdLine, state, err := ReadNextTodo()
		if err != nil {
			return err
		}
		if state.CurrentStep >= len(state.TodoSteps) {
			fmt.Println("Rebase completed successfully.")
			return finishRebase(state)
		}

		parts := strings.Fields(cmdLine)
		if len(parts) < 2 {
			if err := AdvanceRebaseStep(state); err != nil {
				return err
			}
			continue
		}
		action := parts[0]
		commitHash := parts[1]

		fmt.Printf("Rebase (%d/%d): %s\n", state.CurrentStep+1, len(state.TodoSteps), cmdLine)

		var stepErr error
		switch action {
		case "pick", "p":
			stepErr = executePick(commitHash)
		case "reword", "r":
			stepErr = executeReword(commitHash)
		case "squash", "s":
			stepErr = executeSquash(commitHash)
		case "drop", "d":
			fmt.Printf("Dropping commit %s\n", commitHash)
			stepErr = nil
		default:
			fmt.Printf("Unknown command '%s'. Skipping.\n", action)
		}

		if stepErr != nil {
			fmt.Printf("Conflict or error at step %d: %v\n", state.CurrentStep+1, stepErr)
			fmt.Println("Resolve conflicts, then run 'kitcat rebase --continue'.")
			fmt.Println("To stop, run 'kitcat rebase --abort'.")
			return nil
		}

		if err := AdvanceRebaseStep(state); err != nil {
			return err
		}
	}
}

// finishRebase finalizes the rebase by updating HEAD and cleaning up temporary state
// returns an error if any operation fails
func finishRebase(state *RebaseState) error {
	headHash, err := readHead()
	if err != nil {
		return err
	}

	if state.HeadName != "" {
		if err := os.WriteFile(".kitcat/HEAD", []byte("ref: "+state.HeadName), 0o644); err != nil {
			return err
		}
		refPath := filepath.Join(".kitcat", state.HeadName)
		if err := os.WriteFile(refPath, []byte(headHash), 0o644); err != nil {
			return err
		}
	}

	os.Remove(filepath.Join(".kitcat", "refs", "heads", "kitcat-rebase-tmp"))
	return ClearRebaseState()
}

// executePick applies the changes from the commit with the given hash onto the current HEAD
// creates a new commit with the same message
func executePick(hash string) error {
	return cherryPick(hash, false)
}

// executeReword applies the changes from the commit with the given hash onto the current HEAD
// and prompts the user to edit the commit message
func executeReword(hash string) error {
	if err := cherryPick(hash, false); err != nil {
		return err
	}
	head, _ := GetHeadCommit()
	newMsg := promptForMessage(head.Message)
	return amendCommitMessage(head.ID, newMsg)
}

// executeSquash applies the changes from the commit with the given hash onto the current HEAD
// and amends the previous commit with a combined message
func executeSquash(hash string) error {
	if err := cherryPick(hash, true); err != nil {
		return err
	}
	prevHead, _ := GetHeadCommit()
	targetCommit, _ := storage.FindCommit(hash)
	newMsg := prevHead.Message + "\n\n" + targetCommit.Message
	return amendCommit(prevHead, newMsg)
}

// cherryPick applies the changes from the commit with the given hash onto the current HEAD
// if noCommit is true, it applies the changes without creating a new commit
// returns an error if any conflicts are detected
func cherryPick(hash string, noCommit bool) error {
	commit, err := storage.FindCommit(hash)
	if err != nil {
		return err
	}
	parentHash := commit.Parent
	changes, err := getChanges(parentHash, hash)
	if err != nil {
		return err
	}
	if err := applyChanges(changes); err != nil {
		return err
	}
	if noCommit {
		return nil
	}
	_, _, err = Commit(commit.Message)
	if err != nil && strings.Contains(err.Error(), "nothing to commit") {
		return nil
	}
	return err
}

type Change struct {
	OldHash string
	NewHash string
}

// getChanges computes the changes between parentHash and childHash
// returns a map of file paths to their old and new hashes
func getChanges(parentHash, childHash string) (map[string]Change, error) {
	parentTree := make(map[string]string)
	if parentHash != "" {
		pC, err := storage.FindCommit(parentHash)
		if err == nil {
			parentTree, _ = storage.ParseTree(pC.TreeHash)
		}
	}

	childCommit, err := storage.FindCommit(childHash)
	if err != nil {
		return nil, err
	}
	childTree, err := storage.ParseTree(childCommit.TreeHash)
	if err != nil {
		return nil, err
	}

	changes := make(map[string]Change)
	for path, hash := range childTree {
		if pHash, ok := parentTree[path]; !ok || pHash != hash {
			changes[path] = Change{OldHash: parentTree[path], NewHash: hash}
		}
	}
	for path := range parentTree {
		if _, ok := childTree[path]; !ok {
			changes[path] = Change{OldHash: parentTree[path], NewHash: ""}
		}
	}
	return changes, nil
}

// applyChanges applies the given changes to the working directory and index
// returns an error if any conflicts are detected
func applyChanges(changes map[string]Change) error {
	headCommit, _ := GetHeadCommit()
	headTree, _ := storage.ParseTree(headCommit.TreeHash)

	for path, change := range changes {
		targetHash := change.NewHash
		if targetHash == "" {
			headFileHash, existsInHead := headTree[path]
			if existsInHead && headFileHash != change.OldHash {
				return fmt.Errorf(
					"conflict in %s: deleted in incoming commit, but modified in HEAD",
					path,
				)
			}
			if err := RemoveFile(path, true); err != nil {
				return err
			}
		} else {
			content, err := storage.ReadObject(targetHash)
			if err != nil {
				return err
			}
			headFileHash, existsInHead := headTree[path]
			if existsInHead {
				if headFileHash != change.OldHash {
					return fmt.Errorf("conflict in %s: modified in incoming commit, but modified in HEAD", path)
				}
			} else if change.OldHash != "" {
				return fmt.Errorf("conflict in %s: modified in incoming commit, but deleted in HEAD", path)
			}

			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, content, 0o644); err != nil {
				return err
			}
			if err := AddFile(path); err != nil {
				return err
			}
		}
	}
	return nil
}

// generateTodo generates the initial todo content for the given commit hashes
func generateTodo(hashes []string) string {
	var sb strings.Builder
	for _, h := range hashes {
		c, _ := storage.FindCommit(h)
		sb.WriteString(fmt.Sprintf("pick %s %s\n", h, c.Message))
	}
	sb.WriteString("\n# Commands:\n")
	sb.WriteString("# p, pick <commit> = use commit\n")
	sb.WriteString("# r, reword <commit> = use commit, but edit the commit message\n")
	sb.WriteString("# s, squash <commit> = use commit, but meld into previous commit\n")
	sb.WriteString("# d, drop <commit> = remove commit\n")
	return sb.String()
}

// parseTodo parses the todo content and returns a list of steps
// ignores comments and empty lines
func parseTodo(content string) []string {
	steps := make([]string, 0)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		steps = append(steps, line)
	}
	return steps
}

// getCommitsBetween returns a list of commit hashes from start (exclusive) to end (inclusive)
// in chronological order
func getCommitsBetween(start, end string) ([]string, error) {
	var chain []string
	curr := end
	for curr != "" {
		if curr == start {
			break
		}
		chain = append(chain, curr)
		c, err := storage.FindCommit(curr)
		if err != nil {
			return nil, err
		}
		if c.Parent == "" {
			if start == "" {
				return chain, nil
			}
			break
		}
		curr = c.Parent
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

// promptForMessage opens the user's editor to edit the commit message, starting with defaultMsg
// and returns the edited message
func promptForMessage(defaultMsg string) string {
	tmp := ".kitcat/COMMIT_EDITMSG"
	if err := os.WriteFile(tmp, []byte(defaultMsg), 0o644); err != nil {
		fmt.Printf("Warning: failed to write temp commit msg: %v\n", err)
	}

	editor, editorArgs, err := getEditor()
	if err != nil {
		fmt.Printf("Warning: %v. Using default message.\n", err)
		return defaultMsg
	}

	cmdArgs := append(editorArgs, tmp)
	cmd := exec.Command(editor, cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: editor exited with error: %v\n", err)
	}

	out, _ := os.ReadFile(tmp)
	return strings.TrimSpace(string(out))
}

// amendCommitMessage creates a new commit with the same tree and parent as commitID but with newVal
// and updates the current branch to point to it
func amendCommitMessage(commitID, newVal string) error {
	c, err := storage.FindCommit(commitID)
	if err != nil {
		return err
	}
	parentBlock := ""
	if c.Parent != "" {
		parentBlock = "parent " + c.Parent + "\n"
	}
	content := fmt.Sprintf("tree %s\n%s\n%s", c.TreeHash, parentBlock, newVal)
	newHash, err := saveObject([]byte(content))
	if err != nil {
		return err
	}
	return UpdateBranchPointer(newHash)
}

// amendCommit creates a new commit with the same tree and parent as prevHead but with newMsg
// and updates the current branch to point to it
func amendCommit(prevHead models.Commit, newMsg string) error {
	treeHash, err := storage.CreateTree()
	if err != nil {
		return err
	}
	parentBlock := ""
	if prevHead.Parent != "" {
		parentBlock = "parent " + prevHead.Parent + "\n"
	}
	content := fmt.Sprintf("tree %s\n%s\n%s", treeHash, parentBlock, newMsg)
	newHash, err := saveObject([]byte(content))
	if err != nil {
		return err
	}
	return UpdateBranchPointer(newHash)
}

// saveObject saves the given content as an object and returns its hash
func saveObject(content []byte) (string, error) {
	h := sha1.New()
	h.Write(content)
	hash := fmt.Sprintf("%x", h.Sum(nil))
	objPath := filepath.Join(".kitcat", "objects", hash)
	if err := os.MkdirAll(filepath.Dir(objPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(objPath, content, 0o644); err != nil {
		return "", err
	}
	return hash, nil
}
