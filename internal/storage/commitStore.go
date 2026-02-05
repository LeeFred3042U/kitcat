package storage

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/models"
)

var ErrNoCommits = errors.New("no commits yet")

const commitsPath = ".kitcat/commits.log"

// Appends commit as NDJSON
func AppendCommit(commit models.Commit) error {
	if err := os.MkdirAll(".kitcat", 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(commitsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	// Direct locking on the file descriptor
	if err := LockFile(f); err != nil {
		f.Close()
		return err
	}
	// Defer close to unlock
	defer f.Close()

	enc := json.NewEncoder(f)
	if err := enc.Encode(commit); err != nil {
		return err
	}
	return f.Sync()
}

// Reads commits (NDJSON)
func ReadCommits() ([]models.Commit, error) {
	var commits []models.Commit
	if _, err := os.Stat(commitsPath); os.IsNotExist(err) {
		return commits, nil
	}

	f, err := os.Open(commitsPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var c models.Commit
		if err := json.Unmarshal(scanner.Bytes(), &c); err != nil {
			continue
		}
		commits = append(commits, c)
	}
	return commits, scanner.Err()
}

// Returns ErrNoCommits when none exist
func GetLastCommit() (models.Commit, error) {
	commits, err := ReadCommits()
	if err != nil {
		return models.Commit{}, err
	}
	if len(commits) == 0 {
		return models.Commit{}, ErrNoCommits
	}
	return commits[len(commits)-1], nil
}

// Search the commit log for a commit with a matching hash
// Supports both full hashes and short hashes (prefix matching)
func FindCommit(hash string) (models.Commit, error) {
	file, err := os.Open(commitsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return models.Commit{}, ErrNoCommits
		}
		return models.Commit{}, err
	}
	defer file.Close()

	var matches []models.Commit
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var commit models.Commit
		if err := json.Unmarshal(scanner.Bytes(), &commit); err != nil {
			continue
		}
		// Exact match (full hash)
		if commit.ID == hash {
			return commit, nil
		}
		// Prefix match (short hash)
		if strings.HasPrefix(commit.ID, hash) {
			matches = append(matches, commit)
		}
	}

	if err := scanner.Err(); err != nil {
		return models.Commit{}, err
	}

	// If we found exactly one prefix match, return it
	if len(matches) == 1 {
		return matches[0], nil
	}

	// If we found multiple matches, it's ambiguous
	if len(matches) > 1 {
		return models.Commit{}, fmt.Errorf(
			"ambiguous short hash %s (matches %d commits)",
			hash,
			len(matches),
		)
	}

	return models.Commit{}, fmt.Errorf("commit with hash %s not found", hash)
}

// IsAncestor returns true if ancestorHash is equal to or is an ancestor of descendantHash
func IsAncestor(ancestorHash, descendantHash string) (bool, error) {
	if ancestorHash == "" || descendantHash == "" {
		return false, nil
	}
	// A commit is its own ancestor
	if ancestorHash == descendantHash {
		return true, nil
	}

	current := descendantHash
	for current != "" {
		c, err := FindCommit(current)
		if err != nil {
			return false, err
		}
		if c.ID == ancestorHash {
			return true, nil
		}
		// walk up
		current = c.Parent
	}
	return false, nil
}

// FindMergeBase calculates the best common ancestor between two commits.
// Uses a simple ancestry path intersection (assumes linear/simple branching for now).
func FindMergeBase(hash1, hash2 string) (string, error) {
	if hash1 == hash2 {
		return hash1, nil
	}

	// Trace ancestry of hash1
	ancestors1 := make(map[string]bool)
	current := hash1
	for current != "" {
		ancestors1[current] = true
		c, err := FindCommit(current)
		if err != nil {
			return "", err
		}
		current = c.Parent
	}

	// Trace ancestry of hash2 and find first match
	current = hash2
	for current != "" {
		if ancestors1[current] {
			return current, nil
		}
		c, err := FindCommit(current)
		if err != nil {
			return "", err
		}
		current = c.Parent
	}

	return "", fmt.Errorf("no common ancestor found")
}
