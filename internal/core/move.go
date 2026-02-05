package core

import (
	"errors"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

func MoveFile(oldPath, newPath string, force bool) error {
	if oldPath == newPath {
		return errors.New("source and destination paths are the same")
	}
	if !IsSafePath(oldPath) || !IsSafePath(newPath) {
		return errors.New("unsafe path detected")
	}

	// If force is true, overwrites destination
	// If not returns error if destination path already exists
	if force {
		if err := os.RemoveAll(newPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	} else {
		if _, err := os.Stat(newPath); err == nil {
			return errors.New("destination path already exists")
		} else if !os.IsNotExist(err) {
			return err
		}
	}

	// Check for uncommitted changes unless force is true
	if !force {
		idx, err := storage.LoadIndex()
		if err != nil {
			return err
		}

		// Only check if file is tracked in the index
		if indexHash, exists := idx[oldPath]; exists {
			currentHash, err := storage.HashFile(oldPath)
			if err != nil {
				return err
			}

			if currentHash != indexHash {
				return errors.New("local changes present, use -f to force")
			}
		}
	}

	// Rename file
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}

	// Stage new file
	if err := AddFile(newPath); err != nil {
		return err
	}

	// Load index
	idx, err := storage.LoadIndex()
	if err != nil {
		return err
	}

	// Remove old file from index
	delete(idx, oldPath)

	// Write index
	if err := storage.WriteIndex(idx); err != nil {
		return err
	}

	return nil
}
