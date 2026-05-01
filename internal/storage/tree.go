package storage

import (
	"bufio"
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
)

// TreeEntry represents a single file entry in a tree snapshot.
//
// Mode is stored as an octal string (for example "100644" or "100755")
// matching Git-style tree metadata. Hash is the hexadecimal object ID
// of the blob referenced by the entry.
type TreeEntry struct {
	Mode string // Octal string (e.g., "100644")
	Hash string
}

// CreateTree constructs a tree object from the current repository index
// and writes it to object storage using a legacy flat text format
// (one "mode hash path\n" line per entry).
//
// DEPRECATED (M4): This function is unreachable from any production code path.
// plumbing.WriteTree (used everywhere else) writes proper binary Git tree
// objects; having two incompatible tree formats in the same repo is a silent
// landmine. ParseTree has a compatibility shim for the flat format, but new
// code must never call CreateTree. This function is preserved only for
// historical reference and will be removed in a future cleanup.
// Replace any new call sites with plumbing.WriteTree instead.
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

		// Store mode as %06o so file modes match Git conventions
		// such as 100644 or 100755.
		treeContent.WriteString(fmt.Sprintf("%06o %s %s\n", entry.Mode, hashStr, path))
	}

	return plumbing.HashAndWriteObject(treeContent.Bytes(), "tree")
}

// ParseTree reads a tree object from the object database and reconstructs
// a flattened map of file entries keyed by full path.
//
// The function supports two formats:
//
//  1. Legacy flat text format used by earlier versions of the repository
//     implementation.
//  2. Standard Git binary tree object format.
//
// Directory entries are recursively expanded so the resulting map contains
// only file paths mapped to their corresponding TreeEntry metadata.
func ParseTree(hash string) (map[string]TreeEntry, error) {
	tree := make(map[string]TreeEntry)
	err := parseTreeRecursive(hash, "", tree)
	return tree, err
}

// parseTreeRecursive loads a tree object and populates the provided map
// with entries found in the tree. Directory entries trigger recursive
// parsing so the final result is a flattened path map.
//
// prefix represents the current directory path during recursion.
func parseTreeRecursive(hash string, prefix string, tree map[string]TreeEntry) error {
	data, err := ReadObject(hash)
	if err != nil {
		return err
	}

	// Detect legacy flat text tree format written by the now-deprecated
	// CreateTree function. These objects contain newline-delimited text entries
	// and no null bytes. The heuristic is intentionally conservative:
	//
	//   - We additionally require that the first byte is a digit so that a
	//     single-entry binary tree whose path happens to contain '\n' but not
	//     '\x00' does not get misidentified as legacy text.
	//   - If no legacy repos exist, remove this entire branch; it adds overhead
	//     to every ParseTree call and can in theory still misfire on exotic
	//     binary trees (e.g. a path containing a newline followed immediately
	//     by a digit).
	//
	// TODO (M5): gate on a version marker in the repo config rather than
	// relying on content heuristics, then remove this branch entirely.
	if len(data) > 0 && data[0] >= '0' && data[0] <= '9' &&
		bytes.Contains(data, []byte("\n")) && !bytes.Contains(data, []byte("\x00")) {
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

	// Parse Git-standard binary tree format.
	// Each entry is encoded as:
	// "<mode> <name>\0<20-byte binary hash>"
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

		// Mode values representing directories trigger recursion
		// so nested tree objects are expanded into the flattened map.
		if mode == "40000" || mode == "040000" {
			if err := parseTreeRecursive(entryHash, fullPath, tree); err != nil {
				return err
			}
		} else {
			tree[fullPath] = TreeEntry{
				Mode: mode,
				Hash: entryHash,
			}
		}
	}
	return nil
}
