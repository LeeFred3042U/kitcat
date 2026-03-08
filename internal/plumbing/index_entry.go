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

// IndexEntry represents a single entry in the Git index (DIRC) file.
//
// The index acts as Git’s staging area, recording the mapping between
// repository paths and the blob objects that represent their contents.
// In addition to logical Git metadata, the index stores filesystem
// stat information to allow quick change detection without recomputing
// file hashes.
//
// This structure mirrors the on-disk layout used in Git index version 2/3,
// though only the fields necessary for object construction and change
// detection are exposed here.
type IndexEntry struct {
	// ---- Logical (Git semantics) ----

	// Path is the file path relative to the repository root.
	// Paths are stored using forward slashes to match Git’s
	// canonical path representation.
	Path string

	// Hash is the 20-byte SHA-1 object ID of the blob referenced
	// by this index entry. It is stored in raw binary form to
	// simplify serialization into tree and index formats.
	Hash [20]byte

	// Mode encodes the file type and permission bits using the
	// same representation as Git tree objects (for example
	// 0100644 for normal files or 0100755 for executables).
	Mode uint32

	// Stage represents the Git merge stage (0–3). A value of 0
	// indicates a normal index entry, while non-zero values
	// represent entries created during merge conflicts.
	Stage uint8

	// ---- Cached stat info (filesystem) ----

	// CTimeSec and CTimeNSec store the inode change time of the
	// file (seconds and nanoseconds). These values help determine
	// whether the file’s metadata has changed since the last
	// index update.
	CTimeSec  uint32
	CTimeNSec uint32

	// MTimeSec and MTimeNSec store the file modification time
	// (seconds and nanoseconds). Together with file size and
	// device identifiers they allow quick detection of changes
	// without rehashing file contents.
	MTimeSec  uint32
	MTimeNSec uint32

	// Dev and Ino store the device and inode identifiers for the
	// file. These values allow detection of file identity changes
	// even when paths are moved or renamed.
	Dev uint32
	Ino uint32

	// UID and GID record the user and group ownership of the file.
	// These fields are included to mirror Git’s index stat cache
	// and preserve filesystem metadata across operations.
	UID uint32
	GID uint32

	// Size stores the file size in bytes. Together with timestamps,
	// device ID, and inode number it provides a fast path for
	// determining whether a file needs to be rehashed.
	Size uint32
}
