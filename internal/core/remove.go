package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

func RemoveFile(filename string, force bool) error {
	filename = filepath.Clean(filename)
	if !IsSafePath(filename) {
		return fmt.Errorf("unsafe path detected: %s", filename)
	}
	// Use UpdateIndex to safely update the index transactionally
	return storage.UpdateIndex(func(index map[string]string) error {
		// First, verify the file exists in the index
		indexHash, ok := index[filename]
		if !ok {
			return fmt.Errorf("pathspec '%s' did not match any files", filename)
		}

		// Check for uncommitted changes before deletion (unless force is true)
		if !force {
			diskHash, err := storage.HashFile(filename)
			if err != nil {
				// If file is missing from disk, skip the hash check and proceed
				// to remove it from the index since user clearly wants it gone
				if !os.IsNotExist(err) {
					return fmt.Errorf("failed to hash file: %w", err)
				}
			} else if diskHash != indexHash {
				return fmt.Errorf("local changes present")
			}
		}

		// Step 1: Delete file from disk FIRST
		if err := os.Remove(filename); err != nil {
			// If file doesn't exist, that's OK (already deleted)
			if !os.IsNotExist(err) {
				// Permission error or other failure - return immediately
				return err
			}
		}

		// Step 2: Remove from index
		delete(index, filename)
		return nil
	})
}
