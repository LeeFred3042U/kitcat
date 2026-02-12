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

func Checkout(target string) error {
	hash := strings.TrimSpace(target)
	
	if b, err := os.ReadFile(".kitcat/refs/heads/" + target); err == nil {
		hash = strings.TrimSpace(string(b))
		if err := os.WriteFile(".kitcat/HEAD", []byte("ref: refs/heads/"+target), 0644); err != nil {
			return err
		}
	} else {
		if err := os.WriteFile(".kitcat/HEAD", []byte(hash), 0644); err != nil {
			return err
		}
	}

	commit, err := storage.FindCommit(hash)
	if err != nil { return err }
	
	tree, err := storage.ParseTree(commit.TreeHash)
	if err != nil { return err }

	return storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {
		for path, blobHash := range tree {
			content, err := storage.ReadObject(blobHash)
			if err != nil { return err }
			
			if dir := filepath.Dir(path); dir != "." {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", dir, err)
				}
			}

			if err := os.WriteFile(path, content, 0644); err != nil { return err }
			
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

// Restore a file in the working directory to its state in the last commit
func CheckoutFile(filePath string) error {
	// Get the target content (from HEAD/Last Commit)
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

	// SAFETY CHECK: Prevent overwriting dirty or untracked files
	if _, err := os.Stat(filePath); err == nil {
		// File exists, check if it is safe to overwrite
		currentHash, err := calculateHash(filePath)
		if err != nil {
			return fmt.Errorf("failed to calculate hash for safety check: %v", err)
		}

		// Load index to check if the file is tracked and clean
		index, err := storage.LoadIndex()
		if err != nil {
			return err
		}

		if trackedHash, ok := index[filePath]; ok {
			// Compare string (currentHash) with [20]byte (trackedHash.Hash)
			trackedHashHex := hex.EncodeToString(trackedHash.Hash[:])
			if currentHash != trackedHashHex {
				return fmt.Errorf("local changes would be overwritten")
			}
		} else {
			// File exists but is NOT in the index (untracked): fail to prevent data loss
			return fmt.Errorf("error: untracked file '%s' would be overwritten", filePath)
		}
	}

	// Safe to overwrite: Perform the checkout
	content, err := storage.ReadObject(blobHash)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, content, 0644)
}

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
