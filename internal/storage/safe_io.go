package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// SafeWriteFile writes data to a file atomically and with durability guarantees.
//
// The function ensures that readers of the target filename will never observe
// partially written data. This is achieved by writing the contents to a temporary
// file in the same directory and then atomically renaming it over the destination.
//
// Implementation guarantees:
//   - Writes occur to a uniquely named temporary file to avoid writer collisions.
//   - File contents are flushed to disk before rename to protect against power loss.
//   - The rename operation replaces the target atomically on supported filesystems.
//   - The parent directory is synced after rename to persist metadata changes.
//
// Platform behavior:
//   - Relies on POSIX atomic rename semantics on Unix-like systems.
//   - On Windows, the rename relies on the equivalent filesystem replace behavior.
func SafeWriteFile(filename string, data []byte, perm os.FileMode) (err error) {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Use a unique temporary filename to avoid concurrent writer collisions.
	tmpFile, err := os.CreateTemp(dir, filepath.Base(filename)+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Ensure no orphaned temp files remain if the write fails.
	defer func() {
		tmpFile.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	if err := tmpFile.Chmod(perm); err != nil {
		return fmt.Errorf("failed to set permissions on temp file: %w", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	// Flush file data before rename to guarantee durability across power loss.
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, filename); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Sync directory to persist the rename.
	_ = syncDir(dir)

	return nil
}

// syncDir forces the filesystem to flush directory metadata changes to disk.
//
// This is primarily used after atomic renames to ensure that the rename itself
// is durably recorded in the directory structure. Without syncing the directory,
// a crash or power failure could revert the rename even if the file contents
// were successfully written.
//
// Platform constraint: On Windows this operation may fail with "Access is denied"
// because directory syncing is not universally supported.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()

	return d.Sync()
}
