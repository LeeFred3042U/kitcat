package atomicio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func WriteFile(filename string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filename)

	tmp, err := os.CreateTemp(dir, "atomic-*")
	if err != nil {
		return fmt.Errorf("atomicio: create temp in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	cleanup := func(cause error) error {
		tmp.Close()
		os.Remove(tmpName)
		return cause
	}

	if err := tmp.Chmod(perm); err != nil {
		return cleanup(fmt.Errorf("atomicio: chmod %s: %w", tmpName, err))
	}

	if _, err := tmp.Write(data); err != nil {
		return cleanup(fmt.Errorf("atomicio: write %s: %w", tmpName, err))
	}

	if err := tmp.Sync(); err != nil {
		return cleanup(fmt.Errorf("atomicio: sync %s: %w", tmpName, err))
	}

	if err := tmp.Close(); err != nil {
		return cleanup(fmt.Errorf("atomicio: close %s: %w", tmpName, err))
	}

	if err := renameWithRetry(tmpName, filename); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("atomicio: rename %s -> %s: %w", tmpName, filename, err)
	}

	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}

	return nil
}

func renameWithRetry(oldpath, newpath string) error {
	const maxRetries = 5
	delay := 10 * time.Millisecond

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		lastErr = os.Rename(oldpath, newpath)
		if lastErr == nil {
			return nil
		}
		if !isRetryableRenameError(lastErr) {
			return lastErr
		}
		time.Sleep(delay)
		delay *= 2
	}
	return fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
}

func isRetryableRenameError(err error) bool {
	return errors.Is(err, syscall.EBUSY) ||
		errors.Is(err, syscall.EACCES)
}
