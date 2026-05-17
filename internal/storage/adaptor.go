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
	_, after, ok := bytes.Cut(raw, []byte{0})
	if !ok {
		return nil, fmt.Errorf("malformed object %s", hash)
	}

	return after, nil
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
			c.Parents = append(c.Parents, parts[1])
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

// HashAndStageBlob reads the file at path (or the symlink target if path is a
// symlink), writes its content as a "blob" object into the object store, and
// returns the 40-character hex SHA-1 of that object. It both hashes AND
// persists — callers that only want the hash without writing should not use
// this function.
func HashAndStageBlob(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}

	var content []byte
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		content = []byte(target)
	} else {
		content, err = os.ReadFile(path)
		if err != nil {
			return "", err
		}
	}
	return plumbing.HashAndWriteObject(content, "blob")
}

func collectAncestors(start string) (map[string]bool, error) {
	visited := make(map[string]bool)
	queue := []string{start}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if visited[curr] {
			continue
		}
		visited[curr] = true

		c, err := FindCommit(curr)
		if err != nil {
			return nil, err
		}

		for _, p := range c.Parents {
			if !visited[p] {
				queue = append(queue, p)
			}
		}
	}

	return visited, nil
}

func FindMergeBases(h1, h2 string) ([]string, error) {
	anc1, err := collectAncestors(h1)
	if err != nil {
		return nil, err
	}
	anc2, err := collectAncestors(h2)
	if err != nil {
		return nil, err
	}

	var common []string
	for sha := range anc1 {
		if anc2[sha] {
			common = append(common, sha)
		}
	}
	if len(common) == 0 {
		return nil, fmt.Errorf("no merge base found between %s and %s", h1[:8], h2[:8])
	}

	if len(common) == 1 {
		return common, nil
	}

	commonSet := make(map[string]bool, len(common))
	for _, sha := range common {
		commonSet[sha] = true
	}

	dominated := make(map[string]bool, len(common))

	for _, start := range common {
		if dominated[start] {
			continue
		}

		startCommit, err := FindCommit(start)
		if err != nil {
			continue
		}

		visited := map[string]bool{start: true}
		queue := make([]string, len(startCommit.Parents))
		copy(queue, startCommit.Parents)

		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]

			if visited[curr] {
				continue
			}
			visited[curr] = true

			if commonSet[curr] && curr != start {
				dominated[curr] = true
			}

			allFound := true
			for _, ca := range common {
				if !dominated[ca] && ca != start && !visited[ca] {
					allFound = false
					break
				}
			}
			if allFound {
				break
			}

			c, err := FindCommit(curr)
			if err != nil {
				continue
			}
			for _, p := range c.Parents {
				if !visited[p] {
					queue = append(queue, p)
				}
			}
		}
	}

	var result []string
	for _, sha := range common {
		if !dominated[sha] {
			result = append(result, sha)
		}
	}
	return result, nil
}
