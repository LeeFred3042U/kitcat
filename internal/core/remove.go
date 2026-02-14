package core

import (
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// RemoveFile deletes a path from the working directory and removes
// the corresponding entry from the index. Directory removal requires
// the recursive flag to avoid accidental destructive operations.
func RemoveFile(path string, recursive bool) error {
	// Prevent directory deletion unless explicitly requested.
	if !recursive {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			return fmt.Errorf("cannot remove '%s': Is a directory", path)
		}
	}

	// RemoveAll is intentionally destructive and will delete nested contents.
	if err := os.RemoveAll(path); err != nil {
		return err
	}

	// Update index transactionally to keep staging area consistent with disk state.
	return storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {
		delete(index, path)
		return nil
	})
}
