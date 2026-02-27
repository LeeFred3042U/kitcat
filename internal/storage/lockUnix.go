//go:build linux || darwin || freebsd

package storage

import (
	"os"
	"syscall"
)

// lock uses a real file lock (flock) on Unix systems
func lock(path string) (*os.File, error) {
	lockFile := path + ".lock"
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := LockFile(f); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// LockFile applies an exclusive lock on the file descriptor
func LockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// unlock releases the file lock on Unix systems
func unlock(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	f.Close()
}
