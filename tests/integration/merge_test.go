package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LeeFred3042U/kitcat/internal/core"
	"github.com/LeeFred3042U/kitcat/internal/repo"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// TestMerge_FastForward verifies that merging a branch that is a direct
// descendant of the current HEAD performs a fast-forward: the HEAD pointer
// simply advances to the feature tip with no new merge commit created.
func TestMerge_FastForward(t *testing.T) {
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

	if err := os.WriteFile("file1.txt", []byte("file1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile("file1.txt"); err != nil {
		t.Fatalf("AddFile file1.txt: %v", err)
	}
	hashA, err := core.Commit("commit A")
	if err != nil {
		t.Fatalf("Commit A: %v", err)
	}

	if err := core.CreateBranch("feature"); err != nil {
		t.Fatalf("CreateBranch feature: %v", err)
	}
	if err := core.Checkout("feature", false); err != nil {
		t.Fatalf("Checkout feature: %v", err)
	}

	if err := os.WriteFile("file2.txt", []byte("file2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile("file2.txt"); err != nil {
		t.Fatalf("AddFile file2.txt: %v", err)
	}
	hashB, err := core.Commit("commit B")
	if err != nil {
		t.Fatalf("Commit B: %v", err)
	}

	if err := core.Checkout("main", false); err != nil {
		t.Fatalf("Checkout main: %v", err)
	}
	if err := core.Merge("feature"); err != nil {
		t.Fatalf("Merge feature (FF): %v", err)
	}

	head, err := storage.GetLastCommit()
	if err != nil {
		t.Fatalf("GetLastCommit after FF merge: %v", err)
	}
	if head.ID != hashB {
		t.Errorf("expected HEAD == hashB (%s), got %s", hashB, head.ID)
	}

	if len(head.Parents) != 1 {
		t.Errorf("expected 1 parent after FF, got %d: %v", len(head.Parents), head.Parents)
	}
	if head.Parents[0] != hashA {
		t.Errorf("expected parent == hashA (%s), got %s", hashA, head.Parents[0])
	}

	if _, err := os.Stat("file2.txt"); err != nil {
		t.Errorf("expected file2.txt to exist on disk after FF merge: %v", err)
	}
}

// TestMerge_ThreeWay verifies that merging two diverged branches (neither is a
// direct ancestor of the other) creates a proper merge commit with two parents,
// and that non-conflicting files from both branches are present on disk.
func TestMerge_ThreeWay(t *testing.T) {
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

	if err := os.WriteFile("file1.txt", []byte("shared"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile("file1.txt"); err != nil {
		t.Fatalf("AddFile file1.txt: %v", err)
	}
	if _, err := core.Commit("commit A"); err != nil {
		t.Fatalf("Commit A: %v", err)
	}

	if err := core.CreateBranch("feature"); err != nil {
		t.Fatalf("CreateBranch feature: %v", err)
	}
	if err := core.Checkout("feature", false); err != nil {
		t.Fatalf("Checkout feature: %v", err)
	}
	if err := os.WriteFile("file2.txt", []byte("from feature"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile("file2.txt"); err != nil {
		t.Fatalf("AddFile file2.txt: %v", err)
	}
	hashB, err := core.Commit("commit B")
	if err != nil {
		t.Fatalf("Commit B: %v", err)
	}

	if err := core.Checkout("main", false); err != nil {
		t.Fatalf("Checkout main: %v", err)
	}
	if err := os.WriteFile("file3.txt", []byte("from main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile("file3.txt"); err != nil {
		t.Fatalf("AddFile file3.txt: %v", err)
	}
	hashC, err := core.Commit("commit C")
	if err != nil {
		t.Fatalf("Commit C: %v", err)
	}

	if err := core.Merge("feature"); err != nil {
		t.Fatalf("Merge feature (3-way): %v", err)
	}

	mergeHeadPath := filepath.Join(repo.Dir, "MERGE_HEAD")
	if _, err := os.Stat(mergeHeadPath); err != nil {
		t.Errorf("expected MERGE_HEAD to exist before finalising merge commit: %v", err)
	}

	if _, err := core.Commit(""); err != nil {
		t.Fatalf("Commit (merge finalise): %v", err)
	}

	head, err := storage.GetLastCommit()
	if err != nil {
		t.Fatalf("GetLastCommit after 3-way merge: %v", err)
	}
	if len(head.Parents) != 2 {
		t.Fatalf("expected 2 parents on merge commit, got %d: %v", len(head.Parents), head.Parents)
	}

	if head.Parents[0] != hashC {
		t.Errorf("expected Parents[0] == hashC (%s), got %s", hashC, head.Parents[0])
	}
	if head.Parents[1] != hashB {
		t.Errorf("expected Parents[1] == hashB (%s), got %s", hashB, head.Parents[1])
	}

	for _, name := range []string{"file2.txt", "file3.txt"} {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("expected %s to exist on disk after 3-way merge: %v", name, err)
		}
	}
}
