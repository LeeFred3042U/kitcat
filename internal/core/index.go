package core

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/constant"
)

// IndexEntry represents a file in the staging area for the legacy JSON index format.
// Unlike plumbing.IndexEntry, this stores only path and hash as strings.
type IndexEntry struct {
	Path string
	Hash string
}

// LoadIndex reads the JSON-based .kitcat/index file and reconstructs entries.
// Missing or empty files return an empty slice without error.
func LoadIndex() ([]IndexEntry, error) {
	data, err := os.ReadFile(constant.IndexPath)
	if os.IsNotExist(err) {
		return []IndexEntry{}, nil
	}
	if err != nil {
		return nil, err
	}

	// Empty index is treated as valid state.
	if len(data) == 0 {
		return []IndexEntry{}, nil
	}

	var entryMap map[string]string
	if err := json.Unmarshal(data, &entryMap); err != nil {
		// Intentionally hides JSON details to avoid leaking format internals.
		return nil, fmt.Errorf("index file corrupted")
	}

	var entries []IndexEntry
	for key, value := range entryMap {
		entries = append(entries, IndexEntry{Path: key, Hash: value})
	}
	return entries, nil
}

// SaveIndex serializes entries into a JSON map[path]hash and writes it to disk.
// The file is fully rewritten each time; no atomic write or locking is performed.
func SaveIndex(entries []IndexEntry) error {
	file, err := os.Create(constant.IndexPath)
	if err != nil {
		return err
	}
	defer file.Close()

	entryMap := make(map[string]string)
	for _, entry := range entries {
		entryMap[entry.Path] = entry.Hash
	}

	data, err := json.Marshal(entryMap)
	if err != nil {
		return err
	}

	// Write replaces existing content; partial writes can leave index corrupted.
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("unable to write to index")
	}
	return nil
}
