package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

func AddFile(inputPath string) error {
	if _, err := os.Stat(RepoDir); os.IsNotExist(err) {
		return errors.New("not a kitcat repository (run `kitcat init`)")
	}

	absInputPath, err := filepath.Abs(inputPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	absRepoRoot, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("failed to resolve repo root: %w", err)
	}

	if _, err := os.Stat(absInputPath); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", inputPath)
	}

	return storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {
		ignorePatterns, err := LoadIgnorePatterns()
		if err != nil {
			return err
		}

		proxyIndex := make(map[string]string, len(index))
		for k := range index {
			proxyIndex[k] = ""
		}

		return filepath.Walk(absInputPath, func(fullPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relPath, err := filepath.Rel(absRepoRoot, fullPath)
			if err != nil {
				return fmt.Errorf("file %s is outside repository", fullPath)
			}
			cleanPath := filepath.Clean(relPath)

			if cleanPath == "." || strings.HasPrefix(cleanPath, RepoDir) {
				if info.IsDir() && cleanPath != "." {
					return filepath.SkipDir
				}
				return nil
			}

			if info.IsDir() {
				return nil
			}

			if !IsSafePath(cleanPath) {
				return nil
			}
			if ShouldIgnore(cleanPath, ignorePatterns, proxyIndex) {
				return nil
			}

			entry := plumbing.IndexEntry{
				Path:      cleanPath,
				Mode:      0100644,
				Size:      uint32(info.Size()),
				MTimeSec:  uint32(info.ModTime().Unix()),
				MTimeNSec: uint32(info.ModTime().Nanosecond()),
			}

			// Optimization: Metadata Check
			if existing, exists := index[cleanPath]; exists {
				if existing.Size == entry.Size && existing.MTimeSec == entry.MTimeSec {
					return nil
				}
			}

			content, err := os.ReadFile(fullPath)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", fullPath, err)
			}

			hashStr, err := plumbing.HashAndWriteObject(content, "blob")
			if err != nil {
				return fmt.Errorf("failed to write blob for %s: %w", cleanPath, err)
			}

			hashBytes, _ := storage.HexToHash(hashStr)
			entry.Hash = hashBytes

			index[cleanPath] = entry
			return nil
		})
	})
}

func AddAll() error {
	return storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {
		ignorePatterns, err := LoadIgnorePatterns()
		if err != nil {
			return err
		}

		proxyIndex := make(map[string]string, len(index))
		for k := range index {
			proxyIndex[k] = ""
		}

		seen := make(map[string]bool, len(index))
		rootDir, _ := filepath.Abs(".")

		err = filepath.Walk(rootDir, func(fullPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relPath, _ := filepath.Rel(rootDir, fullPath)
			cleanPath := filepath.Clean(relPath)

			if cleanPath == "." || strings.HasPrefix(cleanPath, RepoDir) {
				if info.IsDir() && cleanPath != "." {
					return filepath.SkipDir
				}
				return nil
			}
			if info.IsDir() {
				return nil
			}
			if !IsSafePath(cleanPath) {
				return nil
			}
			if ShouldIgnore(cleanPath, ignorePatterns, proxyIndex) {
				return nil
			}

			seen[cleanPath] = true

			if existing, exists := index[cleanPath]; exists {
				if existing.Size == uint32(info.Size()) && existing.MTimeSec == uint32(info.ModTime().Unix()) {
					return nil
				}
			}

			content, err := os.ReadFile(fullPath)
			if err != nil {
				return err
			}
			hashStr, err := plumbing.HashAndWriteObject(content, "blob")
			if err != nil {
				return err
			}
			hashBytes, _ := storage.HexToHash(hashStr)

			entry := plumbing.IndexEntry{
				Path:      cleanPath,
				Hash:      hashBytes,
				Mode:      0100644,
				Size:      uint32(info.Size()),
				MTimeSec:  uint32(info.ModTime().Unix()),
				MTimeNSec: uint32(info.ModTime().Nanosecond()),
			}
			index[cleanPath] = entry
			return nil
		})
		if err != nil {
			return err
		}

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
