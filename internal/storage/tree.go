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


func ParseTree(hash string) (map[string]TreeEntry, error) {
	tree := make(map[string]TreeEntry)
	err := parseTreeRecursive(hash, "", tree)
	return tree, err
}

func parseTreeRecursive(hash, prefix string, tree map[string]TreeEntry) error {
    data, err := ReadObject(hash)
    if err != nil { return err }

    offset := 0
    for offset < len(data) {
        spaceIdx := bytes.IndexByte(data[offset:], ' ')
        if spaceIdx == -1 { return fmt.Errorf("malformed tree: no space") }
        mode := string(data[offset : offset+spaceIdx])
        offset += spaceIdx + 1

        nullIdx := bytes.IndexByte(data[offset:], 0)
        if nullIdx == -1 { return fmt.Errorf("malformed tree: no null") }
        name := string(data[offset : offset+nullIdx])
        offset += nullIdx + 1

        if offset+20 > len(data) { return fmt.Errorf("malformed tree: short hash") }
        entryHash := fmt.Sprintf("%x", data[offset:offset+20])
        offset += 20

        fullPath := name
        if prefix != "" { fullPath = prefix + "/" + name }

        if mode == "40000" || mode == "040000" {
            if err := parseTreeRecursive(entryHash, fullPath, tree); err != nil {
                return err
            }
        } else {
            tree[fullPath] = TreeEntry{Mode: mode, Hash: entryHash}
        }
    }
    return nil
}
