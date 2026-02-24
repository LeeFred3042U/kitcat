package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// RemoveFile deletes a path from the working directory and removes
// the corresponding entry from the index. Directory removal requires
// the recursive flag. If cached is true, the file is only removed
// from the index, leaving the working directory intact.
func RemoveFile(path string, recursive, cached bool) error {
	if !recursive {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			return fmt.Errorf("cannot remove '%s': Is a directory", path)
		}
	}

	// If not just cached, physically remove the file/directory from disk
	if !cached {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}

	return storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {
		// Ensure descendant paths are removed by matching the directory prefix
		prefix := path + "/"
		for k := range index {
			if k == path || strings.HasPrefix(k, prefix) {
				delete(index, k)
			}
		}
		return nil
	})
}
