package atomicio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFile_CreateAndOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "test.txt")

	t.Run("create new file", func(t *testing.T) {
		data := []byte("hello kitcat")
		if err := WriteFile(target, data, 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		got, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(got) != "hello kitcat" {
			t.Errorf("content = %q, want %q", string(got), "hello kitcat")
		}

		info, _ := os.Stat(target)
		if info.Mode().Perm() != 0o644 {
			t.Errorf("permissions = %o, want 0644", info.Mode().Perm())
		}
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		newData := []byte("updated content")
		if err := WriteFile(target, newData, 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		got, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(got) != "updated content" {
			t.Errorf("content = %q, want %q", string(got), "updated content")
		}
	})

	t.Run("no temp artifacts remain", func(t *testing.T) {
		entries, err := os.ReadDir(tmpDir)
		if err != nil {
			t.Fatalf("ReadDir() error = %v", err)
		}
		for _, e := range entries {
			if e.Name() != "test.txt" {
				t.Errorf("unexpected file left behind: %s", e.Name())
			}
		}
	})
}

func TestWriteFile_MissingParentDir(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "nonexistent", "subdir", "file.txt")

	err := WriteFile(target, []byte("data"), 0o644)
	if err == nil {
		t.Fatal("expected error for missing parent directory, got nil")
	}
}
