package core

import (
	"fmt"
	"os"
	"path/filepath"
	
	"github.com/LeeFred3042U/kitcat/internal/repo"
)

// Init bootstraps the minimal kitcat repository layout.
//
// The operation is idempotent:
//   - existing files are never overwritten
//   - the current branch (HEAD) is preserved
//
// Higher-level commands assume this structure exists,
// so Init guarantees a consistent on-disk layout.
func Init() error {
	// Core repository directories required for objects, refs and hooks.
	dirs := []string{
		repo.Dir,
		filepath.Join(repo.Dir, "hooks"),
		filepath.Join(repo.Dir, "info"),
		filepath.Join(repo.Dir, "objects"),
		filepath.Join(repo.Dir, "objects", "info"),
		filepath.Join(repo.Dir, "objects", "pack"),
		filepath.Join(repo.Dir, "refs"),
		filepath.Join(repo.Dir, "refs", "heads"),
		filepath.Join(repo.Dir, "refs", "tags"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Initialize HEAD only if missing to avoid resetting
	// the active branch on repeated init calls.
	headPath := filepath.Join(repo.Dir, "HEAD")
	if _, err := os.Stat(headPath); os.IsNotExist(err) {
		// Default new repositories to 'main' instead of 'master'
		headContent := []byte("ref: refs/heads/main\n")
		if err := os.WriteFile(headPath, headContent, 0o644); err != nil {
			return fmt.Errorf("failed to create HEAD: %w", err)
		}
	}

	// Create a Git-compliant INI config file so later commands can
	// safely append settings and external Git tools can read it.
	configPath := filepath.Join(repo.Dir, "config")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configContent := []byte("[core]\n\trepositoryformatversion = 0\n\tfilemode = false\n\tbare = false\n\tlogallrefupdates = true\n")
		if err := os.WriteFile(configPath, configContent, 0o644); err != nil {
			return fmt.Errorf("failed to create config: %w", err)
		}
	}

	// Description is informational only; defaults are safe.
	descPath := filepath.Join(repo.Dir, "description")
	if _, err := os.Stat(descPath); os.IsNotExist(err) {
		if err := os.WriteFile(descPath, []byte("Unnamed kitcat repository\n"), 0o644); err != nil {
			return fmt.Errorf("failed to create description: %w", err)
		}
	}

	// Local exclude rules behave like Git's info/exclude:
	// repo-specific ignores that are not tracked.
	excludePath := filepath.Join(repo.Dir, "info", "exclude")
	if _, err := os.Stat(excludePath); os.IsNotExist(err) {
		excludeContent := []byte(
			"# kitcat local exclude rules\n" +
				"# Lines starting with '#' are comments.\n",
		)
		if err := os.WriteFile(excludePath, excludeContent, 0o644); err != nil {
			return fmt.Errorf("failed to create exclude file: %w", err)
		}
	}

	// Provide sample hooks as templates. Never overwrite
	// existing files to preserve user modifications.
	hooks := map[string]string{
		"pre-commit.sample": "#!/bin/sh\n# Example pre-commit hook\n",
		"commit-msg.sample": "#!/bin/sh\n# Example commit-msg hook\n",
		"pre-push.sample":   "#!/bin/sh\n# Example pre-push hook\n",
	}

	for name, content := range hooks {
		hookPath := filepath.Join(repo.Dir, "hooks", name)
		if _, err := os.Stat(hookPath); os.IsNotExist(err) {
			if err := os.WriteFile(hookPath, []byte(content), 0o755); err != nil {
				return fmt.Errorf("failed to create hook %s: %w", name, err)
			}
		}
	}

	return nil
}
