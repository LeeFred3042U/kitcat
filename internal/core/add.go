package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/repo"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// AddFile stages a file or directory.
func AddFile(inputPath string) error {
	// Resolve absolute path first so later Rel computations are stable
	// regardless of the caller’s working directory.
	absInputPath, err := filepath.Abs(inputPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	repoRoot, err := FindRepoRoot()
	if err != nil {
		return errors.New("not a kitcat repository (run `kitcat init`)")
	}

	info, err := os.Stat(absInputPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", inputPath)
	}
	if err != nil {
		return err
	}

	// Switch to repo root so all index paths remain repo-relative.
	originalWd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(repoRoot); err != nil {
		return fmt.Errorf("failed to switch to repo root: %w", err)
	}
	defer func() { _ = os.Chdir(originalWd) }()

	// Index mutation is wrapped in UpdateIndex to guarantee exclusive write access.
	return storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {
		ignorePatterns, err := LoadIgnorePatterns()
		if err != nil {
			return err
		}

		// Proxy index prevents tracked files from being ignored during matching.
		proxyIndex := make(map[string]string, len(index))
		for k := range index {
			proxyIndex[k] = ""
		}

		// Fast path avoids directory walking overhead when input is a single file.
		if !info.IsDir() {
			relPath, err := filepath.Rel(repoRoot, absInputPath)
			if err != nil {
				return fmt.Errorf("file %s is outside repository", absInputPath)
			}
			cleanPath := filepath.Clean(relPath)

			_, err = stageFile(absInputPath, cleanPath, info, index, ignorePatterns, proxyIndex)
			return err
		}

		// Directory walk stages files recursively while pruning ignored paths early.
		return filepath.Walk(absInputPath, func(fullPath string, fInfo os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relPath, relErr := filepath.Rel(repoRoot, fullPath)
			if relErr != nil {
				return relErr
			}
			cleanPath := filepath.Clean(relPath)

			// Skip ignored or internal directories to avoid unnecessary traversal.
			if fInfo.IsDir() {
				if fullPath == absInputPath {
					return nil
				}
				if shouldSkipDir(cleanPath, ignorePatterns, proxyIndex) {
					return filepath.SkipDir
				}
				return nil
			}

			_, stageErr := stageFile(fullPath, cleanPath, fInfo, index, ignorePatterns, proxyIndex)
			return stageErr
		})
	})
}

// AddAll stages all files under the repository root.
func AddAll() error {
	repoRoot, err := FindRepoRoot()
	if err != nil {
		return errors.New("not a kitcat repository (run `kitcat init`)")
	}

	// Ensure index paths remain relative by executing from repo root.
	originalWd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(repoRoot); err != nil {
		return fmt.Errorf("failed to switch to repo root: %w", err)
	}
	defer func() { _ = os.Chdir(originalWd) }()

	return storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {
		ignorePatterns, err := LoadIgnorePatterns()
		if err != nil {
			return err
		}

		proxyIndex := make(map[string]string, len(index))
		for k := range index {
			proxyIndex[k] = ""
		}

		// Track files encountered during walk so removed files can be pruned.
		seen := make(map[string]bool, len(index))

		err = filepath.Walk(repoRoot, func(fullPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relPath, relErr := filepath.Rel(repoRoot, fullPath)
			if relErr != nil {
				return relErr
			}
			cleanPath := filepath.Clean(relPath)

			// Early directory pruning reduces IO and avoids entering metadata dirs.
			if info.IsDir() {
				if fullPath == repoRoot {
					return nil
				}
				if shouldSkipDir(cleanPath, ignorePatterns, proxyIndex) {
					return filepath.SkipDir
				}
				return nil
			}

			tracked, stageErr := stageFile(fullPath, cleanPath, info, index, ignorePatterns, proxyIndex)
			if stageErr != nil {
				return stageErr
			}

			if tracked {
				seen[cleanPath] = true
			}
			return nil
		})
		if err != nil {
			return err
		}

		// Remove index entries not encountered during the scan to reflect deletions.
		var toDelete []string
		for path := range index {
			if !seen[path] {
				toDelete = append(toDelete, path)
			}
		}
		for _, path := range toDelete {
			delete(index, path)
		}

		return nil
	})
}

// stageFile encapsulates the staging logic for a single file.
// Accepts pre-calculated cleanPath to avoid redundant Rel/Clean calls.
// Returns (tracked bool, error).
func stageFile(fullPath, cleanPath string, info os.FileInfo,
	index map[string]plumbing.IndexEntry,
	ignorePatterns []IgnorePattern,
	proxyIndex map[string]string) (bool, error) {

	if info.IsDir() {
		return false, nil
	}

	// Explicit guard prevents repository metadata from entering the index.
	if isInternalDir(cleanPath) {
		return false, nil
	}

	// Safety checks prevent path traversal or ignored files from being staged.
	if !IsSafePath(cleanPath) {
		return false, nil
	}
	if ShouldIgnore(cleanPath, ignorePatterns, proxyIndex) {
		return false, nil
	}

	entry := plumbing.IndexEntry{
		Path:      cleanPath,
		Mode:      0100644,
		Size:      uint32(info.Size()),
		MTimeSec:  uint32(info.ModTime().Unix()),
		MTimeNSec: uint32(info.ModTime().Nanosecond()),
	}

	// If stat cache matches existing entry, skip hashing to avoid redundant IO.
	if existing, exists := index[cleanPath]; exists {
		if existing.Size == entry.Size &&
			existing.MTimeSec == entry.MTimeSec &&
			existing.MTimeNSec == entry.MTimeNSec {
			return true, nil
		}
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return false, fmt.Errorf("failed to read %s: %w", fullPath, err)
	}

	hashStr, err := plumbing.HashAndWriteObject(content, "blob")
	if err != nil {
		return false, fmt.Errorf("failed to write blob for %s: %w", cleanPath, err)
	}

	hashBytes, _ := storage.HexToHash(hashStr)
	entry.Hash = hashBytes

	index[cleanPath] = entry
	return true, nil
}

// shouldSkipDir determines if a directory should be skipped during walking.
func shouldSkipDir(cleanPath string, patterns []IgnorePattern, proxyIndex map[string]string) bool {
	// Internal metadata directory is always excluded from traversal.
	if isInternalDir(cleanPath) {
		return true
	}

	// Delegates ignore decision to pattern matcher to keep walk logic minimal.
	return ShouldIgnore(cleanPath, patterns, proxyIndex)
}

// isInternalDir checks if a path belongs to the kitcat metadata directory.
// It effectively blocks ".kitcat" and ".kitcat/..."
func isInternalDir(path string) bool {
	if path == repo.Dir {
		return true
	}
	// Separator-aware prefix check avoids false positives like ".kitcat_backup".
	prefix := repo.Dir + string(os.PathSeparator)
	return strings.HasPrefix(path, prefix)
}
