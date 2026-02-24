package core

import (
	"fmt"
	"sort"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// ListFiles prints all tracked file paths from the index to stdout.
// It reads the index via storage.LoadIndex and emits one path per line.
// The output is alphabetically sorted to match Git's deterministic behavior.
func ListFiles() error {
	index, err := storage.LoadIndex()
	if err != nil {
		return err
	}

	var paths []string
	for path := range index {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		fmt.Println(path)
	}
	return nil
}