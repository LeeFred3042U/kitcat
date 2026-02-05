package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/models"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// UpdateWorkspaceAndIndex resets the working directory and index to match a specific commit.
// This is shared logic used by checkout, merge, and reset commands.
func UpdateWorkspaceAndIndex(commitHash string) error {
	commit, err := storage.FindCommit(commitHash)
	if err != nil {
		return err
	}
	targetTree, err := storage.ParseTree(commit.TreeHash)
	if err != nil {
		return err
	}

	// Delete files from the current index that are not in the target tree
	currentIndex, _ := storage.LoadIndex()
	for path := range currentIndex {
		if _, existsInTarget := targetTree[path]; !existsInTarget {
			os.Remove(path)
		}
	}

	// Write/update files from the target tree
	for path, hash := range targetTree {
		content, err := storage.ReadObject(hash)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := SafeWrite(path, content, 0o644); err != nil {
			return err
		}
	}

	// Update the index to match the new tree
	return storage.WriteIndex(targetTree)
}

// GetHeadState returns the current branch name or detached HEAD state.
// Returns the branch name (e.g., "main") if on a branch, or a detached HEAD description.
func GetHeadState() (string, error) {
	headData, err := os.ReadFile(HeadPath)
	if err != nil {
		return "", err
	}
	ref := strings.TrimSpace(string(headData))

	// Check if HEAD points to a branch
	if strings.HasPrefix(ref, "ref: ") {
		refPath := strings.TrimPrefix(ref, "ref: ")
		// Extract branch name from refs/heads/<branch>
		if strings.HasPrefix(refPath, "refs/heads/") {
			return strings.TrimPrefix(refPath, "refs/heads/"), nil
		}
		return refPath, nil
	}

	// Detached HEAD - ref contains a commit hash
	if len(ref) >= 7 {
		return fmt.Sprintf("HEAD (detached at %s)", ref[:7]), nil
	}
	return "HEAD (detached)", nil
}

// IsWorkDirDirty checks if there are uncommitted changes in the working directory or staging area.
// Returns true if there are any staged or unstaged changes, false if the working tree is clean.
func IsWorkDirDirty() (bool, error) {
	// Load the tree from the last commit (HEAD)
	headTree := make(map[string]string)
	lastCommit, err := GetHeadCommit() // Use GetHeadCommit, not storage.GetLastCommit
	if err == nil {
		tree, parseErr := storage.ParseTree(lastCommit.TreeHash)
		if parseErr != nil {
			return false, parseErr
		}
		headTree = tree
	} else if !os.IsNotExist(err) && !strings.Contains(err.Error(), "no such file") && !strings.Contains(err.Error(), "cannot find the file") {
		// If the error is NOT "file not found" (meaning no branch tip yet), return it.
		return false, err
	}

	// Load the current staging area
	index, err := storage.LoadIndex()
	if err != nil {
		return false, err
	}

	// Check for staged changes (Index vs. HEAD)
	allPaths := make(map[string]bool)
	for path := range headTree {
		allPaths[path] = true
	}
	for path := range index {
		allPaths[path] = true
	}

	for path := range allPaths {
		headHash, inHead := headTree[path]
		indexHash, inIndex := index[path]

		// If there's any difference between HEAD and index, working dir is dirty
		if (inIndex && !inHead) || (!inIndex && inHead) ||
			(inIndex && inHead && headHash != indexHash) {
			return true, nil
		}
	}

	// Load ignore patterns
	ignorePatterns, err := LoadIgnorePatterns()
	if err != nil {
		return false, err
	}

	// Check for unstaged changes (Working Directory vs. Index)
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		cleanPath := filepath.Clean(path)

		// Skip the .kitcat directory and other directories
		if info.IsDir() || strings.HasPrefix(cleanPath, RepoDir+string(os.PathSeparator)) ||
			cleanPath == RepoDir {
			return nil
		}

		indexHash, isTracked := index[cleanPath]

		// If the file is not in the index, check if it should be ignored
		if !isTracked {
			// Check if file should be ignored
			if ShouldIgnore(cleanPath, ignorePatterns, index) {
				return nil // Skip ignored files
			}
			return fmt.Errorf("untracked") // Use error to signal dirty state
		}

		// If the file is tracked, hash it and compare with the index
		currentHash, hashErr := storage.HashFile(cleanPath)
		if hashErr != nil {
			return hashErr
		}
		if currentHash != indexHash {
			return fmt.Errorf("modified") // Use error to signal dirty state
		}
		return nil
	})
	// If we got an "untracked" or "modified" error, the working dir is dirty
	if err != nil {
		if err.Error() == "untracked" || err.Error() == "modified" {
			return true, nil
		}
		return false, err
	}

	return false, nil
}

// UpdateBranchPointer updates the current branch pointer or HEAD to point to a specific commit.
// Handles both branch mode (updates refs/heads/<branch>) and detached HEAD mode (updates HEAD directly).
func UpdateBranchPointer(commitHash string) error {
	headData, err := os.ReadFile(HeadPath)
	if err != nil {
		return fmt.Errorf("unable to read HEAD file: %w", err)
	}
	ref := strings.TrimSpace(string(headData))

	// Case A: HEAD points to a branch (ref: refs/heads/<branch>)
	if strings.HasPrefix(ref, "ref: ") {
		refPath := strings.TrimPrefix(ref, "ref: ")
		branchFile := filepath.Join(".kitcat", refPath)

		// Verify branch file exists
		if _, err := os.Stat(branchFile); err != nil {
			branchName := strings.TrimPrefix(refPath, "refs/heads/")
			return fmt.Errorf("current branch %s not found", branchName)
		}

		// Update the branch pointer
		if err := SafeWrite(branchFile, []byte(commitHash), 0o644); err != nil {
			return fmt.Errorf("failed to update branch pointer: %w", err)
		}
		return nil
	}

	// Case B: Detached HEAD (HEAD contains a commit hash directly)
	if err := SafeWrite(HeadPath, []byte(commitHash), 0o644); err != nil {
		return fmt.Errorf("failed to update HEAD: %w", err)
	}
	return nil
}

// readHead returns the commit hash that HEAD currently points to.
// This is useful for rollback operations.
func readHead() (string, error) {
	headData, err := os.ReadFile(HeadPath)
	if err != nil {
		return "", err
	}
	ref := strings.TrimSpace(string(headData))

	// If HEAD points to a branch, read the branch file
	if strings.HasPrefix(ref, "ref: ") {
		refPath := strings.TrimPrefix(ref, "ref: ")
		branchFile := filepath.Join(".kitcat", refPath)
		commitHash, err := os.ReadFile(branchFile)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(commitHash)), nil
	}

	// Detached HEAD - ref is the commit hash
	return ref, nil
}

// IsSafePath checks if a file path is safe to use (prevents path traversal attacks).
// Returns false if the path attempts to escape the repository directory.
func IsSafePath(path string) bool {
	cleanPath := filepath.Clean(path)
	if filepath.IsAbs(cleanPath) {
		return false
	}
	if strings.Contains(cleanPath, "..") {
		return false
	}
	// Check for ASCII control characters (0x00-0x1F, 0x7F)
	for _, r := range cleanPath {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}

// GetHeadCommit returns the commit that HEAD currently points to.
// This differs from storage.GetLastCommit() which returns the last commit in the log.
// After a reset, HEAD might point to an earlier commit than the last one in the log.
func GetHeadCommit() (models.Commit, error) {
	// Get the commit hash that HEAD points to
	commitHash, err := readHead()
	if err != nil {
		return models.Commit{}, err
	}

	// Find and return that commit
	return storage.FindCommit(commitHash)
}

// IsRepoInitialized checks if the current directory or any parent is a valid kitcat repository.
// If found, it changes the current working directory to the repository root.
func IsRepoInitialized() bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}

	for {
		if _, err := os.Stat(filepath.Join(cwd, RepoDir)); err == nil {
			// Found the repository root
			// Update the working directory to the repo root so that
			// all relative paths (RepoDir, etc.) are valid.
			if err := os.Chdir(cwd); err != nil {
				return false
			}
			return true
		}

		parent := filepath.Dir(cwd)
		if parent == cwd {
			// Reached the system root without finding .kitcat
			return false
		}
		cwd = parent
	}
}

// Write data in safe way
func SafeWrite(filename string, data []byte, perm os.FileMode) error {
	dirPath := filepath.Dir(filename)

	// Create temp file
	f, err := os.CreateTemp(dirPath, "atomic-")
	if err != nil {
		return err
	}
	tmpName := f.Name()

	// Ensure cleanup of the temp file if we exit early
	defer os.Remove(tmpName)

	// Write data
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}

	// Set Permissions
	if err := f.Chmod(perm); err != nil {
		f.Close()
		return err
	}

	// Sync the file content
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	f.Close()

	if err := os.Rename(tmpName, filename); err != nil {
		return err
	}

	// Sync Directory - only on Unix-like systems
	// On Windows, syncing a directory causes "Access is denied" error
	return syncDir(dirPath)
}

// EnsureArgs validates that the number of arguments is within [min, max].
// passing max = -1 means no upper limit.
// commandName is used for error messages.
func EnsureArgs(args []string, min, max int, commandName string) {
	if min < 0 {
		panic(fmt.Sprintf("EnsureArgs: min arguments must be non-negative, got %d", min))
	}
	if max != -1 {
		if max < 0 {
			panic(fmt.Sprintf("EnsureArgs: max arguments must be non-negative (or -1 for no limit), got %d", max))
		}
		if max < min {
			panic(fmt.Sprintf("EnsureArgs: max arguments (%d) cannot be less than min arguments (%d)", max, min))
		}
	}

	count := len(args)
	if count < min {
		if min == 1 {
			fmt.Printf("Error: %s command requires at least 1 argument\n", commandName)
		} else {
			fmt.Printf("Error: %s command requires at least %d arguments\n", commandName, min)
		}
		os.Exit(2)
	}

	if max != -1 && count > max {
		if max == 0 {
			fmt.Printf("Error: %s command does not accept arguments\n", commandName)
		} else {
			fmt.Printf("Error: %s command accepts at most %d arguments\n", commandName, max)
		}
		os.Exit(2)
	}
}
