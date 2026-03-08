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

// treeNode represents an intermediate in-memory directory tree used
// during tree object construction.
//
// The structure separates file entries from directory entries so the
// tree can be recursively hashed. This mirrors Git’s hierarchical tree
// object model where each directory is itself a tree object.
type treeNode struct {
	files map[string]treeEntry
	dirs  map[string]*treeNode
}

// WriteTree constructs a Git tree object from the repository index and
// writes the resulting object to the object database.
//
// The function reads index entries from disk, builds an in-memory
// directory tree, and recursively serializes it into Git tree objects.
// The resulting root tree hash is returned.
func WriteTree(indexPath string) (string, error) {
	indexEntries, err := ReadIndex(indexPath)
	if err != nil {
		return "", err
	}

	// Root node represents repository root; all paths are inserted relative to it.
	root := &treeNode{
		files: make(map[string]treeEntry),
		dirs:  make(map[string]*treeNode),
	}

	for _, e := range indexEntries {
		addToTree(root, e)
	}

	return writeTreeRecursive(root)
}

// addToTree inserts an IndexEntry into the in-memory tree structure.
//
// The path is split into components and intermediate directory nodes
// are created as needed so the resulting structure matches the logical
// repository directory hierarchy.
func addToTree(root *treeNode, e IndexEntry) {
	// Git paths always use forward slashes '/', regardless of the OS.
	// Using filepath.Separator would break compatibility on Windows.
	parts := strings.Split(e.Path, "/")
	node := root

	// Walk or create directory nodes so the tree structure mirrors index paths.
	// This ensures tree hashes match Git’s hierarchical object model.
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

	// Leaf filename is stored at the final directory level.
	filename := parts[len(parts)-1]
	node.files[filename] = treeEntry{
		mode: e.Mode,
		name: filename,
		hash: e.Hash,
	}
}

// writeTreeRecursive serializes a treeNode into a Git tree object.
//
// Child directories are recursively written first so their resulting
// tree hashes can be embedded into the parent tree entry. This ensures
// the Merkle-tree invariant required by Git’s object model.
func writeTreeRecursive(node *treeNode) (string, error) {
	var entries []treeEntry

	// Collect file entries first; directories are appended after recursive hashing.
	for _, f := range node.files {
		entries = append(entries, f)
	}

	// Recursively write subtrees so child hashes are available before
	// constructing the parent tree object (Merkle tree invariant).
	for name, dir := range node.dirs {
		treeHashHex, err := writeTreeRecursive(dir)
		if err != nil {
			return "", err
		}

		// Tree hashes are returned as hex strings; convert back to raw bytes
		// because tree object format stores binary SHA-1 values.
		hashBytes, err := HexToHash(treeHashHex)
		if err != nil {
			return "", fmt.Errorf("failed to convert tree hash for directory %s: %w", name, err)
		}

		var h [20]byte
		copy(h[:], hashBytes)

		entries = append(entries, treeEntry{
			mode: 0o40000, // Standard Git tree mode for directories
			name: name,
			hash: h,
		})
	}

	// Tree entries must be sorted lexicographically by name.
	// Unsorted entries would produce nondeterministic hashes.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	// Serialize entries using the Git tree object format:
	// "<mode> <name>\0<20-byte raw hash>"
	var buf bytes.Buffer
	for _, e := range entries {
		fmt.Fprintf(&buf, "%o %s\x00", e.mode, e.name)
		buf.Write(e.hash[:])
	}

	// Writing the object stores it in the object database and returns its hash.
	return HashAndWriteObject(buf.Bytes(), "tree")
}
