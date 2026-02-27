//go:build linux || darwin || freebsd

package core

import "os"

// syncDir syncs the directory on Unix-like systems
func syncDir(dirPath string) error {
	d, err := os.Open(dirPath)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
