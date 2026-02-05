package storage

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

const stashPath = ".kitcat/stash.log"

var ErrNoStash = errors.New("no stash entries found")

// PushStash appends a commit ID to the stash stack (LIFO)
// The newest stash is at the end of the file
func PushStash(commitID string) error {
	if commitID == "" {
		return fmt.Errorf("commit ID cannot be empty")
	}

	if err := os.MkdirAll(".kitcat", 0o755); err != nil {
		return err
	}

	// Lock the file to prevent concurrent writes
	lockFile, err := lock(stashPath)
	if err != nil {
		return err
	}
	defer unlock(lockFile)

	f, err := os.OpenFile(stashPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write commit ID as a single line
	if _, err := fmt.Fprintln(f, commitID); err != nil {
		return err
	}

	return f.Sync()
}

// PopStash removes and returns the most recent stash commit ID
// Returns ErrNoStash if the stack is empty
func PopStash() (string, error) {
	stashes, err := ListStashes()
	if err != nil {
		return "", err
	}

	if len(stashes) == 0 {
		return "", ErrNoStash
	}

	// Get the most recent stash (first in the list)
	topStash := stashes[0]

	// Lock the file for writing
	lockFile, err := lock(stashPath)
	if err != nil {
		return "", err
	}
	defer unlock(lockFile)

	// Rewrite the file without the top stash
	tmpPath := stashPath + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}

	// Write all stashes except the first one (in reverse order to maintain file order)
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

	// Replace the original file
	if err := os.Rename(tmpPath, stashPath); err != nil {
		return "", err
	}

	return topStash, nil
}

// PeekStash returns the most recent stash commit ID without removing it
// Returns ErrNoStash if the stack is empty
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

// ListStashes returns all stash commit IDs in order (newest first)
// Returns an empty slice if no stashes exist
func ListStashes() ([]string, error) {
	if _, err := os.Stat(stashPath); os.IsNotExist(err) {
		return []string{}, nil
	}

	f, err := os.Open(stashPath)
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

	// Reverse the order so newest is first
	for i, j := 0, len(stashes)-1; i < j; i, j = i+1, j-1 {
		stashes[i], stashes[j] = stashes[j], stashes[i]
	}

	return stashes, nil
}

// ClearStash removes all stash entries by truncating the stash file to size 0.
func ClearStash() error {
	// If the file doesn't exist, nothing to clear
	if _, err := os.Stat(stashPath); os.IsNotExist(err) {
		return nil
	}

	// Lock the file to prevent concurrent writes
	lockFile, err := lock(stashPath)
	if err != nil {
		return err
	}
	defer unlock(lockFile)

	// Truncate the file to size 0
	return os.Truncate(stashPath, 0)
}
