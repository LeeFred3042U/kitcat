package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// SafeWriteFile writes data to a file atomically and durably.
// It uses a temporary file, syncs it to disk, then atomically renames it.
// The parent directory is also synced to ensure the rename is durable.
func SafeWriteFile(filename string, data []byte, perm os.FileMode) error {
	// Ensure the parent directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Write to a temporary file in the same directory
	tmpPath := filename + ".tmp"
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	// Write data to temp file
	_, writeErr := tmpFile.Write(data)

	// Sync the file to ensure data is flushed to disk
	syncErr := tmpFile.Sync()

	// Close the file
	closeErr := tmpFile.Close()

	// Check for errors in order of occurrence
	if writeErr != nil {
		os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to write data: %w", writeErr)
	}
	if syncErr != nil {
		os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to sync temp file: %w", syncErr)
	}
	if closeErr != nil {
		os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to close temp file: %w", closeErr)
	}

	// Atomically rename temp file to target file
	if err := os.Rename(tmpPath, filename); err != nil {
		os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Sync the parent directory to ensure the rename is durable
	// This is best-effort; on some platforms (like Windows) it may fail
	_ = syncDir(dir)

	return nil
}

// syncDir syncs a directory to ensure metadata changes (like renames) are durable
// This is best-effort and may not work on all platforms (e.g., Windows)
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()

	// Sync may fail on Windows with "Access is denied"
	// This is expected and not a critical error
	return d.Sync()
}
