package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const tagsDir = ".kitcat/refs/tags"

// CreateTag creates a lightweight tag by writing a file that points
// directly to a commit hash. Fails if repository is not initialized
// or if the tag name already exists.
func CreateTag(tagName, commitID string) error {
	// Ensure commands operate only inside a valid repository root.
	if !IsRepoInitialized() {
		return fmt.Errorf("not a kitkat repository (or any of the parent directories): .kitcat")
	}

	// Tags directory is created lazily to mirror Git ref layout behavior.
	if err := os.MkdirAll(tagsDir, 0755); err != nil {
		return err
	}

	tagPath := filepath.Join(tagsDir, tagName)

	// Prevent accidental overwrite; tags are treated as immutable refs.
	if _, err := os.Stat(tagPath); err == nil {
		return fmt.Errorf("error: tag %s already exists", tagName)
	} else if !os.IsNotExist(err) {
		return err
	}

	// Lightweight tags store only the commit hash as raw file content.
	if err := os.WriteFile(tagPath, []byte(commitID), 0644); err != nil {
		return err
	}

	// CLI-facing feedback; introduces stdout side effect.
	fmt.Printf("Tag '%s' created for commit %s\n", tagName, commitID)
	return nil
}

// ListTags reads all lightweight tags from disk and returns them
// sorted lexicographically for stable CLI output.
func ListTags() ([]string, error) {
	// Repository validation prevents scanning arbitrary filesystem paths.
	if !IsRepoInitialized() {
		return nil, fmt.Errorf("not a kitkat repository (or any of the parent directories): .kitcat")
	}

	// Absence of tag directory is treated as empty state rather than error.
	if _, err := os.Stat(tagsDir); err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	entries, err := os.ReadDir(tagsDir)
	if err != nil {
		return nil, err
	}

	var tags []string
	for _, entry := range entries {
		// Skip subdirectories to avoid interpreting nested refs or invalid layout.
		if entry.IsDir() {
			continue
		}
		tags = append(tags, entry.Name())
	}

	sort.Strings(tags)
	return tags, nil
}

// PrintTags prints all tags to stdout, one per line.
// Intended for CLI use; wraps ListTags for separation of concerns.
func PrintTags() error {
	tags, err := ListTags()
	if err != nil {
		return err
	}

	for _, tag := range tags {
		fmt.Println(tag)
	}
	return nil
}
