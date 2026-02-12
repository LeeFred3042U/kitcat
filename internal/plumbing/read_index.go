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
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os"
)

func ReadIndex(path string) ([]IndexEntry, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := bufio.NewReader(f)

	// Header
	var signature [4]byte
	if _, err := io.ReadFull(r, signature[:]); err != nil {
		return nil, err
	}
	if string(signature[:]) != "DIRC" {
		return nil, errors.New("invalid index signature")
	}

	var version, count uint32
	if err := binary.Read(r, binary.BigEndian, &version); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.BigEndian, &count); err != nil {
		return nil, err
	}

	entries := make([]IndexEntry, count)

	for i := 0; i < int(count); i++ {
		var e IndexEntry
		
		// Use a helper function that returns error to simplify checks
		read := func(data interface{}) error {
			return binary.Read(r, binary.BigEndian, data)
		}

		if err := read(&e.CTimeSec); err != nil { return nil, err }
		if err := read(&e.CTimeNSec); err != nil { return nil, err }
		if err := read(&e.MTimeSec); err != nil { return nil, err }
		if err := read(&e.MTimeNSec); err != nil { return nil, err }
		if err := read(&e.Dev); err != nil { return nil, err }
		if err := read(&e.Ino); err != nil { return nil, err }
		if err := read(&e.Mode); err != nil { return nil, err }
		if err := read(&e.UID); err != nil { return nil, err }
		if err := read(&e.GID); err != nil { return nil, err }
		if err := read(&e.Size); err != nil { return nil, err }

		if _, err := io.ReadFull(r, e.Hash[:]); err != nil {
			return nil, err
		}

		var flags uint16
		if err := binary.Read(r, binary.BigEndian, &flags); err != nil {
			return nil, err
		}
		
		// Read Name (null terminated)
		var nameBuf []byte
		for {
			b, err := r.ReadByte()
			if err != nil {
				return nil, err
			}
			if b == 0 {
				break
			}
			nameBuf = append(nameBuf, b)
		}
		e.Path = string(nameBuf)

		// Padding logic (1-8 null bytes to align to 8 bytes)
		// Entry size = 62 bytes fixed + len(path) + 1 (null)
		entrySize := 62 + len(nameBuf) + 1
		pad := 8 - (entrySize % 8)
		for j := 0; j < pad; j++ {
			if _, err := r.ReadByte(); err != nil {
				// End of file might be reached on last padding, strictly we should check
				if err == io.EOF {
					break
				}
				return nil, err
			}
		}

		entries[i] = e
	}

	// Try to read signature
	var extSig [4]byte
	if _, err := io.ReadFull(r, extSig[:]); err == nil {
		// If we successfully read a signature, read size and skip
		var extSize uint32
		if err := binary.Read(r, binary.BigEndian, &extSize); err == nil {
			// Skip content
			// Since r is bufio, we can't verify Seek easily without underlying file, 
			// but we can discard.
			if _, err := r.Discard(int(extSize)); err != nil {
				return nil, err
			}
		}
	}

	return entries, nil
}
