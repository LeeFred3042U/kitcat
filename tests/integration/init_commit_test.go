package integration

import (
	"os"
	"regexp"
	"testing"

	"github.com/LeeFred3042U/kitcat/internal/core"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// TestInitAndFirstCommit exercises the full init → add → commit lifecycle and
// verifies the resulting commit object is well-formed.
func TestInitAndFirstCommit(t *testing.T) {
	tmpDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir) //nolint:errcheck

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	if err := core.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if err := os.WriteFile("a.txt", []byte("a, kitcat!\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := core.AddFile("a.txt"); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	hash, err := core.Commit("initial commit")
	if err != nil {
		t.Fatalf("Commit returned error: %v", err)
	}

	hexRe := regexp.MustCompile(`^[0-9a-f]{40}$`)
	if !hexRe.MatchString(hash) {
		t.Errorf("expected 40-char hex hash, got %q", hash)
	}

	commit, err := storage.GetLastCommit()
	if err != nil {
		t.Fatalf("GetLastCommit failed: %v", err)
	}

	if commit.Message != "initial commit" {
		t.Errorf("commit.Message: expected %q, got %q", "initial commit", commit.Message)
	}

	if len(commit.Parents) != 0 {
		t.Errorf("expected 0 parents for root commit, got %d: %v", len(commit.Parents), commit.Parents)
	}
}
