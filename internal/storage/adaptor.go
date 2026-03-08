package storage

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/LeeFred3042U/kitcat/internal/models"
	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/repo"
)

// ErrNoCommits indicates that the repository does not yet contain
// any commit history. It is returned when HEAD cannot be resolved
// to a valid commit reference.
var ErrNoCommits = errors.New("no commits found")

// ReadObject locates, decompresses, and returns the raw payload of an
// object stored in the repository object database.
//
// Objects are stored in a Git-style format under repo.ObjectsDir using
// a fan-out directory structure where the first two hex characters of
// the hash form the directory and the remainder form the filename.
//
// The stored object format is:
//
//	"<type> <size>\0<payload>"
//
// This function removes the header and returns only the payload.
func ReadObject(hash string) ([]byte, error) {
	if len(hash) < 2 {
		return nil, fmt.Errorf("invalid hash: %s", hash)
	}
	path := filepath.Join(repo.ObjectsDir, hash[:2], hash[2:])
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

// GetRef reads the content of a reference file located within the
// repository directory and returns the trimmed reference value.
//
// References typically contain commit hashes or symbolic references
// such as "ref: refs/heads/main".
func GetRef(name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(repo.Dir, name))
	return strings.TrimSpace(string(b)), err
}

// ReadCommits walks commit history starting from the current HEAD and
// returns a linear slice of commits following parent pointers.
//
// Traversal stops when:
//   - A commit has no parent (root commit)
//   - A referenced parent cannot be resolved
//   - A previously seen commit ID appears (cycle protection)
//
// Cycle detection protects against corrupted commit graphs that could
// otherwise produce infinite loops.
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

// GetLastCommit resolves the repository HEAD reference to the most
// recent commit object.
//
// HEAD may contain either:
//
//   - A direct commit hash
//   - A symbolic reference such as "ref: refs/heads/main"
//
// Symbolic references are resolved by reading the referenced file
// inside the repository directory.
func GetLastCommit() (models.Commit, error) {
	head, err := os.ReadFile(repo.HeadPath)
	if os.IsNotExist(err) {
		return models.Commit{}, ErrNoCommits
	}
	if err != nil {
		return models.Commit{}, err
	}

	ref := strings.TrimSpace(string(head))

	// Resolve symbolic refs like "ref: refs/heads/main".
	if trimmed, ok := strings.CutPrefix(ref, "ref: "); ok {
		data, err := os.ReadFile(filepath.Join(repo.Dir, trimmed))
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

// FindCommit loads a commit object by hash from the object database
// and parses its contents into a models.Commit structure.
func FindCommit(hash string) (models.Commit, error) {
	raw, err := ReadObject(hash)
	if err != nil {
		return models.Commit{}, err
	}
	return parseCommit(hash, raw)
}

// parseCommit parses the textual payload of a commit object and
// extracts structured metadata into a models.Commit instance.
//
// The commit format follows Git's canonical layout:
//
//	tree <hash>
//	parent <hash>
//	author Name <email> timestamp timezone
//
//	<commit message>
//
// Only the fields required by the higher-level model are extracted.
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
				// Fallback if the author line is malformed.
				c.AuthorName = line
			}
		}
	}

	if i < len(lines) {
		c.Message = strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
	}
	return c, nil
}

// HashFile reads a file from disk, creates a blob object from its
// contents, stores it in the object database, and returns the resulting
// object hash.
func HashFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return plumbing.HashAndWriteObject(content, "blob")
}

// FindMergeBase performs a simple ancestry traversal to locate the first
// common ancestor between two commits.
//
// The algorithm records all ancestors of the first commit and then walks
// the ancestry of the second commit until a matching commit ID is found.
//
// Limitations:
//   - Assumes linear history (single parent)
//   - Does not perform full DAG merge-base computation
//   - Stops traversal when a commit cannot be resolved
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
