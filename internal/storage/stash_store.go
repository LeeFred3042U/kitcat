package storage

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/repo"
)

// ErrNoStash indicates that a stash operation requiring an existing entry
// was attempted while the stash stack is empty.
var ErrNoStash = errors.New("no stash entries found")

// getStashPath returns the absolute filesystem path of the stash log file.
//
// The stash is implemented as a simple append-only log located inside the
// repository directory. Each line represents a single commit ID.
func getStashPath() string {
	return filepath.Join(repo.Dir, "stash.log")
}

// PushStash appends a commit ID to the stash stack.
//
// The stash is stored as a newline-delimited file where the newest entry
// is appended to the end. Logical LIFO ordering is reconstructed during
// reads by reversing the file order.
func PushStash(commitID string) error {
	if commitID == "" {
		return fmt.Errorf("commit ID cannot be empty")
	}

	if err := os.MkdirAll(repo.Dir, 0o755); err != nil {
		return err
	}

	// Lock the file to prevent concurrent writes during the stash operation.
	lockFile, err := lock(getStashPath())
	if err != nil {
		return err
	}
	defer unlock(lockFile)

	f, err := os.OpenFile(getStashPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Each stash entry is stored as a single line containing the commit ID.
	if _, err := fmt.Fprintln(f, commitID); err != nil {
		return err
	}

	// Ensure the append is flushed to disk.
	return f.Sync()
}

// PopStash removes and returns the most recent stash entry.
//
// The stash stack is logically LIFO. Internally the newest entry appears
// at the end of the file, but ListStashes returns entries reversed so
// index 0 always represents the most recent stash.
func PopStash() (string, error) {
	stashes, err := ListStashes()
	if err != nil {
		return "", err
	}

	if len(stashes) == 0 {
		return "", ErrNoStash
	}

	// Get the most recent stash (first in the list).
	topStash := stashes[0]

	// Lock the file for writing to safely rewrite the stash log.
	lockFile, err := lock(getStashPath())
	if err != nil {
		return "", err
	}
	defer unlock(lockFile)

	// Rewrite the file without the top stash.
	tmpPath := getStashPath() + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}

	// Write remaining stashes back in original file order.
	for i := len(stashes) - 1; i > 0; i-- {
		if _, err := fmt.Fprintln(tmpFile, stashes[i]); err != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
			return "", err
		}
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", err
	}

	// Atomically replace the original stash file.
	if err := os.Rename(tmpPath, getStashPath()); err != nil {
		return "", err
	}

	return topStash, nil
}

// PeekStash returns the most recent stash entry without removing it
// from the stash stack.
func PeekStash() (string, error) {
	stashes, err := ListStashes()
	if err != nil {
		return "", err
	}

	if len(stashes) == 0 {
		return "", ErrNoStash
	}

	return stashes[0], nil
}

// ListStashes reads the stash log file and returns all stored commit IDs
// in last-in-first-out (LIFO) order.
//
// Internally the file stores entries oldest-to-newest. The returned slice
// is reversed so callers always see the most recent stash first.
func ListStashes() ([]string, error) {
	if _, err := os.Stat(getStashPath()); os.IsNotExist(err) {
		return []string{}, nil
	}

	f, err := os.Open(getStashPath())
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var stashes []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			stashes = append(stashes, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Reverse order so newest stash appears first.
	for i, j := 0, len(stashes)-1; i < j; i, j = i+1, j-1 {
		stashes[i], stashes[j] = stashes[j], stashes[i]
	}

	return stashes, nil
}

// ClearStash removes all stash entries by truncating the stash log file.
//
// If the stash file does not exist, the operation is treated as a no-op.
func ClearStash() error {
	// If the file doesn't exist, nothing to clear.
	if _, err := os.Stat(getStashPath()); os.IsNotExist(err) {
		return nil
	}

	// Lock the file to prevent concurrent writes during truncation.
	lockFile, err := lock(getStashPath())
	if err != nil {
		return err
	}
	defer unlock(lockFile)

	// Truncate the file to size 0.
	return os.Truncate(getStashPath(), 0)
}


func DropStash(index int) error {
	stashes, err := ListStashes()
	if err != nil {
		return err
	}

	if len(stashes) == 0 {
		return ErrNoStash
	}

	if index < 0 || index >= len(stashes) {
		return fmt.Errorf("invalid stash index")
	}

	// Lock file
	lockFile, err := lock(getStashPath())
	if err != nil {
		return err
	}
	defer unlock(lockFile)

	tmpPath := getStashPath() + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	// Rewrite everything except the removed index
	for i := len(stashes) - 1; i >= 0; i-- {
		if i == index {
			continue
		}
		if _, err := fmt.Fprintln(tmpFile, stashes[i]); err != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
			return err
		}
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, getStashPath())
}
