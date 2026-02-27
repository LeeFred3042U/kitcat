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
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
)

// HashAndWriteObject constructs a Git-style object, computes its SHA-1,
// and stores it in the object database using content-addressed storage.
// Supports all object types (blob, tree, commit).
func HashAndWriteObject(content []byte, objType string) (string, error) {
	// Object header is part of the hash input; omitting it would produce
	// incompatible object IDs compared to Git.
	header := fmt.Sprintf("%s %d\x00", objType, len(content))
	store := append([]byte(header), content...)

	// Hash is computed over the full header+payload buffer.
	sum := sha1.Sum(store)
	hash := fmt.Sprintf("%x", sum)

	// Fan-out directory structure avoids too many files in a single directory
	dir := filepath.Join(".kitcat/objects", hash[:2])
	path := filepath.Join(dir, hash[2:])

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	// Objects are immutable; skip write if object already exists.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		var buf bytes.Buffer
		w := zlib.NewWriter(&buf)

		// Objects are stored compressed to reduce disk usage and match Git format.
		if _, err := w.Write(store); err != nil {
			return "", err
		}
		w.Close()

		if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
			return "", err
		}
	}

	return hash, nil
}
