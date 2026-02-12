package storage

import (
	"bufio"
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
)

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
		// Convert hash [20]byte to hex string
		hashStr := fmt.Sprintf("%x", entry.Hash)
		treeContent.WriteString(fmt.Sprintf("%s %s\n", hashStr, path))
	}

	// Use plumbing to hash and write the tree object
	return plumbing.HashAndWriteObject(treeContent.Bytes(), "tree")
}

func ParseTree(hash string) (map[string]string, error) {
	tree := make(map[string]string)

	// Use ReadObject to properly locate and decompress the object
	data, err := ReadObject(hash)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			tree[parts[1]] = parts[0]
		}
	}
	return tree, scanner.Err()
}
