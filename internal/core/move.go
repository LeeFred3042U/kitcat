package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// MoveFile moves or renames a file or directory and updates the index.
// Behaviour mirrors `git mv`:
//
// - Moves files or entire directories.
// - Updates all matching index entries.
// - Falls back to copy+delete if rename crosses filesystem boundaries.
func MoveFile(src, dst string, force bool) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	if src == dst {
		return fmt.Errorf("source and destination are the same")
	}
	
	if !IsSafePath(src) || !IsSafePath(dst) {
		return fmt.Errorf("unsafe path")
	}

	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("source '%s' does not exist", src)
	}

	if !force {
		if _, err := os.Stat(dst); err == nil {
			return fmt.Errorf("destination '%s' exists (use -f to force)", dst)
		}
	}

	//  Filesystem move
	if err := os.Rename(src, dst); err != nil {
		// Cross-device rename fallback (git does this internally)
		if err := copyRecursive(src, dst); err != nil {
			return fmt.Errorf("failed to copy during move: %w", err)
		}
		if err := os.RemoveAll(src); err != nil {
			return fmt.Errorf("failed to remove source after copy: %w", err)
		}
	}

	//  Update Index
	return storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {
		// File move
		if !info.IsDir() {
			if entry, ok := index[src]; ok {
				delete(index, src)
				entry.Path = dst
				index[dst] = entry
			}
			return nil
		}

		// Directory move:
		// Rewrite ALL entries under src/ to dst/
		prefix := src + string(os.PathSeparator)

		for path, entry := range index {
			if path == src || strings.HasPrefix(path, prefix) {

				// Compute new path preserving relative layout
				newPath := strings.Replace(path, src, dst, 1)

				delete(index, path)
				entry.Path = newPath
				index[newPath] = entry
			}
		}

		return nil
	})
}
