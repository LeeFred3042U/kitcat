package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeWriteFile_Success(t *testing.T) {
	// Setup isolated environment
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("Hello, World!")

	// Execute SafeWriteFile
	err := SafeWriteFile(targetFile, testData, 0644)
	if err != nil {
		t.Fatalf("SafeWriteFile failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Fatal("Target file was not created")
	}

	// Verify file contents
	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read target file: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("File content mismatch. Expected %q, got %q", testData, content)
	}

	// Verify temp file was cleaned up
	tmpFile := targetFile + ".tmp"
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("Temporary file was not cleaned up")
	}
}

func TestSafeWriteFile_CreatesParentDir(t *testing.T) {
	// Setup isolated environment
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "subdir", "nested", "test.txt")
	testData := []byte("nested file content")

	// Execute SafeWriteFile
	err := SafeWriteFile(targetFile, testData, 0644)
	if err != nil {
		t.Fatalf("SafeWriteFile failed: %v", err)
	}

	// Verify parent directories were created
	parentDir := filepath.Dir(targetFile)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		t.Fatal("Parent directories were not created")
	}

	// Verify file contents
	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read target file: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("File content mismatch. Expected %q, got %q", testData, content)
	}
}

func TestSafeWriteFile_AtomicBehavior(t *testing.T) {
	// Setup isolated environment
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "atomic.txt")
	testData := []byte("atomic write test")

	// Execute SafeWriteFile
	err := SafeWriteFile(targetFile, testData, 0644)
	if err != nil {
		t.Fatalf("SafeWriteFile failed: %v", err)
	}

	// Verify temp file doesn't exist (atomic rename completed)
	tmpFile := targetFile + ".tmp"
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("Temporary file still exists after atomic rename")
	}

	// Verify target file exists
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Fatal("Target file doesn't exist after atomic rename")
	}
}

func TestSafeWriteFile_Overwrite(t *testing.T) {
	// Setup isolated environment
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "overwrite.txt")

	// Create initial file
	initialData := []byte("initial content")
	err := os.WriteFile(targetFile, initialData, 0644)
	if err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	// Overwrite with SafeWriteFile
	newData := []byte("new content")
	err = SafeWriteFile(targetFile, newData, 0644)
	if err != nil {
		t.Fatalf("SafeWriteFile failed on overwrite: %v", err)
	}

	// Verify new content
	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read target file: %v", err)
	}
	if string(content) != string(newData) {
		t.Errorf("File content mismatch after overwrite. Expected %q, got %q", newData, content)
	}
}

func TestSafeWriteFile_EmptyData(t *testing.T) {
	// Setup isolated environment
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "empty.txt")
	testData := []byte("")

	// Execute SafeWriteFile with empty data
	err := SafeWriteFile(targetFile, testData, 0644)
	if err != nil {
		t.Fatalf("SafeWriteFile failed with empty data: %v", err)
	}

	// Verify file exists
	info, err := os.Stat(targetFile)
	if os.IsNotExist(err) {
		t.Fatal("Target file was not created")
	}

	// Verify file is empty
	if info.Size() != 0 {
		t.Errorf("Expected empty file, got size %d", info.Size())
	}
}

func TestSafeWriteFile_LargeData(t *testing.T) {
	// Setup isolated environment
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "large.txt")

	// Create large data (1MB)
	testData := make([]byte, 1024*1024)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	// Execute SafeWriteFile
	err := SafeWriteFile(targetFile, testData, 0644)
	if err != nil {
		t.Fatalf("SafeWriteFile failed with large data: %v", err)
	}

	// Verify file size
	info, err := os.Stat(targetFile)
	if err != nil {
		t.Fatalf("Failed to stat target file: %v", err)
	}
	if info.Size() != int64(len(testData)) {
		t.Errorf("File size mismatch. Expected %d, got %d", len(testData), info.Size())
	}

	// Verify content integrity
	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read target file: %v", err)
	}
	if len(content) != len(testData) {
		t.Errorf("Content length mismatch. Expected %d, got %d", len(testData), len(content))
	}
}
