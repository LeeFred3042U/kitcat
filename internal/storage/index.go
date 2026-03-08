package storage

import (
	"encoding/hex"
	"fmt"
	"os"
	"sort"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/repo"
)

// LoadIndex reads the repository index file from disk and converts the
// underlying slice of plumbing.IndexEntry values into a path-keyed map.
//
// The returned map is optimized for mutation by higher-level storage
// operations that need efficient lookup or modification of entries by
// path. If the index file does not yet exist, an empty map is returned.
func LoadIndex() (map[string]plumbing.IndexEntry, error) {
	// Read-only operations avoid locking to keep reads cheap; writers must lock.
	entries, err := plumbing.ReadIndex(repo.IndexPath)
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

// UpdateIndex acquires an exclusive index lock, exposes the in-memory
// index map to the provided mutation callback, and then persists the
// resulting state back to disk.
//
// The callback receives the mutable map representation of the index.
// If the callback returns an error, the update is aborted and the
// index file is left unchanged.
func UpdateIndex(fn func(index map[string]plumbing.IndexEntry) error) error {
	// Writers must serialize access to prevent concurrent index corruption.
	l, err := lock(repo.IndexPath)
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

// WriteIndex is a compatibility helper that converts a simplified
// path-to-hash map into the richer TreeEntry representation used by
// tree reconstruction logic.
//
// This function exists primarily to support older tests and legacy
// call sites that operate on simple string mappings.
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

// WriteIndexFromTree rebuilds the index file from a flattened tree
// snapshot representation.
//
// Each TreeEntry is converted into a plumbing.IndexEntry, reconstructing
// the necessary fields such as the raw hash bytes and file mode. The
// resulting index is then written to disk in canonical order.
func WriteIndexFromTree(tree map[string]TreeEntry) error {
	// Lock ensures only one writer modifies the index at a time.
	l, err := lock(repo.IndexPath)
	if err != nil {
		return err
	}
	defer unlock(l)

	indexMap := make(map[string]plumbing.IndexEntry)
	for path, entry := range tree {
		hb, _ := HexToHash(entry.Hash)

		var mode uint32
		if _, err := fmt.Sscanf(entry.Mode, "%o", &mode); err != nil {
			mode = 0o100644 // Fallback if mode parsing fails
		}

		indexMap[path] = plumbing.IndexEntry{
			Path: path,
			Hash: hb,
			Mode: mode,
		}
	}
	return writeMapToDisk(indexMap)
}

// writeMapToDisk converts the in-memory map representation of the index
// into a deterministically ordered slice before delegating serialization
// to plumbing.UpdateIndex.
//
// Deterministic ordering is required so that the resulting index file
// produces a stable checksum and consistent object state.
func writeMapToDisk(indexMap map[string]plumbing.IndexEntry) error {
	var entries []plumbing.IndexEntry
	for _, e := range indexMap {
		entries = append(entries, e)
	}

	// Stable ordering is required so index checksum remains deterministic.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	if err := os.MkdirAll(repo.Dir, 0o755); err != nil {
		return err
	}
	return plumbing.UpdateIndex(entries, repo.IndexPath)
}

// HexToHash converts a hexadecimal SHA-1 string into its fixed-length
// [20]byte representation used by plumbing.IndexEntry.
//
// The function decodes the hex string into raw bytes and copies the
// result into a fixed-size array suitable for use in index structures.
func HexToHash(s string) ([20]byte, error) {
	var out [20]byte
	slice, err := hex.DecodeString(s)
	if err != nil {
		return out, err
	}
	copy(out[:], slice)
	return out, nil
}
