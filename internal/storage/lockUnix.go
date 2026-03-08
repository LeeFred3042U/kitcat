//go:build linux || darwin || freebsd

package storage

import (
	"os"
	"syscall"
)

// lock acquires an exclusive advisory lock for the given path using the
// Unix flock system call.
//
// The lock is implemented by opening (or creating) a companion ".lock"
// file and applying syscall.Flock with LOCK_EX. The returned file handle
// must remain open for the duration of the critical section; closing it
// releases the lock.
func lock(path string) (*os.File, error) {
	lockFile := path + ".lock"
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := LockFile(f); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// LockFile applies an exclusive advisory lock to the provided file
// descriptor using syscall.Flock.
//
// The lock is process-scoped and blocks until the lock becomes available.
// Callers must keep the file descriptor open while the lock is held.
func LockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// unlock releases the advisory lock previously acquired with LockFile
// and closes the associated file descriptor.
//
// Releasing the lock allows other processes waiting on the same lock
// file to proceed.
func unlock(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	f.Close()
}
