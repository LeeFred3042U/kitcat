package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// SetupTestRepo creates a temporary kitcat repository for tests.
// It returns the path to the repository root and a cleanup function.
func SetupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	// Save current working directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Create temp repo
	repoDir := t.TempDir()

	// Switch to temp repo
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("failed to change directory to temp repo: %v", err)
	}

	kitcatDir := filepath.Join(repoDir, ".kitcat")

	// Create .kitcat directory
	if err := os.MkdirAll(kitcatDir, 0o755); err != nil {
		t.Fatalf("failed to create .kitcat dir: %v", err)
	}

	// Create required subdirectories
	dirs := []string{
		"objects",
		filepath.Join("refs", "heads"),
		filepath.Join("refs", "tags"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(kitcatDir, d), 0o755); err != nil {
			t.Fatalf("failed to create %s dir: %v", d, err)
		}
	}

	// Create empty files required by storage functions
	files := []string{
		"index",
		"commits.log",
		filepath.Join("refs", "heads", "main"),
	}

	for _, f := range files {
		if err := os.WriteFile(filepath.Join(kitcatDir, f), []byte{}, 0o644); err != nil {
			t.Fatalf("failed to create %s: %v", f, err)
		}
	}

	// Create HEAD file
	headPath := filepath.Join(kitcatDir, "HEAD")
	if err := os.WriteFile(headPath, []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("failed to create HEAD file: %v", err)
	}

	// Cleanup function
	cleanup := func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Fatalf("failed to restore working directory: %v", err)
		}
	}

	return repoDir, cleanup
}
