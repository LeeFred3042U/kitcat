package storage

import (
	"bufio"
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
)

// TreeEntry represents a file in a tree object
type TreeEntry struct {
	Mode string // Octal string (e.g., "100644")
	Hash string
}

func CreateTree() (string, error) {
	index, err := LoadIndex()
	if err != nil {
		return "", err
	}

	var treeContent bytes.Buffer
	keys := make([]string, 0, len(index))
	for p := range index {
		keys = append(keys, p)
	}
	sort.Strings(keys)

	for _, path := range keys {
		entry := index[path]
		hashStr := fmt.Sprintf("%x", entry.Hash)
		// Store mode as %06o (format: 100644 or 100755)
		treeContent.WriteString(fmt.Sprintf("%06o %s %s\n", entry.Mode, hashStr, path))
	}

	return plumbing.HashAndWriteObject(treeContent.Bytes(), "tree")
}

// ParseTree reads a tree object (either legacy flat-text or Git-standard binary)
// and returns a flattened map of all file entries with their full paths.
func ParseTree(hash string) (map[string]TreeEntry, error) {
	tree := make(map[string]TreeEntry)
	err := parseTreeRecursive(hash, "", tree)
	return tree, err
}

func parseTreeRecursive(hash string, prefix string, tree map[string]TreeEntry) error {
	data, err := ReadObject(hash)
	if err != nil {
		return err
	}

	// Fast check for legacy flat text format (used by older object snapshots)
	if bytes.Contains(data, []byte("\n")) && !bytes.Contains(data, []byte("\x00")) {
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.SplitN(line, " ", 3)
			if len(parts) == 3 {
				tree[parts[2]] = TreeEntry{Mode: parts[0], Hash: parts[1]}
			} else if len(parts) == 2 {
				tree[parts[1]] = TreeEntry{Mode: "100644", Hash: parts[0]}
			}
		}
		return scanner.Err()
	}

	// Git-standard binary tree format parsing
	offset := 0
	for offset < len(data) {
		spaceIdx := bytes.IndexByte(data[offset:], ' ')
		if spaceIdx == -1 {
			return fmt.Errorf("malformed tree object: no space found")
		}
		mode := string(data[offset : offset+spaceIdx])
		offset += spaceIdx + 1

		nullIdx := bytes.IndexByte(data[offset:], 0)
		if nullIdx == -1 {
			return fmt.Errorf("malformed tree object: no null byte found")
		}
		name := string(data[offset : offset+nullIdx])
		offset += nullIdx + 1

		if offset+20 > len(data) {
			return fmt.Errorf("malformed tree object: truncated hash")
		}
		entryHash := fmt.Sprintf("%x", data[offset:offset+20])
		offset += 20

		fullPath := name
		if prefix != "" {
			fullPath = prefix + "/" + name
		}

		if mode == "40000" || mode == "040000" {
			// Recurse into subdirectory
			if err := parseTreeRecursive(entryHash, fullPath, tree); err != nil {
				return err
			}
		} else {
			// Add file to flat map
			tree[fullPath] = TreeEntry{
				Mode: mode,
				Hash: entryHash,
			}
		}
	}
	return nil
}
