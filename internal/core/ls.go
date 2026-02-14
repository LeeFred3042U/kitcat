package core

import (
	"fmt"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// ListFiles prints all tracked file paths from the index to stdout.
// It reads the index via storage.LoadIndex and emits one path per line.
// Note: iteration over a map is unordered — this output is not stable across runs.
func ListFiles() error {
	index, err := storage.LoadIndex()
	if err != nil {
		return err
	}

	for path := range index {
		fmt.Println(path)
	}
	return nil
}
