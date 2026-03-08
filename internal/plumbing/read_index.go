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

// ReadIndex reads a Git index (DIRC) file from disk and reconstructs the
// in-memory slice of IndexEntry records.
//
// The parser implements the Git index v2/v3 binary format and performs
// several safety checks during decoding:
//
//   - Validates the index header signature.
//   - Verifies the trailing SHA-1 checksum to detect corruption.
//   - Rejects unsupported index versions.
//   - Correctly consumes extended flag fields and padding.
//   - Safely skips extension blocks that may appear before the checksum.
//
// Version compatibility:
//   - Supported: v2 and v3
//   - Rejected: v4 (path prefix compression not implemented)
//
// The returned slice preserves the entry order found in the index file.
func ReadIndex(path string) ([]IndexEntry, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}

	// Minimum valid size:
	// Header (12 bytes) + checksum footer (20 bytes).
	if len(data) < 32 {
		return nil, errors.New("index file too small")
	}

	// Git stores a SHA-1 checksum of the entire index file (excluding
	// the checksum itself) as the final 20 bytes.
	toHash := data[:len(data)-20]
	expectedSum := data[len(data)-20:]

	actualSum := sha1.Sum(toHash)
	if !bytes.Equal(actualSum[:], expectedSum) {
		return nil, errors.New("index checksum mismatch: file corrupted or tampered")
	}

	// Validate header signature.
	if string(data[0:4]) != "DIRC" {
		return nil, errors.New("invalid index signature")
	}

	version := binary.BigEndian.Uint32(data[4:8])

	// Only versions 2 and 3 are supported.
	// Version 4 introduces prefix-compressed paths which require a
	// different decoding algorithm.
	if version < 2 || version > 3 {
		return nil, fmt.Errorf("unsupported index version %d (only v2/v3 supported)", version)
	}

	count := binary.BigEndian.Uint32(data[8:12])
	entries := make([]IndexEntry, count)

	offset := 12 // Start immediately after the header.

	// Parse index entries sequentially.
	for i := 0; i < int(count); i++ {
		// Ensure enough bytes remain for the fixed-width portion.
		if offset+62 > len(data) {
			return nil, errors.New("unexpected EOF in index entries")
		}

		var e IndexEntry

		// Read stat metadata fields in strict on-disk order.
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

		// Copy the 20-byte object ID.
		copy(e.Hash[:], data[offset:offset+20])
		offset += 20

		// Flags contain the path length and stage bits.
		flags := binary.BigEndian.Uint16(data[offset : offset+2])
		offset += 2

		// Extract merge stage (bits 12–13).
		e.Stage = uint8((flags >> 12) & 0x3)

		// Extended flags indicator (bit 0x4000).
		// If present, an additional 16-bit field follows.
		if (flags & 0x4000) != 0 {
			if offset+2 > len(data) {
				return nil, errors.New("unexpected EOF reading extended flags")
			}
			offset += 2
		}

		// Path is stored as a null-terminated string.
		idx := bytes.IndexByte(data[offset:], 0)
		if idx == -1 {
			return nil, errors.New("malformed index: path not null-terminated")
		}

		nameEnd := offset + idx
		e.Path = string(data[offset:nameEnd])

		// Advance past the null terminator.
		offset = nameEnd + 1

		// Entries are padded with null bytes so that their total size
		// aligns to an 8-byte boundary.
		baseSize := 62
		if (flags & 0x4000) != 0 {
			baseSize += 2
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

	// Extension blocks may appear after entries but before the checksum.
	// Unknown extensions are skipped safely by reading their declared size.
	for offset < len(data)-20 {
		if offset+8 > len(data)-20 {
			break
		}

		// Skip 4-byte extension signature.
		offset += 4

		// Read extension payload size.
		extSize := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4

		if offset+int(extSize) > len(data)-20 {
			return nil, errors.New("malformed index extension size")
		}

		// Skip extension payload.
		offset += int(extSize)
	}

	return entries, nil
}
