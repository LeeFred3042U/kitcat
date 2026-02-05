//go:build !linux && !darwin && !freebsd

package storage

import (
	"fmt"
	"os"
	"time"
)

// lock implements a spin-lock using atomic file creation.
// On Windows/non-Unix, we can't easily use syscall.Flock, so we use the existence
// of the lock file as the lock itself.
func lock(path string) (*os.File, error) {
	lockFile := path + ".lock"

	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		// Try to create the file exclusively. This fails if file exists.
		f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			return f, nil
		}

		if !os.IsExist(err) {
			return nil, fmt.Errorf("failed to acquire lock: %w", err)
		}

		// File exists, wait and retry
		select {
		case <-timeout:
			return nil, fmt.Errorf("timed out acquiring lock for %s", path)
		case <-ticker.C:
			continue
		}
	}
}

// LockFile is a no-op on non-Unix systems (unless we add valid windows locking later)
func LockFile(f *os.File) error {
	return nil
}

func unlock(f *os.File) {
	// Close and remove the lock file to release the lock
	name := f.Name()
	f.Close()
	os.Remove(name)
}
