package core

import (
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

func MoveFile(src, dst string, force bool) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("source '%s' does not exist", src)
	}
	if !force {
		if _, err := os.Stat(dst); err == nil {
			return fmt.Errorf("destination '%s' exists (use -f to force)", dst)
		}
	}

	if err := os.Rename(src, dst); err != nil {
		return err
	}

	return storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {
		entry, ok := index[src]
		if !ok {
			return nil
		} // Untracked

		delete(index, src)
		entry.Path = dst
		index[dst] = entry
		return nil
	})
}
