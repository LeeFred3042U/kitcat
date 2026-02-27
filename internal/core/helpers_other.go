//go:build !linux && !darwin && !freebsd

package core

// syncDir is a no-op on Windows since directory sync causes "Access is denied"
func syncDir(dirPath string) error {
	// On Windows, syncing directories is not supported and causes errors
	// The file sync is sufficient for our purposes
	return nil
}
