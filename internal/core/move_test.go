package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Replace loadIndexForTest with a call to storage.LoadIndex() (which correctly uses plumbing.ReadIndex).

func TestMoveFile(t *testing.T) {
	// Restores the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	// Create temp directory
	tmpDir := t.TempDir()

	// Change working directory into temp repo
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Initialize kitkat repository
	if err := Init(); err != nil {
		t.Fatal(err)
	}

	oldPath := "old_test.txt"
	newPath := "new_test.txt"

	// Create old file
	if err := os.WriteFile(oldPath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Stage old file
	if err := AddFile(oldPath); err != nil {
		t.Fatal(err)
	}

	// Move file
	if err := MoveFile(oldPath, newPath, false); err != nil {
		t.Fatal(err)
	}

	// Old file should be gone
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expected old file to be removed")
	}

	// New file should exist
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("expected new file to exist")
	}

	// Index should contain new file
	idx, err := loadIndexForTest(filepath.Join(tmpDir, ".kitcat", "index"))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := idx[newPath]; !ok {
		t.Fatalf("expected new file to be staged")
	}
}

func TestMoveFile_DestinationExists(t *testing.T) {
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd() // assuming no error in test setup

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(cwd) // Ignore error in cleanup
	}()

	if err := Init(); err != nil {
		t.Fatal(err)
	}

	// Create source and destination files
	src := "source.txt"
	dst := "destination.txt"
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("hola"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Stage source and destination files
	if err := AddFile(src); err != nil {
		t.Fatal(err)
	}
	if err := AddFile(dst); err != nil {
		t.Fatal(err)
	}

	// Moves the file without the flag
	if err := MoveFile(src, dst, false); err == nil {
		t.Fatalf("expected error when destination exists")
	}

	// Moves the file with the flag
	if err := MoveFile(src, dst, true); err != nil {
		t.Fatal(err)
	}

	// Checks for the files to be moved
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("expected src to be moved")
	}

	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("expected dst to exist")
	}

	// Index should contain the destination file
	idx, err := loadIndexForTest(filepath.Join(tmpDir, ".kitcat", "index"))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := idx[dst]; !ok {
		t.Fatalf("expected new file to be staged")
	}
}

func TestMoveFile_SamePath(t *testing.T) {
	// Restores the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	// Create a temp directory
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)

	// Initialize kitkat repository
	if err := Init(); err != nil {
		t.Fatal(err)
	}

	// Create source and destination files
	f := "file"
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Stage source and destination files
	if err := AddFile(f); err != nil {
		t.Fatal(err)
	}

	if err := MoveFile(f, f, false); err == nil {
		t.Fatalf("expected error for same source and destination")
	}

	if err := MoveFile(f, f, true); err == nil {
		t.Fatalf("expected error for same source and destination")
	}
}

func loadIndexForTest(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var idx map[string]string
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}

	return idx, nil
}
