package core

import (
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

func RemoveFile(path string, recursive bool) error {
	if !recursive {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			return fmt.Errorf("cannot remove '%s': Is a directory", path)
		}
	}

	if err := os.RemoveAll(path); err != nil {
		return err
	}

	return storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {
		delete(index, path)
		return nil
	})
}
