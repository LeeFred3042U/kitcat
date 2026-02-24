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

// IndexEntry is the in-memory representation of a single entry from the
// Git index (DIRC) file. It combines the logical Git fields needed to
// construct trees/commits with filesystem metadata cached to avoid
// re-hashing unchanged files.
type IndexEntry struct {
	// ---- Logical (Git semantics) ----

	// Path is the file path relative to the repository root. Stored as
	// path separators matching Git (forward slashes).
	Path string

	// Hash is the 20-byte SHA-1 object ID of the blob this index entry
	// points to. Stored as raw binary for efficient serialization.
	Hash [20]byte

	// Mode encodes file type and permission bits (e.g. 0100644, 0100755,
	// 0120000 for symlink). Must be preserved when writing tree objects.
	Mode uint32

	// Stage is the Git merge stage (0..3). 0 means normal (not in a
	// conflicted merge state).
	Stage uint8

	// ---- Cached stat info (filesystem) ----

	// CTimeSec / CTimeNSec: inode change time (seconds, nanoseconds).
	// Used for quick index validity checks against on-disk files.
	CTimeSec  uint32
	CTimeNSec uint32

	// MTimeSec / MTimeNSec: modification time (seconds, nanoseconds).
	// Used together with size/dev/ino to detect modifications cheaply.
	MTimeSec  uint32
	MTimeNSec uint32

	// Device and inode identifiers to detect file identity across renames
	// or moves without relying solely on path comparisons.
	Dev uint32
	Ino uint32

	// UID / GID: file owner/group; present to fully mirror Git's index
	// stat cache and for portability checks where relevant.
	UID uint32
	GID uint32

	// Size is the file size in bytes. Combined with timestamps and device/inode
	// it provides a fast path to skip expensive blob hashing when unchanged.
	Size uint32
}
