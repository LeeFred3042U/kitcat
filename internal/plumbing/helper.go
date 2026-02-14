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
	"fmt"
	"os"
)

// SafeWriteFile writes raw data to disk using the provided permissions.
// Despite the name, this currently performs a direct write and does not
// implement atomic replace or directory creation safeguards.
func SafeWriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

// HexToHash converts a 40-character hexadecimal SHA-1 string into its
// raw 20-byte representation. Strict length and per-byte parsing prevent
// malformed hashes from entering object logic.
func HexToHash(s string) ([]byte, error) {
	if len(s) != 40 {
		return nil, fmt.Errorf("invalid hash length: %d", len(s))
	}
	out := make([]byte, 20)
	for i := 0; i < 20; i++ {
		// Parse two hex characters per byte to maintain strict decoding
		// and fail fast on invalid characters.
		if _, err := fmt.Sscanf(s[i*2:i*2+2], "%02x", &out[i]); err != nil {
			return nil, fmt.Errorf("invalid hex at index %d: %w", i, err)
		}
	}
	return out, nil
}
