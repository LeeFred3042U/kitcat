package integration

import (
	"os"
	"testing"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

// TestStashPushAndPop verifies the full stash round-trip: pushing a dirty
// working tree cleans the workspace, and popping it restores the modifications.
func TestStashPushAndPop(t *testing.T) {
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
		t.Fatalf("Init: %v", err)
	}

	if err := os.WriteFile("tracked.txt", []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile("tracked.txt"); err != nil {
		t.Fatalf("AddFile tracked.txt: %v", err)
	}
	if _, err := core.Commit("base commit"); err != nil {
		t.Fatalf("Commit base: %v", err)
	}

	if err := os.WriteFile("tracked.txt", []byte("modified content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := core.StashPush(""); err != nil {
		t.Fatalf("StashPush: %v", err)
	}

	afterPush, err := os.ReadFile("tracked.txt")
	if err != nil {
		t.Fatalf("reading tracked.txt after StashPush: %v", err)
	}
	if string(afterPush) != "original" {
		t.Errorf("after StashPush: expected %q, got %q", "original", string(afterPush))
	}

	if err := core.StashPop(); err != nil {
		t.Fatalf("StashPop: %v", err)
	}

	afterPop, err := os.ReadFile("tracked.txt")
	if err != nil {
		t.Fatalf("reading tracked.txt after StashPop: %v", err)
	}
	if string(afterPop) != "modified content" {
		t.Errorf("after StashPop: expected %q, got %q", "modified content", string(afterPop))
	}
}
