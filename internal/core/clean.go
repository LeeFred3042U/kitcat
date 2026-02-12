package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Clean removes untracked files from the working directory.
// If dryRun is true, it only lists files that would be removed.
func Clean(dryRun bool) error {
	// Load the index to check which files are tracked
	index, err := storage.LoadIndex()
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	// Load ignore patterns to preserve ignored files
	patterns, err := LoadIgnorePatterns()
	if err != nil {
		return fmt.Errorf("failed to load ignore patterns: %w", err)
	}

	// Build proxy map for ShouldIgnore compatibility
	proxyIndex := make(map[string]string, len(index))
	for k := range index {
		proxyIndex[k] = ""
	}

	var toRemove []string

	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the .kitcat directory
		if path == RepoDir || strings.HasPrefix(path, RepoDir+string(os.PathSeparator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// We only clean files, not directories (unless empty, but git clean usually cleans files)
		if info.IsDir() {
			return nil
		}

		// Clean path
		cleanPath := filepath.Clean(path)

		// 1. Skip if tracked
		if _, tracked := index[cleanPath]; tracked {
			return nil
		}

		// 2. Skip if ignored
		if ShouldIgnore(cleanPath, patterns, proxyIndex) {
			return nil
		}

		// If untracked and not ignored, verify safety and mark for removal
		if IsSafePath(cleanPath) {
			toRemove = append(toRemove, cleanPath)
		}

		return nil
	})

	if err != nil {
		return err
	}

	if len(toRemove) == 0 {
		if dryRun {
			fmt.Println("No files to clean.")
		}
		return nil
	}

	for _, file := range toRemove {
		if dryRun {
			fmt.Printf("Would remove %s\n", file)
		} else {
			if err := os.Remove(file); err != nil {
				fmt.Printf("warning: failed to remove %s: %v\n", file, err)
			} else {
				fmt.Printf("Removing %s\n", file)
			}
		}
	}

	return nil
}
