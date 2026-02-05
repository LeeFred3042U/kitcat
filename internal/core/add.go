package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

func AddFile(path string) error {
	path = filepath.Clean(path)
	if !IsSafePath(path) {
		return fmt.Errorf("unsafe path detected: %s", path)
	}
	// Guard: ensure we're inside a kitcat repo
	if _, err := os.Stat(RepoDir); os.IsNotExist(err) {
		return errors.New("not a kitcat repository (run `kitcat init`)")
	}

	hash, err := storage.HashAndStoreFile(path)
	if err != nil {
		return err
	}

	// Use UpdateIndex to safely update the index transactionally
	return storage.UpdateIndex(func(index map[string]string) error {
		// Skip if already tracked with same hash
		if existing, ok := index[path]; ok && existing == hash {
			return nil
		}

		index[path] = hash
		return nil
	})
}

// AddAll stages all changes in the working directory.
// This includes new files, modified files, and deleted files.
func AddAll() error {
	// Use UpdateIndex to safely update the index transactionally.
	// We hold the lock during the entire walk to ensure consistency.
	return storage.UpdateIndex(func(index map[string]string) error {
		// Load ignore patterns
		ignorePatterns, err := LoadIgnorePatterns()
		if err != nil {
			return err
		}

		// We need a way to track which files we see in the working directory
		// A map is used for this, giving us O(1) average time complexity for lookups
		filesInWorkDir := make(map[string]bool)

		// Walk the entire directory tree, starting from the current location "."
		err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Clean the path to use consistent separators.
			cleanPath := filepath.Clean(path)

			if !IsSafePath(cleanPath) {
				fmt.Printf("warning: skipping unsafe path: %s\n", cleanPath)
				return nil // Just skip unsafe paths found during a walk
			}

			// IMPORTANT: Skip the .kitcat directory entirely to avoid tracking our own database files.
			if strings.HasPrefix(cleanPath, RepoDir+string(os.PathSeparator)) || cleanPath == RepoDir {
				if info.IsDir() {
					return filepath.SkipDir // This is an efficient way to stop descending into a directory.
				}
				return nil
			}

			// We only care about files, not directories
			if info.IsDir() {
				return nil
			}

			// Check if file should be ignored (but only if not already tracked)
			if ShouldIgnore(cleanPath, ignorePatterns, index) {
				return nil // Skip this file
			}

			// Mark this file as "seen" in the working directory
			filesInWorkDir[cleanPath] = true

			// Hash the file and add/update it in the index.
			// This is the same logic as AddFile, but applied to every file we find
			hash, err := storage.HashAndStoreFile(cleanPath)
			if err != nil {
				// Continue even if one file fails.
				fmt.Printf("warning: could not add file %s: %v\n", cleanPath, err)
				return nil
			}
			index[cleanPath] = hash
			return nil
		})
		if err != nil {
			return err
		}

		// Find and handle deleted files.
		// We loop through the original index. If a file from the index was NOT seen
		// during our walk of the working directory, it must have been deleted
		for pathInIndex := range index {
			if !filesInWorkDir[pathInIndex] {
				// Remove the deleted file from our index map
				delete(index, pathInIndex)
			}
		}

		return nil // Commit changes
	})
}
