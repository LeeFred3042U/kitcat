//go:build !linux && !darwin && !freebsd

package storage

import (
	"fmt"
	"os"
	"time"
)

// lock acquires a filesystem-based lock for the provided path by creating
// a companion ".lock" file using atomic file creation semantics.
//
// This implementation is used on platforms where syscall.Flock is unavailable.
// Mutual exclusion is achieved by attempting to create the lock file with the
// O_CREATE|O_EXCL flags, which guarantees that the operation fails if the file
// already exists.
//
// If the lock file already exists, the function retries periodically until
// either the lock becomes available or a timeout occurs.
func lock(path string) (*os.File, error) {
	lockFile := path + ".lock"

	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		// Try to create the file exclusively. This fails if file exists.
		f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
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

// LockFile applies a file lock to the provided file descriptor.
//
// On non-Unix platforms this function is currently a no-op because the
// storage package relies on the lock file itself to provide mutual
// exclusion. The function exists to preserve a consistent API with the
// Unix implementation where a real advisory lock is applied.
func LockFile(f *os.File) error {
	return nil
}

// unlock releases the lock acquired by lock by closing the lock file
// descriptor and removing the lock file from the filesystem.
//
// Removing the file signals that the critical section is complete and
// allows other processes to acquire the lock.
func unlock(f *os.File) {
	// Close and remove the lock file to release the lock
	name := f.Name()
	f.Close()
	os.Remove(name)
}
