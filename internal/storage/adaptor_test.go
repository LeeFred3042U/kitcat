package storage

import (
	"os"
	"testing"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
)

// TestFindCommit_MultipleParents verifies that FindCommit correctly parses and
// returns all parent hashes from a merge-style commit object that carries two
// parent references.

func TestFindCommit_MultipleParents(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir) //nolint:errcheck

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(".kitcat/objects", 0o755); err != nil {
		t.Fatal(err)
	}

	parent1Hash, err := plumbing.HashAndWriteObject(
		[]byte("tree aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\nauthor A <a@a> 0 +0000\n\nparent-1\n"),
		"commit",
	)
	if err != nil {
		t.Fatalf("writing parent1: %v", err)
	}

	parent2Hash, err := plumbing.HashAndWriteObject(
		[]byte("tree bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\nauthor A <a@a> 0 +0000\n\nparent-2\n"),
		"commit",
	)
	if err != nil {
		t.Fatalf("writing parent2: %v", err)
	}

	mergePayload := "tree cccccccccccccccccccccccccccccccccccccccc\n" +
		"parent " + parent1Hash + "\n" +
		"parent " + parent2Hash + "\n" +
		"author Merge Bot <bot@example.com> 1700000000 +0000\n\n" +
		"Merge branch 'feature'\n"

	mergeHash, err := plumbing.HashAndWriteObject([]byte(mergePayload), "commit")
	if err != nil {
		t.Fatalf("writing merge commit: %v", err)
	}

	commit, err := FindCommit(mergeHash)
	if err != nil {
		t.Fatalf("FindCommit(%s) returned error: %v", mergeHash, err)
	}
	if len(commit.Parents) != 2 {
		t.Fatalf("expected 2 parents, got %d: %v", len(commit.Parents), commit.Parents)
	}
	if commit.Parents[0] != parent1Hash {
		t.Errorf("Parents[0]: expected %s, got %s", parent1Hash, commit.Parents[0])
	}
	if commit.Parents[1] != parent2Hash {
		t.Errorf("Parents[1]: expected %s, got %s", parent2Hash, commit.Parents[1])
	}
}

// TestReadObject_RoundTrip verifies the full write-then-read cycle through the
// content-addressable object database.
func TestReadObject_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir) //nolint:errcheck

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(".kitcat/objects", 0o755); err != nil {
		t.Fatal(err)
	}

	content := []byte("hello world")
	hash, err := plumbing.HashAndWriteObject(content, "blob")
	if err != nil {
		t.Fatalf("HashAndWriteObject failed: %v", err)
	}
	if len(hash) != 40 {
		t.Fatalf("expected 40-char hex hash, got %q (%d chars)", hash, len(hash))
	}

	got, err := ReadObject(hash)
	if err != nil {
		t.Fatalf("ReadObject(%s) failed: %v", hash, err)
	}
	if string(got) != "hello world" {
		t.Fatalf("round-trip mismatch: expected %q, got %q", "hello world", string(got))
	}
}
