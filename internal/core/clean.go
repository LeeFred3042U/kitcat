package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Clean removes untracked files from the working directory.
// When dryRun is true, files are only listed and not deleted.
func Clean(dryRun bool) error {
	// Load tracked files from index; used to prevent accidental deletion.
	index, err := storage.LoadIndex()
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	// Load ignore rules so ignored files are preserved.
	patterns, err := LoadIgnorePatterns()
	if err != nil {
		return fmt.Errorf("failed to load ignore patterns: %w", err)
	}

	// ShouldIgnore expects a map[string]string; build a lightweight proxy.
	proxyIndex := make(map[string]string, len(index))
	for k := range index {
		proxyIndex[k] = ""
	}

	var toRemove []string

	// Walk working directory to find candidates for cleanup.
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Never descend into repository metadata directory.
		if path == RepoDir || strings.HasPrefix(path, RepoDir+string(os.PathSeparator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Only consider files; directories are left intact.
		if info.IsDir() {
			return nil
		}

		cleanPath := filepath.Clean(path)

		// Skip tracked files.
		if _, tracked := index[cleanPath]; tracked {
			return nil
		}

		// Skip ignored files.
		if ShouldIgnore(cleanPath, patterns, proxyIndex) {
			return nil
		}

		// Final safety guard before scheduling deletion.
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

	// Perform deletion or dry-run output.
	for _, file := range toRemove {
		if dryRun {
			fmt.Printf("Would remove %s\n", file)
			continue
		}

		// Best-effort deletion; errors reported but do not abort entire operation.
		if err := os.Remove(file); err != nil {
			fmt.Printf("warning: failed to remove %s: %v\n", file, err)
		} else {
			fmt.Printf("Removing %s\n", file)
		}
	}

	return nil
}
