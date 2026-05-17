package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/LeeFred3042U/kitcat/internal/hashutil"
	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/repo"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

func AddFile(inputPath string) error {
	absInputPath, err := filepath.Abs(inputPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	repoRoot, err := FindRepoRoot()
	if err != nil {
		return errors.New("not a kitcat repository (run `kitcat init`)")
	}

	info, err := os.Lstat(absInputPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", inputPath)
	}
	if err != nil {
		return err
	}

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

		if !info.IsDir() {
			relPath, err := filepath.Rel(repoRoot, absInputPath)
			if err != nil {
				return fmt.Errorf("file %s is outside repository", absInputPath)
			}
			cleanPath := filepath.Clean(relPath)

			_, err = stageFile(absInputPath, cleanPath, info, index, ignorePatterns, proxyIndex)
			return err
		}

		return filepath.Walk(absInputPath, func(fullPath string, fInfo os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relPath, relErr := filepath.Rel(repoRoot, fullPath)
			if relErr != nil {
				return relErr
			}
			cleanPath := filepath.Clean(relPath)

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

func AddAll() error {
	repoRoot, err := FindRepoRoot()
	if err != nil {
		return errors.New("not a kitcat repository (run `kitcat init`)")
	}

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

		type fileWork struct {
			fullPath  string
			cleanPath string
			info      os.FileInfo
		}

		var files []fileWork
		var walkErr error

		walkErr = filepath.Walk(repoRoot, func(fullPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			relPath, err := filepath.Rel(repoRoot, fullPath)
			if err != nil {
				return err
			}
			cleanPath := filepath.Clean(relPath)

			if info.IsDir() {
				if fullPath == repoRoot {
					return nil
				}
				if shouldSkipDir(cleanPath, ignorePatterns, proxyIndex) {
					return filepath.SkipDir
				}
				return nil
			}

			if isInternalDir(cleanPath) || !IsSafePath(cleanPath) {
				return nil
			}
			if ShouldIgnore(cleanPath, ignorePatterns, proxyIndex) {
				return nil
			}

			files = append(files, fileWork{
				fullPath:  fullPath,
				cleanPath: cleanPath,
				info:      info,
			})
			return nil
		})
		if walkErr != nil {
			return walkErr
		}

		type stageResult struct {
			cleanPath string
			entry     plumbing.IndexEntry
			tracked   bool
			err       error
		}

		numWorkers := runtime.NumCPU()
		if numWorkers > 8 {
			numWorkers = 8
		}

		workCh := make(chan fileWork, numWorkers*2)
		resultCh := make(chan stageResult, numWorkers*2)

		var wg sync.WaitGroup
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for work := range workCh {
					tracked, entry, err := stageFileWorker(
						work.fullPath,
						work.cleanPath,
						work.info,
						index,
						ignorePatterns,
						proxyIndex,
					)
					resultCh <- stageResult{
						cleanPath: work.cleanPath,
						entry:     entry,
						tracked:   tracked,
						err:       err,
					}
				}
			}()
		}

		go func() {
			wg.Wait()
			close(resultCh)
		}()

		go func() {
			for _, f := range files {
				workCh <- f
			}
			close(workCh)
		}()

		seen := make(map[string]bool, len(files))
		var firstErr error

		for result := range resultCh {
			if result.err != nil {
				if firstErr == nil {
					firstErr = result.err
				}
				continue
			}
			if result.tracked {
				index[result.cleanPath] = result.entry
				seen[result.cleanPath] = true
			}
		}

		if firstErr != nil {
			return firstErr
		}

		for path := range index {
			if !seen[path] {
				delete(index, path)
			}
		}

		return nil
	})
}

func stageFileWorker(
	fullPath, cleanPath string,
	info os.FileInfo,
	index map[string]plumbing.IndexEntry, // read-only
	ignorePatterns []IgnorePattern,
	proxyIndex map[string]string,
) (tracked bool, entry plumbing.IndexEntry, err error) {
	if info.IsDir() {
		return false, plumbing.IndexEntry{}, nil
	}
	if isInternalDir(cleanPath) || !IsSafePath(cleanPath) {
		return false, plumbing.IndexEntry{}, nil
	}
	if ShouldIgnore(cleanPath, ignorePatterns, proxyIndex) {
		return false, plumbing.IndexEntry{}, nil
	}

	isSymlink := info.Mode()&os.ModeSymlink != 0
	var fileMode uint32
	switch {
	case isSymlink:
		fileMode = 0o120000
	case info.Mode()&0o111 != 0:
		fileMode = 0o100755
	default:
		fileMode = 0o100644
	}

	candidate := plumbing.IndexEntry{
		Path:      cleanPath,
		Mode:      fileMode,
		Size:      uint32(info.Size()),
		MTimeSec:  uint32(info.ModTime().Unix()),
		MTimeNSec: uint32(info.ModTime().Nanosecond()),
	}

	if existing, exists := index[cleanPath]; exists {
		if existing.Size == candidate.Size &&
			existing.MTimeSec == candidate.MTimeSec &&
			existing.MTimeNSec == candidate.MTimeNSec {
			return true, existing, nil
		}
	}

	var content []byte
	if isSymlink {
		target, err := os.Readlink(fullPath)
		if err != nil {
			return false, plumbing.IndexEntry{}, fmt.Errorf("readlink %s: %w", fullPath, err)
		}
		content = []byte(target)
	} else {
		content, err = os.ReadFile(fullPath)
		if err != nil {
			return false, plumbing.IndexEntry{}, fmt.Errorf("read %s: %w", fullPath, err)
		}
	}

	hashStr, err := plumbing.HashAndWriteObject(content, "blob")
	if err != nil {
		return false, plumbing.IndexEntry{}, fmt.Errorf("hash %s: %w", cleanPath, err)
	}

	hashBytes, err := hashutil.DecodeHex(hashStr)
	if err != nil {
		return false, plumbing.IndexEntry{}, err
	}
	candidate.Hash = hashBytes

	return true, candidate, nil
}

func stageFile(fullPath, cleanPath string, info os.FileInfo,
	index map[string]plumbing.IndexEntry,
	ignorePatterns []IgnorePattern,
	proxyIndex map[string]string,
) (bool, error) {
	if info.IsDir() {
		return false, nil
	}

	// Prevent repository metadata from entering the index.
	if isInternalDir(cleanPath) {
		return false, nil
	}

	// Ensure path safety and ignore rules.
	if !IsSafePath(cleanPath) {
		return false, nil
	}
	if ShouldIgnore(cleanPath, ignorePatterns, proxyIndex) {
		return false, nil
	}

	isSymlink := info.Mode()&os.ModeSymlink != 0
	var fileMode uint32
	switch {
	case isSymlink:
		fileMode = 0o120000
	case info.Mode()&0o111 != 0:
		fileMode = 0o100755
	default:
		fileMode = 0o100644
	}

	entry := plumbing.IndexEntry{
		Path:      cleanPath,
		Mode:      fileMode,
		Size:      uint32(info.Size()),
		MTimeSec:  uint32(info.ModTime().Unix()),
		MTimeNSec: uint32(info.ModTime().Nanosecond()),
	}

	if existing, exists := index[cleanPath]; exists {
		if existing.Size == entry.Size &&
			existing.MTimeSec == entry.MTimeSec &&
			existing.MTimeNSec == entry.MTimeNSec {
			return true, nil
		}
	}

	// Read content based on file type.
	// For symlinks, content is the link target path stored as UTF-8 bytes.
	// For regular files, content is the file bytes.
	var content []byte
	if isSymlink {
		target, err := os.Readlink(fullPath)
		if err != nil {
			return false, fmt.Errorf("failed to read symlink %s: %w", fullPath, err)
		}
		content = []byte(target)
	} else {
		var err error
		content, err = os.ReadFile(fullPath)
		if err != nil {
			return false, fmt.Errorf("failed to read %s: %w", fullPath, err)
		}
	}

	hashStr, err := plumbing.HashAndWriteObject(content, "blob")
	if err != nil {
		return false, fmt.Errorf("failed to write blob for %s: %w", cleanPath, err)
	}

	hashBytes, _ := hashutil.DecodeHex(hashStr)
	entry.Hash = hashBytes

	index[cleanPath] = entry
	return true, nil
}

// shouldSkipDir determines whether a directory should be excluded during
// recursive repository scans.
func shouldSkipDir(cleanPath string, patterns []IgnorePattern, proxyIndex map[string]string) bool {
	// Internal metadata directory is always excluded from traversal.
	if isInternalDir(cleanPath) {
		return true
	}

	// Delegate ignore logic to pattern matcher.
	return ShouldIgnore(cleanPath, patterns, proxyIndex)
}

// isInternalDir reports whether the given path belongs to the repository's
// internal metadata directory (".kitcat").
func isInternalDir(path string) bool {
	if path == repo.Dir {
		return true
	}

	// Separator-aware prefix check avoids false positives.
	prefix := repo.Dir + string(os.PathSeparator)
	return strings.HasPrefix(path, prefix)
}
