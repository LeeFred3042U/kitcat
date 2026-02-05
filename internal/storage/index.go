package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const indexPath = ".kitcat/index"

// LoadIndex reads the .kitcat/index file (in JSON format) and returns it as a map
// It returns an empty map if the file doesn't exist, which is normal for a new repository
func LoadIndex() (map[string]string, error) {
	return loadIndexInternal()
}

func loadIndexInternal() (map[string]string, error) {
	index := make(map[string]string)

	content, err := os.ReadFile(indexPath)
	if os.IsNotExist(err) {
		// File doesn't exist, return empty index. This is not an error ^-^
		return index, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not read index file: %w", err)
	}

	// If the file is empty, avoid a JSON error.
	if len(content) == 0 {
		return index, nil
	}

	if err := json.Unmarshal(content, &index); err != nil {
		return nil, fmt.Errorf("could not parse index file: %w", err)
	}

	return index, nil
}

// UpdateIndex safely updates the index by locking it before reading.
// The callback function 'fn' is allowed to modify the index map.
// If 'fn' returns nil, the modified index is written to disk.
// If 'fn' returns an error, the operation is aborted and nothing is written.
func UpdateIndex(fn func(index map[string]string) error) error {
	// Ensure the parent directory (.kitcat) exists.
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		return err
	}

	// 1. Lock the file
	l, err := lock(indexPath)
	if err != nil {
		return err
	}
	defer unlock(l)

	// 2. Load the index (without locking, since we already hold the lock)
	index, err := loadIndexInternal()
	if err != nil {
		return err
	}

	// 3. Callback to modify the index
	if err := fn(index); err != nil {
		return err // Abort transaction
	}

	// 4. Write the updated index (without locking, as we hold it)
	return writeIndexInternal(index)
}

// WriteIndex writes the index map to the .kitcat/index file atomically using a JSON format
// It uses SafeWriteFile to ensure atomic and durable writes
func WriteIndex(index map[string]string) error {
	// Ensure the parent directory (.kitcat) exists.
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		return err
	}

	// Lock the file to prevent concurrent writes
	l, err := lock(indexPath)
	if err != nil {
		return err
	}
	defer unlock(l)

	return writeIndexInternal(index)
}

// writeIndexInternal writes the index without acquiring a lock.
// Caller must ensure the lock is held.
func writeIndexInternal(index map[string]string) error {
	// Marshal the index to JSON with indentation
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	// Use SafeWriteFile to atomically write the index
	return SafeWriteFile(indexPath, data, 0644)
}
