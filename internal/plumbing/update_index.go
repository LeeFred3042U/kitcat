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
	"os"
)

// UpdateIndex serializes the provided IndexEntry slice into the Git
// index (DIRC) binary format and writes it to disk.
//
// The function produces a version 2 index file consisting of:
//
//   - A header containing the "DIRC" signature, index version, and entry count.
//   - A sequence of serialized index entries.
//   - A trailing SHA-1 checksum computed over all previous bytes.
//
// The checksum ensures index integrity and allows readers to detect
// corruption or tampering when loading the file.
func UpdateIndex(entries []IndexEntry, indexPath string) error {
	var buf bytes.Buffer

	// Write index header: signature, version, and entry count.
	if _, err := buf.Write([]byte("DIRC")); err != nil {
		return err
	}
	if err := binary.Write(&buf, binary.BigEndian, uint32(2)); err != nil {
		return err
	}
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(entries))); err != nil {
		return err
	}

	// Entries must be written sequentially so checksum reflects exact byte layout.
	for _, e := range entries {
		if err := writeEntry(&buf, e); err != nil {
			return err
		}
	}

	// Git index checksum is SHA-1 over all prior bytes.
	sum := sha1.Sum(buf.Bytes())
	if _, err := buf.Write(sum[:]); err != nil {
		return err
	}

	// Final write replaces index file contents.
	return os.WriteFile(indexPath, buf.Bytes(), 0o644)
}

// writeEntry serializes a single index entry to the buffer using
// the Git index v2 binary layout
//
// Layout:
//   [stat data][object id][flags][path][NUL][padding]
//
// Flags:
//   - lower 12 bits: truncated path length
//   - upper 2 bits: stage (merge state)
//
// Entries are padded to 8-byte alignment
func writeEntry(buf *bytes.Buffer, e IndexEntry) error {
	// statBlock mirrors the fixed-width stat section of a Git index v2 entry
	// Field order and sizes must match the on-disk specification exactly
	type statBlock struct {
		CTimeSec  uint32
		CTimeNSec uint32
		MTimeSec  uint32
		MTimeNSec uint32
		Dev       uint32
		Ino       uint32
		Mode      uint32
		UID       uint32
		GID       uint32
		Size      uint32
	}

	// Populate stat block from index entry
	// This isolates fixed-size metadata from variable-length fields
	sb := statBlock{
		e.CTimeSec,
		e.CTimeNSec,
		e.MTimeSec,
		e.MTimeNSec,
		e.Dev,
		e.Ino,
		e.Mode,
		e.UID,
		e.GID,
		e.Size,
	}

	// Serialize stat block in big-endian order as required by Git index format
	if err := binary.Write(buf, binary.BigEndian, sb); err != nil {
		return err
	}

	// Write object ID (20 bytes)
	if _, err := buf.Write(e.Hash[:]); err != nil {
		return err
	}

	// Encode flags:
	//   bits 12–13: stage (0–3)
	//   bits 0–11 : path length (capped at 0xFFF)
	nameLen := len(e.Path)
	if nameLen > 0xFFF {
		nameLen = 0xFFF
	}
	stage := uint16(e.Stage & 0x3)
	length := uint16(nameLen)
	if length > 0x0FFF {
		length = 0x0FFF
	}
	
	flags := (stage << 12) | (length & 0x0FFF)

	if err := binary.Write(buf, binary.BigEndian, flags); err != nil {
		return err
	}

	// Write path (NUL-terminated).
	if _, err := buf.WriteString(e.Path); err != nil {
		return err
	}
	if err := buf.WriteByte(0); err != nil {
		return err
	}

	// Pad entry to 8-byte boundary.
	// Base entry size (without path) is 62 bytes.
	entrySize := 62 + len(e.Path) + 1
	pad := (8 - (entrySize % 8)) % 8

	for i := 0; i < pad; i++ {
		if err := buf.WriteByte(0); err != nil {
			return err
		}
	}

	return nil
}
