package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/repo"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Clean removes untracked files and directories from the working directory.
func Clean(dryRun, removeDirs, removeIgnored, onlyIgnored bool) error {
	index, err := storage.LoadIndex()
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	patterns, err := LoadIgnorePatterns()
	if err != nil {
		return fmt.Errorf("failed to load ignore patterns: %w", err)
	}

	proxyIndex := make(map[string]string, len(index))
	for k := range index {
		proxyIndex[k] = ""
	}

	var toRemove []string

	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || path == "." {
			return nil
		}

		cleanPath := filepath.Clean(path)

		// Never touch the repository metadata
		if cleanPath == repo.Dir || strings.HasPrefix(cleanPath, repo.Dir+string(os.PathSeparator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		isIgnored := ShouldIgnore(cleanPath, patterns, proxyIndex)

		// Determine if we should process this file based on ignore flags
		if isIgnored {
			if !removeIgnored && !onlyIgnored {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		} else {
			if onlyIgnored {
				return nil
			}
		}

		if info.IsDir() {
			if !removeDirs {
				return nil // Don't remove, but keep walking inside
			}

			// Check if any tracked files exist inside this directory
			prefix := cleanPath + "/"
			hasTrackedFiles := false
			for trackedPath := range index {
				if strings.HasPrefix(trackedPath, prefix) {
					hasTrackedFiles = true
					break
				}
			}

			if hasTrackedFiles {
				return nil // It's a tracked directory, keep walking
			}

			// Untracked directory found!
			if IsSafePath(cleanPath) {
				toRemove = append(toRemove, cleanPath)
			}
			return filepath.SkipDir // Skip walking children, we'll delete the whole dir
		}

		// It's a file. Skip if tracked.
		if _, tracked := index[cleanPath]; tracked {
			return nil
		}

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
			continue
		}

		if err := os.RemoveAll(file); err != nil {
			fmt.Printf("warning: failed to remove %s: %v\n", file, err)
		} else {
			fmt.Printf("Removing %s\n", file)
		}
	}

	return nil
}
