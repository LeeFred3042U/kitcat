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
func Checkout(target string) error {
	hash := strings.TrimSpace(target)

	// If target is a branch, resolve commit hash and update HEAD as symbolic ref.
	if b, err := os.ReadFile(".kitcat/refs/heads/" + target); err == nil {
		hash = strings.TrimSpace(string(b))
		if err := os.WriteFile(".kitcat/HEAD", []byte("ref: refs/heads/"+target), 0644); err != nil {
			return err
		}
	} else {
		// Detached HEAD: write raw commit hash.
		if err := os.WriteFile(".kitcat/HEAD", []byte(hash), 0644); err != nil {
			return err
		}
	}

	commit, err := storage.FindCommit(hash)
	if err != nil { return err }

	tree, err := storage.ParseTree(commit.TreeHash)
	if err != nil { return err }

	// Rewrite working tree files and rebuild index entries.
	return storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {
		for path, blobHash := range tree {
			content, err := storage.ReadObject(blobHash)
			if err != nil { return err }

			// Ensure directory exists before writing file.
			if dir := filepath.Dir(path); dir != "." {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", dir, err)
				}
			}

			// Overwrite file content unconditionally.
			if err := os.WriteFile(path, content, 0644); err != nil { return err }

			// Convert hex blob hash into binary index hash.
			hb, _ := storage.HexToHash(blobHash)
			index[path] = plumbing.IndexEntry{
				Path: path,
				Hash: hb,
				Mode: 0100644,
			}
		}
		return nil
	})
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

	blobHash, ok := tree[filePath]
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
	content, err := storage.ReadObject(blobHash)
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
