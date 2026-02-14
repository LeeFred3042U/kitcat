package core

import (
	"fmt"
	"os"
	"path/filepath"
)

// Init initializes a new kitcat repository.
//
// Behaviour aligned closer to `git init`:
// - Creates full refs layout.
// - Writes HEAD only if missing.
// - Safe to run repeatedly (idempotent).
// - Ensures object and ref directories exist before use.
func Init() error {
	// Repository directory layout
	dirs := []string{
		".kitcat",
		".kitcat/objects",
		".kitcat/refs",
		".kitcat/refs/heads",
		".kitcat/refs/tags",
	}

	// Create directory structure if missing.
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// HEAD should only be created if it does not already exist.
	// This preserves current branch when re-running init.
	headPath := ".kitcat/HEAD"
	if _, err := os.Stat(headPath); os.IsNotExist(err) {
		headContent := []byte("ref: refs/heads/master\n")
		if err := os.WriteFile(headPath, headContent, 0644); err != nil {
			return fmt.Errorf("failed to create HEAD: %w", err)
		}
	}

	// Create empty config if missing so later commands don't assume existence.
	configPath := ".kitcat/config"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to create config: %w", err)
		}
	}

	// Optional description file (git init parity; harmless metadata).
	descPath := filepath.Join(".kitcat", "description")
	if _, err := os.Stat(descPath); os.IsNotExist(err) {
		_ = os.WriteFile(descPath, []byte("Unnamed kitcat repository\n"), 0644)
	}

	return nil
}
