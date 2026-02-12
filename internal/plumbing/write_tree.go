// MIT License

// Copyright (c) [2025] [Zeeshan Ahmad Alavi]

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package plumbing

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// treeNode is a temporary structure to build the tree hierarchy in memory
type treeNode struct {
	files map[string]treeEntry
	dirs  map[string]*treeNode
}

func WriteTree(indexPath string) (string, error) {
	indexEntries, err := ReadIndex(indexPath)
	if err != nil {
		return "", err
	}

	root := &treeNode{
		files: make(map[string]treeEntry),
		dirs:  make(map[string]*treeNode),
	}

	for _, e := range indexEntries {
		addToTree(root, e)
	}

	return writeTreeRecursive(root)
}

func addToTree(root *treeNode, e IndexEntry) {
	// Git paths always use forward slashes '/', regardless of the OS.
	// filepath.Separator would break this on Windows.
	parts := strings.Split(e.Path, "/")
	node := root

	// Build directory hierarchy
	for i := 0; i < len(parts)-1; i++ {
		dir := parts[i]
		if node.dirs[dir] == nil {
			node.dirs[dir] = &treeNode{
				files: make(map[string]treeEntry),
				dirs:  make(map[string]*treeNode),
			}
		}
		node = node.dirs[dir]
	}

	// Add file to the specific directory node
	filename := parts[len(parts)-1]
	node.files[filename] = treeEntry{
		mode: e.Mode,
		name: filename,
		hash: e.Hash,
	}
}

func writeTreeRecursive(node *treeNode) (string, error) {
	var entries []treeEntry

	// 1. Collect files
	for _, f := range node.files {
		entries = append(entries, f)
	}

	// 2. Recursively process directories
	for name, dir := range node.dirs {
		treeHashHex, err := writeTreeRecursive(dir)
		if err != nil {
			return "", err
		}

		// Convert the hex string hash back to [20]byte using the safe helper
		hashBytes, err := HexToHash(treeHashHex)
		if err != nil {
			return "", fmt.Errorf("failed to convert tree hash for directory %s: %w", name, err)
		}

		var h [20]byte
		copy(h[:], hashBytes)

		entries = append(entries, treeEntry{
			mode: 040000, // Standard Git Tree Mode (directory)
			name: name,
			hash: h,
		})
	}

	// 3. Sort entries (required for deterministic tree hash)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	// 4. Write Content
	var buf bytes.Buffer
	for _, e := range entries {
		// Format: "%o %s\x00%s" -> mode name\0hash
		fmt.Fprintf(&buf, "%o %s\x00", e.mode, e.name)
		buf.Write(e.hash[:])
	}

	// 5. Store Object
	return HashAndWriteObject(buf.Bytes(), "tree")
}
