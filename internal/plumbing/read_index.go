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
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

// ReadIndex parses a Git index file from disk and reconstructs
// in-memory IndexEntry records from the binary format.
//
// Compatibility:
//   - Supports Git Index Version 2 and 3.
//   - Version 4 (prefix-compressed paths) is intentionally rejected.
//   - Verifies SHA-1 checksum to prevent silent corruption.
//   - Correctly consumes extended flags and padding to maintain alignment.
//   - Safely skips unknown extension blocks.
func ReadIndex(path string) ([]IndexEntry, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}

	// Minimum valid size:
	// Header (12 bytes) + Checksum footer (20 bytes)
	if len(data) < 32 {
		return nil, errors.New("index file too small")
	}

	// Checksum Verification
	// Git appends a SHA-1 checksum of the entire file (excluding the footer).
	toHash := data[:len(data)-20]
	expectedSum := data[len(data)-20:]

	actualSum := sha1.Sum(toHash)
	if !bytes.Equal(actualSum[:], expectedSum) {
		return nil, errors.New("index checksum mismatch: file corrupted or tampered")
	}

	// Parse Header
	if string(data[0:4]) != "DIRC" {
		return nil, errors.New("invalid index signature")
	}

	version := binary.BigEndian.Uint32(data[4:8])

	// Only V2 and V3 are supported.
	// V4 introduces path compression which cannot be decoded safely
	// without a dedicated implementation.
	if version < 2 || version > 3 {
		return nil, fmt.Errorf("unsupported index version %d (only v2/v3 supported)", version)
	}

	count := binary.BigEndian.Uint32(data[8:12])
	entries := make([]IndexEntry, count)

	offset := 12 // Start immediately after header

	// Parse Entries
	for i := 0; i < int(count); i++ {
		// Ensure enough bytes remain for fixed-width metadata.
		if offset+62 > len(data) {
			return nil, errors.New("unexpected EOF in index entries")
		}

		var e IndexEntry

		// Stat metadata fields must be read in strict order.
		e.CTimeSec = binary.BigEndian.Uint32(data[offset : offset+4])
		e.CTimeNSec = binary.BigEndian.Uint32(data[offset+4 : offset+8])
		e.MTimeSec = binary.BigEndian.Uint32(data[offset+8 : offset+12])
		e.MTimeNSec = binary.BigEndian.Uint32(data[offset+12 : offset+16])
		e.Dev = binary.BigEndian.Uint32(data[offset+16 : offset+20])
		e.Ino = binary.BigEndian.Uint32(data[offset+20 : offset+24])
		e.Mode = binary.BigEndian.Uint32(data[offset+24 : offset+28])
		e.UID = binary.BigEndian.Uint32(data[offset+28 : offset+32])
		e.GID = binary.BigEndian.Uint32(data[offset+32 : offset+36])
		e.Size = binary.BigEndian.Uint32(data[offset+36 : offset+40])

		offset += 40

		// Copy 20-byte object hash.
		copy(e.Hash[:], data[offset:offset+20])
		offset += 20

		// Flags contain stage bits and optional extended flag indicator.
		flags := binary.BigEndian.Uint16(data[offset : offset+2])
		offset += 2

		// Extract merge stage (0–3).
		e.Stage = uint8((flags >> 12) & 0x3)

		
		// Handle Extended Flags		
		// If bit 0x4000 is set, an additional 16-bit flag field follows.
		// We ignore the contents but MUST consume it to maintain alignment.
		if (flags & 0x4000) != 0 {
			if offset+2 > len(data) {
				return nil, errors.New("unexpected EOF reading extended flags")
			}
			offset += 2
		}

		
		// Parse Path (Null-terminated)		
		idx := bytes.IndexByte(data[offset:], 0)
		if idx == -1 {
			return nil, errors.New("malformed index: path not null-terminated")
		}

		nameEnd := offset + idx
		e.Path = string(data[offset:nameEnd])

		// Move past the null byte.
		offset = nameEnd + 1

		// Handle Entry Padding
		// Entry size must be aligned to an 8-byte boundary.
		baseSize := 62
		if (flags & 0x4000) != 0 {
			baseSize += 2 // extended flag field
		}

		currentEntryLen := baseSize + len(e.Path) + 1
		pad := 8 - (currentEntryLen % 8)
		if pad != 8 {
			if offset+pad > len(data) {
				return nil, errors.New("malformed index: padding out of bounds")
			}
			offset += pad
		}

		entries[i] = e
	}

	// Parse Extensions (Skip Safely)
	// Extension blocks exist between entries and the checksum footer.
	for offset < len(data)-20 {
		if offset+8 > len(data)-20 {
			break // Not enough space for a valid extension header
		}

		// Signature (ignored)
		offset += 4

		// Size
		extSize := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		if offset+int(extSize) > len(data)-20 {
			return nil, errors.New("malformed index extension size")
		}

		// Skip payload
		offset += int(extSize)
	}

	return entries, nil
}
