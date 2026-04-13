package core

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/repo"
)

// PrintTags lists all tags in the repository.
func PrintTags() error {
	if _, err := os.Stat(repo.Dir); os.IsNotExist(err) {
		return fmt.Errorf("not a %s repository (or any of the parent directories): %s", app.Name, repo.Dir)
	}

	entries, err := os.ReadDir(repo.TagsDir)
	if os.IsNotExist(err) {
		return nil // No tags yet
	}
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fmt.Println(entry.Name())
	}
	return nil
}

// CreateTag creates a lightweight tag pointing to a specific commit.
func CreateTag(tagName, commitHash string) error {
	if _, err := os.Stat(repo.Dir); os.IsNotExist(err) {
		return fmt.Errorf("not a %s repository (or any of the parent directories): %s", app.Name, repo.Dir)
	}

	tagPath := filepath.Join(repo.TagsDir, tagName)

	if err := os.MkdirAll(repo.TagsDir, 0o755); err != nil {
		return err
	}

	if err := SafeWrite(tagPath, []byte(commitHash+"\n"), 0o644); err != nil {
		return err
	}

	fmt.Printf("Tag '%s' created for commit %s\n", tagName, commitHash)
	return nil
}

// CreateAnnotatedTag creates a Git-compliant annotated tag object in the database.
func CreateAnnotatedTag(tagName, commitHash, message string) error {
	if _, err := os.Stat(repo.Dir); os.IsNotExist(err) {
		return fmt.Errorf("not a %s repository (or any of the parent directories): %s", app.Name, repo.Dir)
	}

	// 1. Get tagger identity
	name, _, _ := GetConfig("user.name")
	email, _, _ := GetConfig("user.email")
	if name == "" {
		name = "Unknown"
	}
	if email == "" {
		email = "unknown@example.com"
	}

	// 2. Format Timestamp (Unix Time + Timezone Offset)
	now := time.Now()
	timestamp := now.Unix()
	_, offset := now.Zone()
	tzSign := "+"
	if offset < 0 {
		tzSign = "-"
		offset = -offset
	}
	tzHours := offset / 3600
	tzMins := (offset % 3600) / 60
	tzStr := fmt.Sprintf("%s%02d%02d", tzSign, tzHours, tzMins)

	taggerStr := fmt.Sprintf("%s <%s> %d %s", name, email, timestamp, tzStr)

	// 3. Construct the exact Git Tag Object payload
	payload := fmt.Sprintf("object %s\ntype commit\ntag %s\ntagger %s\n\n%s\n", commitHash, tagName, taggerStr, message)

	// 4. Hash and write the Tag Object to the database (.kitcat/objects/...)
	tagHash, err := plumbing.HashAndWriteObject([]byte(payload), "tag")
	if err != nil {
		return fmt.Errorf("failed to write tag object: %w", err)
	}

	// 5. Write the TAG OBJECT'S HASH to the refs/tags/ file
	tagPath := filepath.Join(repo.TagsDir, tagName)
	if err := os.MkdirAll(repo.TagsDir, 0o755); err != nil {
		return err
	}

	if err := SafeWrite(tagPath, []byte(tagHash+"\n"), 0o644); err != nil {
		return err
	}

	return nil
}
