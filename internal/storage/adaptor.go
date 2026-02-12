package storage

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/models"
	"github.com/LeeFred3042U/kitcat/internal/plumbing"
)

// ObjectsDir is where objects are stored
const ObjectsDir = ".kitcat/objects"

var ErrNoCommits = errors.New("no commits found")

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

	nullIdx := bytes.IndexByte(raw, 0)
	if nullIdx == -1 {
		return nil, fmt.Errorf("malformed object %s", hash)
	}

	return raw[nullIdx+1:], nil
}

// GetRef reads a reference file content
func GetRef(name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(".kitcat", name))
	return strings.TrimSpace(string(b)), err
}

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

func GetLastCommit() (models.Commit, error) {
	head, err := os.ReadFile(".kitcat/HEAD")
	if os.IsNotExist(err) {
		return models.Commit{}, ErrNoCommits
	}
	if err != nil {
		return models.Commit{}, err
	}

	ref := strings.TrimSpace(string(head))
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

func FindCommit(hash string) (models.Commit, error) {
	raw, err := ReadObject(hash)
	if err != nil {
		return models.Commit{}, err
	}
	return parseCommit(hash, raw)
}

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
			// Simple author parsing: "Name <email> timestamp"
			// Just taking the whole string for now to avoid parsing complexity here
			c.AuthorName = parts[1]
		}
	}
	if i < len(lines) {
		c.Message = strings.Join(lines[i+1:], "\n")
	}
	return c, nil
}

// HashFile hashes a file on disk without writing it
func HashFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return plumbing.HashAndWriteObject(content, "blob")
}

// FindMergeBase finds the common ancestor (simple version).
func FindMergeBase(h1, h2 string) (string, error) {
	// Simple traversal: Get all ancestors of h1, then walk h2 up until match
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
