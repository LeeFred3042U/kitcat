package storage

import (
	"encoding/hex"
	"fmt"
	"os"
	"sort"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
)

const IndexPath = ".kitcat/index"

// LoadIndex reads the plumbing index file and converts it into a
// path-keyed map for easier mutation by higher-level logic.
func LoadIndex() (map[string]plumbing.IndexEntry, error) {
	// Read-only operations avoid locking to keep reads cheap; writers must lock.
	entries, err := plumbing.ReadIndex(IndexPath)
	if os.IsNotExist(err) {
		return make(map[string]plumbing.IndexEntry), nil
	}
	if err != nil {
		return nil, err
	}
	indexMap := make(map[string]plumbing.IndexEntry, len(entries))
	for _, e := range entries {
		indexMap[e.Path] = e
	}
	return indexMap, nil
}

// UpdateIndex acquires an exclusive lock, exposes the index map to a
// mutation callback, and persists the result atomically.
func UpdateIndex(fn func(index map[string]plumbing.IndexEntry) error) error {
	// Writers must serialize access to prevent concurrent index corruption.
	l, err := lock(IndexPath)
	if err != nil {
		return err
	}
	defer unlock(l)

	indexMap, err := LoadIndex()
	if err != nil {
		return err
	}

	if err := fn(indexMap); err != nil {
		return err
	}

	return writeMapToDisk(indexMap)
}

// WriteIndex provides a compatibility wrapper used by older tests.
// Delegates to tree-based reconstruction logic.
func WriteIndex(simpleMap map[string]string) error {
	// Convert simple string map to TreeEntry map for compatibility
	complexMap := make(map[string]TreeEntry, len(simpleMap))
	for k, v := range simpleMap {
		complexMap[k] = TreeEntry{
			Hash: v,
			Mode: "100644", // Default to safe file mode
		}
	}
	return WriteIndexFromTree(complexMap)
}

// WriteIndexFromTree rebuilds the index from a tree snapshot by
// converting hashes into IndexEntry values.
func WriteIndexFromTree(tree map[string]TreeEntry) error {
	// Lock ensures only one writer modifies the index at a time.
	l, err := lock(IndexPath)
	if err != nil {
		return err
	}
	defer unlock(l)

	indexMap := make(map[string]plumbing.IndexEntry)
	for path, entry := range tree {
		hb, _ := HexToHash(entry.Hash)

		var mode uint32
		if _, err := fmt.Sscanf(entry.Mode, "%o", &mode); err != nil {
			mode = 0100644 // Fallback if mode parsing fails
		}

		indexMap[path] = plumbing.IndexEntry{
			Path: path,
			Hash: hb,
			Mode: mode,
		}
	}
	return writeMapToDisk(indexMap)
}

// writeMapToDisk converts the in-memory map into a deterministically
// ordered slice before delegating serialization to plumbing.UpdateIndex.
func writeMapToDisk(indexMap map[string]plumbing.IndexEntry) error {
	var entries []plumbing.IndexEntry
	for _, e := range indexMap {
		entries = append(entries, e)
	}

	// Stable ordering is required so index checksum remains deterministic.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	if err := os.MkdirAll(".kitcat", 0755); err != nil {
		return err
	}
	return plumbing.UpdateIndex(entries, IndexPath)
}

// HexToHash converts a hex SHA-1 string into a fixed-length [20]byte.
// Used when reconstructing index entries from tree snapshots.
func HexToHash(s string) ([20]byte, error) {
	var out [20]byte
	slice, err := hex.DecodeString(s)
	if err != nil {
		return out, err
	}
	copy(out[:], slice)
	return out, nil
}
