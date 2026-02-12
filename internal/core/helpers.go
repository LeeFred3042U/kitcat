package core

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/models"
	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// UpdateWorkspaceAndIndex resets working dir and index to match commit
func UpdateWorkspaceAndIndex(commitHash string) error {
	commit, err := storage.FindCommit(commitHash)
	if err != nil {
		return err
	}
	targetTree, err := storage.ParseTree(commit.TreeHash)
	if err != nil {
		return err
	}

	currentIndex, _ := storage.LoadIndex()
	for path := range currentIndex {
		if _, exists := targetTree[path]; !exists {
			os.Remove(path)
		}
	}

	for path, hash := range targetTree {
		content, err := storage.ReadObject(hash)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, content, 0644); err != nil {
			return err
		}
	}

	// Update Index using the target tree map
	return storage.WriteIndex(targetTree)
}

// GetHeadState returns the current branch name or commit hash if detached
func GetHeadState() (string, error) {
	headContent, err := os.ReadFile(".kitcat/HEAD")
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(string(headContent))
	if strings.HasPrefix(content, "ref: refs/heads/") {
		return strings.TrimPrefix(content, "ref: refs/heads/"), nil
	}
	return content, nil
}

// IsDetachedHead checks if the repository is in detached HEAD state
func IsDetachedHead() (bool, error) {
	headContent, err := os.ReadFile(".kitcat/HEAD")
	if err != nil {
		return false, err
	}
	return !strings.HasPrefix(string(headContent), "ref: "), nil
}

// VerifyCleanTree checks if the working directory matches the current HEAD
func VerifyCleanTree() error {
	index, err := storage.LoadIndex()
	if err != nil {
		return err
	}

	head, err := storage.GetLastCommit()
	if err != nil {
		if err == storage.ErrNoCommits && len(index) == 0 {
			return nil
		}
		return err
	}

	headTree, err := storage.ParseTree(head.TreeHash)
	if err != nil {
		return err
	}

	if len(index) != len(headTree) {
		return fmt.Errorf("working tree is dirty")
	}

	for path, entry := range index {
		headHash, ok := headTree[path]
		if !ok {
			return fmt.Errorf("dirty: %s", path)
		}

		// Fix: Convert [20]byte to hex string
		entryHashHex := hex.EncodeToString(entry.Hash[:])
		if entryHashHex != headHash {
			return fmt.Errorf("dirty: %s", path)
		}
	}
	return nil
}

// RestoreIndexFromCommit updates the index to match a specific commit
func RestoreIndexFromCommit(commitID string) error {
	commit, err := storage.FindCommit(commitID)
	if err != nil {
		return err
	}

	tree, err := storage.ParseTree(commit.TreeHash)
	if err != nil {
		return err
	}

	return storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {
		// Clear existing
		for k := range index {
			delete(index, k)
		}

		for path, hash := range tree {
			hb, _ := storage.HexToHash(hash)
			index[path] = plumbing.IndexEntry{
				Path: path,
				Hash: hb,
				Mode: 0100644,
			}
		}
		return nil
	})
}

// IsWorkDirDirty checks if there are uncommitted changes
func IsWorkDirDirty() (bool, error) {
	headTree := make(map[string]string)
	lastCommit, err := GetHeadCommit()
	if err == nil {
		tree, parseErr := storage.ParseTree(lastCommit.TreeHash)
		if parseErr != nil {
			return false, parseErr
		}
		headTree = tree
	}

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
		indexEntry, inIndex := index[path]

		var indexHash string
		if inIndex {
			indexHash = hex.EncodeToString(indexEntry.Hash[:])
		}

		if (inIndex && !inHead) || (!inIndex && inHead) || (inIndex && inHead && headHash != indexHash) {
			return true, nil
		}
	}

	// Check for unstaged changes (Working Directory vs. Index)
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		cleanPath := filepath.Clean(path)

		if info.IsDir() || strings.HasPrefix(cleanPath, RepoDir+string(os.PathSeparator)) || cleanPath == RepoDir {
			return nil
		}

		indexEntry, isTracked := index[cleanPath]
		if !isTracked {
			return fmt.Errorf("untracked")
		}

		currentHash, hashErr := storage.HashFile(cleanPath)
		if hashErr != nil {
			return hashErr
		}

		indexHashHex := hex.EncodeToString(indexEntry.Hash[:])
		if currentHash != indexHashHex {
			return fmt.Errorf("modified")
		}
		return nil
	})

	if err != nil {
		if err.Error() == "untracked" || err.Error() == "modified" {
			return true, nil
		}
		return false, err
	}

	return false, nil
}

// UpdateBranchPointer updates HEAD or branch ref
func UpdateBranchPointer(commitHash string) error {
	headData, err := os.ReadFile(HeadPath)
	if err != nil {
		return fmt.Errorf("unable to read HEAD file: %w", err)
	}
	ref := strings.TrimSpace(string(headData))

	if strings.HasPrefix(ref, "ref: ") {
		refPath := strings.TrimPrefix(ref, "ref: ")
		branchFile := filepath.Join(".kitcat", refPath)
		if err := SafeWrite(branchFile, []byte(commitHash), 0644); err != nil {
			return fmt.Errorf("failed to update branch pointer: %w", err)
		}
		return nil
	}

	if err := SafeWrite(HeadPath, []byte(commitHash), 0644); err != nil {
		return fmt.Errorf("failed to update HEAD: %w", err)
	}
	return nil
}

func readHead() (string, error) {
	headData, err := os.ReadFile(HeadPath)
	if err != nil {
		return "", err
	}
	ref := strings.TrimSpace(string(headData))

	if strings.HasPrefix(ref, "ref: ") {
		refPath := strings.TrimPrefix(ref, "ref: ")
		branchFile := filepath.Join(".kitcat", refPath)
		commitHash, err := os.ReadFile(branchFile)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(commitHash)), nil
	}
	return ref, nil
}

func IsSafePath(path string) bool {
	cleanPath := filepath.Clean(path)
	if filepath.IsAbs(cleanPath) {
		return false
	}
	if strings.Contains(cleanPath, "..") {
		return false
	}
	return true
}

func IsRepoInitialized() bool {
	cwd, err := os.Getwd()
	if err != nil { return false }
	for {
		if _, err := os.Stat(filepath.Join(cwd, RepoDir)); err == nil {
			if err := os.Chdir(cwd); err != nil {
				return false
			}
			return true
		}
		parent := filepath.Dir(cwd)
		if parent == cwd { return false }
		cwd = parent
	}
}

func SafeWrite(filename string, data []byte, perm os.FileMode) error {
	dirPath := filepath.Dir(filename)
	f, err := os.CreateTemp(dirPath, "atomic-")
	if err != nil { return err }
	tmpName := f.Name()
	defer os.Remove(tmpName)

	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return os.Rename(tmpName, filename)
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
