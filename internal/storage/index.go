package storage

import (
	"encoding/hex"
	"os"
	"sort"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
)

const IndexPath = ".kitcat/index"

func LoadIndex() (map[string]plumbing.IndexEntry, error) {
	// Read-only operations might not strict locking for MVP,
	// but writing absolutely does.
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

// UpdateIndex handles the transaction, converting the map back to a sorted slice.
func UpdateIndex(fn func(index map[string]plumbing.IndexEntry) error) error {
	// FIX: Re-enable locking mechanism
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

// WriteIndex is used by legacy tests or simple overwrites
func WriteIndex(simpleMap map[string]string) error {
	return WriteIndexFromTree(simpleMap)
}

func WriteIndexFromTree(tree map[string]string) error {
	// FIX: Re-enable locking mechanism
	l, err := lock(IndexPath)
	if err != nil {
		return err
	}
	defer unlock(l)

	indexMap := make(map[string]plumbing.IndexEntry)
	for path, hash := range tree {
		hb, _ := HexToHash(hash)
		indexMap[path] = plumbing.IndexEntry{
			Path: path,
			Hash: hb,
			Mode: 0100644,
		}
	}
	return writeMapToDisk(indexMap)
}

func writeMapToDisk(indexMap map[string]plumbing.IndexEntry) error {
	var entries []plumbing.IndexEntry
	for _, e := range indexMap {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	if err := os.MkdirAll(".kitcat", 0755); err != nil {
		return err
	}
	return plumbing.UpdateIndex(entries, IndexPath)
}

func HexToHash(s string) ([20]byte, error) {
	var out [20]byte
	slice, err := hex.DecodeString(s)
	if err != nil {
		return out, err
	}
	copy(out[:], slice)
	return out, nil
}
