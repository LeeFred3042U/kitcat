package core

import (
	"fmt"
	"os"
)

// Init initializes a new kitcat repository.
func Init() error {
	dirs := []string{
		".kitcat",
		".kitcat/objects",
		".kitcat/refs",
		".kitcat/refs/heads",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	headPath := ".kitcat/HEAD"
	if _, err := os.Stat(headPath); os.IsNotExist(err) {
		if err := os.WriteFile(headPath, []byte("ref: refs/heads/master\n"), 0644); err != nil {
			return fmt.Errorf("failed to create HEAD: %w", err)
		}
	}

	return nil
}
