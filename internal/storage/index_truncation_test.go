package storage

import (
	"encoding/json"
	"os"
	"testing"
)

func TestIndexWriteSanity(t *testing.T) {
	// Setup isolated environment
	tmpDir := t.TempDir()

	// Switch CWD to temp dir because WriteIndex writes to ".kitcat/index" relative path
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to chdir to temp dir: %v", err)
	}

	// Prepare Index Data
	indexData := map[string]string{
		"test_file.txt": "da39a3ee5e6b4b0d3255bfef95601890afd80709",
		"README.md":     "8843d7f92416211de9ebb963ff4ce28125932878",
	}

	// Execute WriteIndex
	if err := WriteIndex(indexData); err != nil {
		t.Fatalf("WriteIndex failed: %v", err)
	}

	// Assertions
	targetPath := ".kitcat/index"

	// Assert file exists
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("Failed to read index file at %s: %v", targetPath, err)
	}

	// Assert Valid JSON
	var loadedMap map[string]string
	if err := json.Unmarshal(content, &loadedMap); err != nil {
		t.Fatalf("Index file contains invalid JSON: %v", err)
	}

	// Assert Content Integrity
	if len(loadedMap) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(loadedMap))
	}
	if loadedMap["test_file.txt"] != "da39a3ee5e6b4b0d3255bfef95601890afd80709" {
		t.Errorf("Index content mismatch for test_file.txt")
	}
}
