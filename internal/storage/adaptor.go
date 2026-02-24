package storage

import (
	"compress/zlib"
	"path/filepath"
	"strings"
	"strconv"
	"errors"
	"bytes"
	"time"
	"fmt"
	"io"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/models"
	"github.com/LeeFred3042U/kitcat/internal/plumbing"
)

// ObjectsDir defines the root directory for all stored objects.
const ObjectsDir = ".kitcat/objects"

var ErrNoCommits = errors.New("no commits found")

// ReadObject locates, decompresses, and returns the raw payload of an
// object by stripping the "<type> <size>\0" header used in storage.
func ReadObject(hash string) ([]byte, error) {
	if len(hash) < 2 {
		return nil, fmt.Errorf("invalid hash: %s", hash)
	}
	path := filepath.Join(ObjectsDir, hash[:2], hash[2:])
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r, err := zlib.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// Object header ends at the first null byte; payload follows immediately.
	nullIdx := bytes.IndexByte(raw, 0)
	if nullIdx == -1 {
		return nil, fmt.Errorf("malformed object %s", hash)
	}

	return raw[nullIdx+1:], nil
}

// GetRef reads the content of a reference file and trims whitespace.
func GetRef(name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(".kitcat", name))
	return strings.TrimSpace(string(b)), err
}

// ReadCommits walks commit history starting from HEAD and returns a linear
// slice of commits by following parent links until root or cycle detection.
func ReadCommits() ([]models.Commit, error) {
	head, err := GetLastCommit()
	if err != nil {
		if err == ErrNoCommits {
			return nil, nil
		}
		return nil, err
	}

	var commits []models.Commit
	curr := head
	seen := make(map[string]bool)

	for {
		// Prevent infinite loops if history becomes cyclic due to corruption.
		if seen[curr.ID] {
			break
		}
		seen[curr.ID] = true

		commits = append(commits, curr)
		if curr.Parent == "" {
			break
		}

		curr, err = FindCommit(curr.Parent)
		if err != nil {
			break
		}
	}
	return commits, nil
}

// GetLastCommit resolves HEAD to a commit hash, supporting symbolic refs.
func GetLastCommit() (models.Commit, error) {
	head, err := os.ReadFile(".kitcat/HEAD")
	if os.IsNotExist(err) {
		return models.Commit{}, ErrNoCommits
	}
	if err != nil {
		return models.Commit{}, err
	}

	ref := strings.TrimSpace(string(head))

	// Resolve symbolic refs like "ref: refs/heads/main".
	if strings.HasPrefix(ref, "ref: ") {
		ref = strings.TrimPrefix(ref, "ref: ")
		data, err := os.ReadFile(".kitcat/" + ref)
		if err != nil {
			return models.Commit{}, ErrNoCommits
		}
		ref = strings.TrimSpace(string(data))
	}

	if ref == "" {
		return models.Commit{}, ErrNoCommits
	}
	return FindCommit(ref)
}

// FindCommit reads a commit object and parses its metadata into a model.
func FindCommit(hash string) (models.Commit, error) {
	raw, err := ReadObject(hash)
	if err != nil {
		return models.Commit{}, err
	}
	return parseCommit(hash, raw)
}

// parseCommit extracts metadata from a raw commit object payload.
func parseCommit(hash string, data []byte) (models.Commit, error) {
	s := string(data)
	lines := strings.Split(s, "\n")
	c := models.Commit{ID: hash}
	var i int

	for i = 0; i < len(lines); i++ {
		if lines[i] == "" {
			break
		}
		parts := strings.SplitN(lines[i], " ", 2)
		if len(parts) < 2 {
			continue
		}
		
		switch parts[0] {
		case "tree":
			c.TreeHash = parts[1]
		case "parent":
			c.Parent = parts[1]
		case "author":
			// Canonical format: Name <email> timestamp timezone
			line := parts[1]
			emailStart := strings.IndexByte(line, '<')
			emailEnd := strings.IndexByte(line, '>')
			
			if emailStart != -1 && emailEnd != -1 && emailEnd > emailStart {
				c.AuthorName = strings.TrimSpace(line[:emailStart])
				c.AuthorEmail = line[emailStart+1 : emailEnd]
				
				timeData := strings.TrimSpace(line[emailEnd+1:])
				timeParts := strings.Fields(timeData)
				if len(timeParts) >= 2 {
					if unixTime, err := strconv.ParseInt(timeParts[0], 10, 64); err == nil {
						c.Timestamp = time.Unix(unixTime, 0)
					}
				}
			} else {
				c.AuthorName = line // Fallback if malformed
			}
		}
	}

	if i < len(lines) {
		c.Message = strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
	}
	return c, nil
}

// HashFile reads a file from disk and stores it as a blob object,
// returning the resulting object hash.
func HashFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return plumbing.HashAndWriteObject(content, "blob")
}

// FindMergeBase performs a simple ancestry search to locate the first
// common ancestor between two commits. Assumes linear history and does
// not account for complex DAG traversal or multiple parents.
func FindMergeBase(h1, h2 string) (string, error) {
	// Record all ancestors of h1.
	ancestors := make(map[string]bool)

	curr := h1
	for curr != "" {
		ancestors[curr] = true
		c, err := FindCommit(curr)
		if err != nil {
			break
		}
		curr = c.Parent
	}

	// Walk ancestors of h2 until a match is found.
	curr = h2
	for curr != "" {
		if ancestors[curr] {
			return curr, nil
		}
		c, err := FindCommit(curr)
		if err != nil {
			break
		}
		curr = c.Parent
	}

	return "", fmt.Errorf("no merge base found")
}
