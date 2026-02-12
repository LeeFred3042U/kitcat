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

// UpdateIndex writes the index to disk
func UpdateIndex(entries []IndexEntry, indexPath string) error {
	var buf bytes.Buffer

	// Header
	if _, err := buf.Write([]byte("DIRC")); err != nil { return err }
	if err := binary.Write(&buf, binary.BigEndian, uint32(2)); err != nil { return err }
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(entries))); err != nil { return err }

	// Entries
	for _, e := range entries {
		if err := writeEntry(&buf, e); err != nil {
			return err
		}
	}

	// Checksum
	sum := sha1.Sum(buf.Bytes())
	if _, err := buf.Write(sum[:]); err != nil { return err }

	return os.WriteFile(indexPath, buf.Bytes(), 0644)
}

func writeEntry(buf *bytes.Buffer, e IndexEntry) error {
	if err := binary.Write(buf, binary.BigEndian, e.CTimeSec); err != nil { return err }
	if err := binary.Write(buf, binary.BigEndian, e.CTimeNSec); err != nil { return err }
	if err := binary.Write(buf, binary.BigEndian, e.MTimeSec); err != nil { return err }
	if err := binary.Write(buf, binary.BigEndian, e.MTimeNSec); err != nil { return err }
	if err := binary.Write(buf, binary.BigEndian, e.Dev); err != nil { return err }
	if err := binary.Write(buf, binary.BigEndian, e.Ino); err != nil { return err }
	if err := binary.Write(buf, binary.BigEndian, e.Mode); err != nil { return err }
	if err := binary.Write(buf, binary.BigEndian, e.UID); err != nil { return err }
	if err := binary.Write(buf, binary.BigEndian, e.GID); err != nil { return err }
	if err := binary.Write(buf, binary.BigEndian, e.Size); err != nil { return err }
	
	if _, err := buf.Write(e.Hash[:]); err != nil { return err }

	nameLen := len(e.Path)
	if nameLen > 0xFFF {
		nameLen = 0xFFF
	}
	if err := binary.Write(buf, binary.BigEndian, uint16(nameLen)); err != nil { return err }

	if _, err := buf.WriteString(e.Path); err != nil { return err }
	if err := buf.WriteByte(0); err != nil { return err } // Null terminator

	// Padding
	entrySize := 62 + len(e.Path) + 1
	pad := 8 - (entrySize % 8)
	for i := 0; i < pad; i++ {
		if err := buf.WriteByte(0); err != nil { return err }
	}
	return nil
}
