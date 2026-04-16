package integration

import (
	"os"
	"testing"

	"github.com/LeeFred3042U/kitcat/internal/core"
)

// TestBranchAndCheckout verifies that creating a branch, checking it out,
// committing on it, and then switching back to main correctly isolates file
// state per branch.

func TestBranchAndCheckout(t *testing.T) {
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

	if err := os.WriteFile("main.txt", []byte("main content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile("main.txt"); err != nil {
		t.Fatalf("AddFile main.txt: %v", err)
	}
	if _, err := core.Commit("main: initial"); err != nil {
		t.Fatalf("Commit on main: %v", err)
	}

	if err := core.CreateBranch("feature"); err != nil {
		t.Fatalf("CreateBranch feature: %v", err)
	}

	if err := core.Checkout("feature", false); err != nil {
		t.Fatalf("Checkout feature: %v", err)
	}

	state, err := core.GetHeadState()
	if err != nil {
		t.Fatalf("GetHeadState after checkout feature: %v", err)
	}
	if state != "feature" {
		t.Errorf("expected HEAD on 'feature', got %q", state)
	}

	if err := os.WriteFile("feature.txt", []byte("feature only"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile("feature.txt"); err != nil {
		t.Fatalf("AddFile feature.txt: %v", err)
	}
	if _, err := core.Commit("feature: add file"); err != nil {
		t.Fatalf("Commit on feature: %v", err)
	}

	if err := core.Checkout("main", false); err != nil {
		t.Fatalf("Checkout main: %v", err)
	}

	if _, err := os.Stat("feature.txt"); !os.IsNotExist(err) {
		t.Errorf("feature.txt should not exist on main branch (err: %v)", err)
	}

	state, err = core.GetHeadState()
	if err != nil {
		t.Fatalf("GetHeadState after checkout main: %v", err)
	}
	if state != "main" {
		t.Errorf("expected HEAD on 'main', got %q", state)
	}
}
