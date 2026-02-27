package core

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

func TestGrepBasic(t *testing.T) {
	tmpDir := t.TempDir()

	// Save and restore working directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Fatalf("failed to restore working dir: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create test files
	mainContent := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile("main.go", []byte(mainContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile("ignore.go", []byte("func ignored() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create .kitcat directory
	if err := os.Mkdir(".kitcat", 0755); err != nil {
		t.Fatal(err)
	}

	// Write kitcat index (ONLY tracked files)
	index := map[string]string{
		"main.go": "dummyhash",
	}

	if err := storage.WriteIndex(index); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	// Run grep
	if err := Grep([]string{"--line-number", "func"}); err != nil {
		t.Fatalf("grep returned error: %v", err)
	}

	// Restore stdout
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = oldStdout

	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	output := buf.String()

	// Assert expected match
	if !strings.Contains(output, "main.go:3:func main()") {
		t.Fatalf("expected match not found, got:\n%s", output)
	}

	// Ensure untracked file is ignored
	if strings.Contains(output, "ignore.go") {
		t.Fatalf("untracked file should not be searched")
	}
}
