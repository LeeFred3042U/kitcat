package core

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Checkout switches HEAD to a branch or detached commit and restores
// the working tree to match the target commit. Index entries are rebuilt
// inside a transactional storage.UpdateIndex call.
func Checkout(target string, force bool) error {
	// 1. PREFLIGHT: Abort if worktree is dirty unless forced.
	if !force {
		dirty, err := IsWorkDirDirty()
		if err != nil {
			return fmt.Errorf("failed to check working directory status: %w", err)
		}
		if dirty {
			return fmt.Errorf("error: your local changes would be overwritten by checkout.\nPlease commit your changes or stash them before you switch branches.\nAborting")
		}
	}

	hash := strings.TrimSpace(target)
	oldHeadHash, _ := ResolveHead() // Capture the current HEAD before switching

	// 2. RESOLVE TARGET
	var headContent string
	if b, err := os.ReadFile(".kitcat/refs/heads/" + target); err == nil {
		hash = strings.TrimSpace(string(b))
		headContent = "ref: refs/heads/" + target
	} else {
		headContent = hash
	}

	commit, err := storage.FindCommit(hash)
	if err != nil {
		return fmt.Errorf("pathspec '%s' did not match any file(s) known to kitcat", target)
	}

	tree, err := storage.ParseTree(commit.TreeHash)
	if err != nil {
		return err
	}

	// 2.5 UNTRACKED COLLISION CHECK (The 1.3 Fix)
	// Prevent checkout if it would overwrite an untracked local file.
	if !force {
		index, err := storage.LoadIndex()
		if err != nil {
			return err
		}
		var collisions []string
		for path := range tree {
			if _, inIndex := index[path]; !inIndex {
				// The target tree has this file, but we don't track it.
				// If it exists on disk, it's an untracked collision!
				if _, err := os.Stat(path); err == nil {
					collisions = append(collisions, path)
				}
			}
		}
		if len(collisions) > 0 {
			return fmt.Errorf("error: The following untracked working tree files would be overwritten by checkout:\n  %s\nPlease move or remove them before you switch branches.\nAborting", strings.Join(collisions, "\n  "))
		}
	}

	// 3. WORKSPACE & INDEX REWRITE
	err = storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {
		// Safe delete: remove files present in current index but absent in target tree
		for path := range index {
			if _, exists := tree[path]; !exists {
				os.Remove(path)     // Remove from disk
				delete(index, path) // Remove from index
			}
		}

		// Restore/Overwrite files from target tree
		for path, entry := range tree {
			content, err := storage.ReadObject(entry.Hash)
			if err != nil {
				return err
			}

			// Ensure directory exists before writing file.
			if dir := filepath.Dir(path); dir != "." {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", dir, err)
				}
			}

			if err := os.WriteFile(path, content, 0644); err != nil {
				return err
			}

			// Convert hex blob hash into binary index hash.
			hb, _ := storage.HexToHash(entry.Hash)

			var mode uint32
			if _, err := fmt.Sscanf(entry.Mode, "%o", &mode); err != nil {
				mode = 0100644
			}

			index[path] = plumbing.IndexEntry{
				Path: path,
				Hash: hb,
				Mode: mode,
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	// 4. INVARIANT: HEAD moves last!
	if err := SafeWrite(".kitcat/HEAD", []byte(headContent), 0644); err != nil {
		return err
	}

	// 5. REFLOG: Write history of the movement
	return ReflogAppend("HEAD", oldHeadHash, hash, "checkout: moving to "+target)
}

// CheckoutFile restores a single file from the last commit into the working tree.
// Includes safety checks to avoid overwriting modified or untracked files.
func CheckoutFile(filePath string) error {
	// Resolve file content from HEAD commit.
	lastCommit, err := storage.GetLastCommit()
	if err != nil {
		return err
	}

	tree, err := storage.ParseTree(lastCommit.TreeHash)
	if err != nil {
		return err
	}

	entry, ok := tree[filePath]
	if !ok {
		return errors.New("file not found in the last commit")
	}

	// Safety check: refuse overwrite if file has local modifications or is untracked.
	if _, err := os.Stat(filePath); err == nil {
		currentHash, err := calculateHash(filePath)
		if err != nil {
			return fmt.Errorf("failed to calculate hash for safety check: %v", err)
		}

		index, err := storage.LoadIndex()
		if err != nil {
			return err
		}

		if trackedHash, ok := index[filePath]; ok {
			trackedHashHex := hex.EncodeToString(trackedHash.Hash[:])
			if currentHash != trackedHashHex {
				return fmt.Errorf("local changes would be overwritten")
			}
		} else {
			// Prevent destructive overwrite of untracked files.
			return fmt.Errorf("error: untracked file '%s' would be overwritten", filePath)
		}
	}

	// Restore file content from object storage.
	content, err := storage.ReadObject(entry.Hash)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, content, 0644)
}

// calculateHash computes a raw SHA-1 hash of file contents.
// Used only for safety comparison with tracked index hashes.
func calculateHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
