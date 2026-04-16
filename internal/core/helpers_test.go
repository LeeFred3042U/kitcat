package core

import (
	"os"
	"testing"
)

// TestIsWorkDirDirty_Sentinels verifies that IsWorkDirDirty returns (true, nil)
// when the working directory contains unstaged modifications, and that no
// internal sentinel error values leak out through the error return.
func TestIsWorkDirDirty_Sentinels(t *testing.T) {
	tmpDir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd) //nolint:errcheck

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	if err := Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if err := os.WriteFile("file.txt", []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AddFile("file.txt"); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}
	if _, err := Commit("initial commit"); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if err := os.WriteFile("file.txt", []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}

	dirty, err := IsWorkDirDirty()
	if err != nil {
		t.Fatalf("IsWorkDirDirty returned unexpected error: %v", err)
	}
	if !dirty {
		t.Fatal("expected working directory to be dirty after unstaged modification")
	}
}
