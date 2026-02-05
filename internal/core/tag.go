package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const tagsDir = ".kitcat/refs/tags"

// Creates a new lightweight tag pointing to a specific commit

func CreateTag(tagName, commitID string) error {
	if !IsRepoInitialized() {
		return fmt.Errorf("not a kitcat repository (or any of the parent directories): .kitcat")
	}

	if !IsValidRefName(tagName) {
		return fmt.Errorf("invalid tag name: %s", tagName)
	}

	if err := os.MkdirAll(tagsDir, 0o755); err != nil {
		return err
	}

	tagPath := filepath.Join(tagsDir, tagName)
	// Checks if tag already exists.
	if _, err := os.Stat(tagPath); err == nil {
		return fmt.Errorf("error: tag %s already exists", tagName)
	} else if !os.IsNotExist(err) {
		return err
	}

	// Creates a new tag.
	if err := os.WriteFile(tagPath, []byte(commitID), 0o644); err != nil {
		return err
	}

	fmt.Printf("Tag '%s' created for commit %s\n", tagName, commitID)
	return nil
}

// ListTags returns all tag names stored in .kitcat/refs/tags
func ListTags() ([]string, error) {
	if !IsRepoInitialized() {
		return nil, fmt.Errorf(
			"not a kitcat repository (or any of the parent directories): .kitcat",
		)
	}

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

		if entry.IsDir() {
			continue
		}
		tags = append(tags, entry.Name())
	}

	sort.Strings(tags)
	return tags, nil
}

// PrintTags prints all tags, one per line
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
