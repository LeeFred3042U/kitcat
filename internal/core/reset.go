package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/storage"
	"github.com/LeeFred3042U/kitcat/internal/plumbing"
)

const (
	ResetMixed = 0
	ResetSoft  = 1
	ResetHard  = 2
)

// Reset implements the porcelain Git reset behavior using separated plumbing layers.
func Reset(commitStr string, mode int) error {
	// 1. Resolve Reference
	if commitStr == "" || commitStr == "HEAD" {
		hash, err := ResolveHead()
		if err != nil {
			return fmt.Errorf("could not resolve HEAD: %w", err)
		}
		commitStr = hash
	} else {
		branchPath := filepath.Join(".kitcat", "refs", "heads", commitStr)
		if b, err := os.ReadFile(branchPath); err == nil {
			commitStr = strings.TrimSpace(string(b))
		}
	}

	commitObj, err := storage.FindCommit(commitStr)
	if err != nil {
		return fmt.Errorf("invalid commit %s: %w", commitStr, err)
	}

	// 2. Destructive Workspace Rewrite (checkout-index layer)
	if mode == ResetHard {
		if err := CheckoutTree(commitObj.TreeHash); err != nil {
			return fmt.Errorf("checkout failed: %w", err)
		}
	}

	// 3. Index Rewrite (read-tree layer)
	if mode == ResetMixed || mode == ResetHard {
		if err := ReadTree(commitObj.TreeHash); err != nil {
			return fmt.Errorf("read-tree failed: %w", err)
		}
	}

	// INVARIANT: HEAD is updated ONLY after the index and working tree
	// successfully reflect the target commit. Do not reorder this!
	actionMsg := fmt.Sprintf("reset: moving to %s", commitStr)
	if err := UpdateRef(commitObj.ID, actionMsg); err != nil {
		return fmt.Errorf("update-ref failed: %w", err)
	}

	return nil
}

// ReadTree reads a tree object into the index map and persists it.
func ReadTree(treeHash string) error {
	tree, err := storage.ParseTree(treeHash)
	if err != nil {
		return err
	}
	return storage.WriteIndexFromTree(tree)
}

// CheckoutTree compares the workspace vs the target tree and aligns them.
// NOTE: Workspace updates are currently NOT transactional. Partial file
// updates may remain on disk if an error occurs mid-operation.

func CheckoutTree(treeHash string) error {
	targetTree, err := storage.ParseTree(treeHash)
	if err != nil {
		return err
	}

	currentIndex, _ := storage.LoadIndex()

	for path := range currentIndex {
		if _, exists := targetTree[path]; !exists {
			os.Remove(path)
		}
	}

	for path, entry := range targetTree {
		content, err := storage.ReadObject(entry.Hash)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}

		var modeVal uint32
		fmt.Sscanf(entry.Mode, "%o", &modeVal)
		
		if modeVal == 0120000 {
			// Symlink: content is the target path. 
			os.Remove(path)
			if err := os.Symlink(string(content), path); err != nil {
				return err
			}
		} else {
			os.Remove(path)
			perm := os.FileMode(0o644)
			if modeVal&0o111 != 0 {
				perm = 0o755
			}
			if err := os.WriteFile(path, content, perm); err != nil {
				return err
			}
		}
	}
	return nil
}

// UpdateRef safely updates HEAD/branch pointers and creates reflog entries.
func UpdateRef(newCommit string, actionMsg string) error {
	headPath := filepath.Join(".kitcat", "HEAD")
	headData, err := os.ReadFile(headPath)
	if err != nil {
		return fmt.Errorf("unable to read HEAD: %w", err)
	}

	ref := strings.TrimSpace(string(headData))
	var oldCommit string
	var targetRefPath string

	if strings.HasPrefix(ref, "ref: ") {
		targetRefPath = strings.TrimPrefix(ref, "ref: ")
		branchFile := filepath.Join(".kitcat", targetRefPath)

		if b, err := os.ReadFile(branchFile); err == nil {
			oldCommit = strings.TrimSpace(string(b))
		}

		// Write to reflog BEFORE the atomic ref update for crash recovery
		_ = ReflogAppend(targetRefPath, oldCommit, newCommit, actionMsg)

		if err := SafeWrite(branchFile, []byte(newCommit), 0o644); err != nil {
			return fmt.Errorf("failed to update branch ref: %w", err)
		}

	} else {
		oldCommit = ref

		if err := SafeWrite(headPath, []byte(newCommit), 0o644); err != nil {
			return fmt.Errorf("failed to update detached HEAD: %w", err)
		}
	}

	// Always write HEAD reflog last
	_ = ReflogAppend("HEAD", oldCommit, newCommit, actionMsg)
	return nil
}

func UnstageFile(commitStr string, paths []string) error {
	if commitStr == "" || commitStr == "HEAD" {
		hash, err := ResolveHead()
		if err != nil {
			return fmt.Errorf("could not resolve HEAD: %w", err)
		}
		commitStr = hash
	} else {
		branchPath := filepath.Join(".kitcat", "refs", "heads", commitStr)
		if b, err := os.ReadFile(branchPath); err == nil {
			commitStr = strings.TrimSpace(string(b))
		}
	}

	commitObj, err := storage.FindCommit(commitStr)
	if err != nil {
		return fmt.Errorf("invalid commit %s: %w", commitStr, err)
	}

	targetTree, err := storage.ParseTree(commitObj.TreeHash)
	if err != nil {
		return err
	}

	return storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {
		for _, path := range paths {
			if entry, exists := targetTree[path]; exists {
				var mode uint32
				fmt.Sscanf(entry.Mode, "%o", &mode)

				hb, err := storage.HexToHash(entry.Hash)
				if err != nil {
					return err
				}

				index[path] = plumbing.IndexEntry{
					Path: path,
					Hash: hb,
					Mode: mode,
				}
			} else {
				delete(index, path)
			}
		}
		return nil
	})
}
