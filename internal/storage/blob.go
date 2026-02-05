package storage

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
)

const (
	objectsDir = ".kitcat/objects"
)

// computeFileHash computes the SHA-1 hash of a file at the given path.
// Returns the hash as a hexadecimal string and any error encountered.
func computeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func HashAndStoreFile(path string) (string, error) {
	hash, err := computeFileHash(path)
	if err != nil {
		return "", err
	}

	objPath := filepath.Join(objectsDir, hash)
	// ensure objects dir exists
	if err := os.MkdirAll(objectsDir, 0o755); err != nil {
		return "", err
	}

	if _, err := os.Stat(objPath); os.IsNotExist(err) {
		// write via tmp file — read file again for storage
		tmp := objPath + ".tmp"
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		out, err := os.Create(tmp)
		if err != nil {
			f.Close()
			return "", err
		}
		if _, err := io.Copy(out, f); err != nil {
			f.Close()
			out.Close()
			os.Remove(tmp)
			return "", err
		}
		f.Close()
		out.Close()
		if err := os.Rename(tmp, objPath); err != nil {
			os.Remove(tmp)
			return "", err
		}
	}
	return hash, nil
}

// Reads an object from the objects directory
func ReadObject(hash string) ([]byte, error) {
	objectPath := filepath.Join(objectsDir, hash)
	return os.ReadFile(objectPath)
}

// Computes the SHA-1 hash of a file's content
// does not store the file in the object database
func HashFile(path string) (string, error) {
	return computeFileHash(path)
}
