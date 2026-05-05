package remote

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	
)

// Pack object type constants (types 6 and 7 are delta-encoded).
const (
	objOfsDelta = 6 // OBJ_OFS_DELTA  — base is at a negative offset in this packfile
	objRefDelta = 7 // OBJ_REF_DELTA  — base is identified by a 20-byte SHA-1
)

// rawEntry holds a decoded packfile entry before delta resolution.
type rawEntry struct {
	objType int
	payload []byte // decompressed; for deltas this is the delta data, not the final object

	// OBJ_REF_DELTA only: SHA-1 of the base object.
	baseSHA string

	// OBJ_OFS_DELTA only: absolute byte offset of the base entry in the packfile.
	baseOffset int64

	// Byte offset of this entry in the packfile (used for OFS delta resolution).
	offset int64
}

// unpackPackfile reads a binary packfile from r and writes every contained
// object into the local kitcat object store under objectsDir.
//
// Delta objects (OBJ_REF_DELTA type 7, OBJ_OFS_DELTA type 6) are fully
// resolved: we buffer all entries, then apply deltas in dependency order
// until all objects have been materialised.
func unpackPackfile(r io.Reader, objectsDir string) error {
	// Buffer the entire packfile so we can use bytes.Reader throughout.
	// This is critical: zlib.NewReader reads ahead internally and will
	// over-consume bytes from a plain io.Reader, corrupting the stream
	// position for the next object. bytes.Reader tracks position exactly
	// after each zlib stream ends via Seek.
	all, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("buffering packfile: %w", err)
	}
	br := bytes.NewReader(all)

	// ── Header ──────────────────────────────────────────────────────────
	header := make([]byte, 12)
	if _, err := io.ReadFull(br, header); err != nil {
		return fmt.Errorf("reading pack header: %w", err)
	}
	if string(header[:4]) != "PACK" {
		return fmt.Errorf("not a packfile: missing PACK magic (got %q)", header[:4])
	}
	version := binary.BigEndian.Uint32(header[4:8])
	if version != 2 && version != 3 {
		return fmt.Errorf("unsupported packfile version %d", version)
	}
	count := int(binary.BigEndian.Uint32(header[8:12]))

	// ── Pass 1: read all entries, write full objects immediately ─────────
	offsetToSHA := make(map[int64]string, count)
	var pending []rawEntry

	for i := 0; i < count; i++ {
		// Record offset relative to start of packfile data (after 12-byte header).
		entryOffset := int64(len(all)) - int64(br.Len())
		objType, _, err := readPackObjectHeader(br)
		if err != nil {
			return fmt.Errorf("reading object %d header: %w", i+1, err)
		}

		switch objType {
		case objCommit, objTree, objBlob, objTag:
			payload, err := zlibDecompressExact(br)
			if err != nil {
				return fmt.Errorf("decompressing object %d: %w", i+1, err)
			}
			sha, err := writeLooseObjectGetSHA(objType, payload, objectsDir)
			if err != nil {
				return fmt.Errorf("writing object %d: %w", i+1, err)
			}
			offsetToSHA[entryOffset] = sha

		case objRefDelta:
			rawSHA := make([]byte, 20)
			if _, err := io.ReadFull(br, rawSHA); err != nil {
				return fmt.Errorf("reading ref-delta base SHA: %w", err)
			}
			baseSHA := fmt.Sprintf("%x", rawSHA)
			delta, err := zlibDecompressExact(br)
			if err != nil {
				return fmt.Errorf("decompressing ref-delta %d: %w", i+1, err)
			}
			pending = append(pending, rawEntry{
				objType: objRefDelta,
				payload: delta,
				baseSHA: baseSHA,
				offset:  entryOffset,
			})

		case objOfsDelta:
			negOffset, err := readOfsDeltaOffset(br)
			if err != nil {
				return fmt.Errorf("reading ofs-delta offset: %w", err)
			}
			baseOffset := entryOffset - negOffset
			delta, err := zlibDecompressExact(br)
			if err != nil {
				return fmt.Errorf("decompressing ofs-delta %d: %w", i+1, err)
			}
			pending = append(pending, rawEntry{
				objType:    objOfsDelta,
				payload:    delta,
				baseOffset: baseOffset,
				offset:     entryOffset,
			})

		default:
			return fmt.Errorf("unknown pack object type %d at entry %d", objType, i+1)
		}
	}

	// ── Pass 2: resolve deltas iteratively until all are resolved ────────
	// We loop until no progress is made (which would indicate a broken pack).
	for len(pending) > 0 {
		resolved := 0
		var stillPending []rawEntry

		for _, entry := range pending {
			// Find the base SHA.
			var baseSHA string
			switch entry.objType {
			case objRefDelta:
				baseSHA = entry.baseSHA
			case objOfsDelta:
				var ok bool
				baseSHA, ok = offsetToSHA[entry.baseOffset]
				if !ok {
					stillPending = append(stillPending, entry)
					continue
				}
			}

			// Load the base object from the object store.
			basePayload, baseType, err := readLooseObject(baseSHA, objectsDir)
			if err != nil {
				// Base might not be written yet if it was itself a delta.
				stillPending = append(stillPending, entry)
				continue
			}

			// Apply the delta to reconstruct the target object.
			result, err := applyDelta(basePayload, entry.payload)
			if err != nil {
				return fmt.Errorf("applying delta (base %s): %w", baseSHA, err)
			}

			sha, err := writeLooseObjectGetSHA(baseType, result, objectsDir)
			if err != nil {
				return fmt.Errorf("writing delta result: %w", err)
			}
			offsetToSHA[entry.offset] = sha
			resolved++
		}

		if resolved == 0 && len(stillPending) > 0 {
			return fmt.Errorf("cannot resolve %d delta(s): possible missing base objects", len(stillPending))
		}
		pending = stillPending
	}

	// Consume the trailing 20-byte SHA-1 checksum.
	checksum := make([]byte, 20)
	_, _ = io.ReadFull(br, checksum)

	return nil
}

// ── Delta application ────────────────────────────────────────────────────────
//
// Git delta format:
//
//	source-size   (variable-length little-endian)
//	target-size   (variable-length little-endian)
//	instructions...
//
// Each instruction is either:
//
//	Copy:   0x80 | flags  [offset bytes]  [size bytes]
//	Insert: 0x00–0x7f  <n data bytes>
func applyDelta(base, delta []byte) ([]byte, error) {
	r := bytes.NewReader(delta)

	srcSize, err := readDeltaSize(r)
	if err != nil {
		return nil, fmt.Errorf("reading source size: %w", err)
	}
	if srcSize != uint64(len(base)) {
		return nil, fmt.Errorf("delta source size mismatch: expected %d got %d", srcSize, len(base))
	}

	targetSize, err := readDeltaSize(r)
	if err != nil {
		return nil, fmt.Errorf("reading target size: %w", err)
	}

	out := make([]byte, 0, targetSize)

	for r.Len() > 0 {
		cmd, err := r.ReadByte()
		if err != nil {
			return nil, err
		}

		if cmd&0x80 != 0 {
			// ── Copy instruction ─────────────────────────────────────
			// Bit layout of cmd: 1 o o o o s s s
			//   o bits select which of 4 offset bytes follow
			//   s bits select which of 3 size bytes follow
			var offset, size uint32

			for i := uint(0); i < 4; i++ {
				if cmd&(1<<i) != 0 {
					b, err := r.ReadByte()
					if err != nil {
						return nil, err
					}
					offset |= uint32(b) << (i * 8)
				}
			}
			for i := uint(0); i < 3; i++ {
				if cmd&(1<<(i+4)) != 0 {
					b, err := r.ReadByte()
					if err != nil {
						return nil, err
					}
					size |= uint32(b) << (i * 8)
				}
			}
			if size == 0 {
				size = 0x10000
			}
			end := int(offset) + int(size)
			if end > len(base) {
				return nil, fmt.Errorf("copy out of bounds: [%d:%d] base len %d", offset, end, len(base))
			}
			out = append(out, base[offset:end]...)

		} else if cmd != 0 {
			// ── Insert instruction ───────────────────────────────────
			// cmd itself is the number of literal bytes that follow.
			buf := make([]byte, cmd)
			if _, err := io.ReadFull(r, buf); err != nil {
				return nil, err
			}
			out = append(out, buf...)

		} else {
			return nil, fmt.Errorf("unexpected zero delta instruction byte")
		}
	}

	if uint64(len(out)) != targetSize {
		return nil, fmt.Errorf("delta target size mismatch: expected %d got %d", targetSize, len(out))
	}
	return out, nil
}

// readDeltaSize reads a variable-length little-endian size from a delta stream.
// Each byte contributes 7 bits; the MSB is a continuation flag.
func readDeltaSize(r *bytes.Reader) (uint64, error) {
	var size uint64
	var shift uint
	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		size |= uint64(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			break
		}
	}
	return size, nil
}

// readOfsDeltaOffset reads the variable-length negative offset used by
// OBJ_OFS_DELTA to point to the base object within the same packfile.
//
// Format: each byte has MSB as continuation; the value is accumulated as
// (value+1) << 7 | (next & 0x7f) to avoid ambiguous zero encodings.
func readOfsDeltaOffset(r io.Reader) (int64, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return 0, err
	}
	val := int64(b[0] & 0x7f)
	for b[0]&0x80 != 0 {
		if _, err := io.ReadFull(r, b); err != nil {
			return 0, err
		}
		val = ((val + 1) << 7) | int64(b[0]&0x7f)
	}
	return val, nil
}

// ── Object store helpers ─────────────────────────────────────────────────────

// writeLooseObjectGetSHA writes an object and returns its SHA-1 hex string.
func writeLooseObjectGetSHA(objType int, payload []byte, objectsDir string) (string, error) {
	typeName := packTypeToName(objType)
	header := fmt.Sprintf("%s %d\x00", typeName, len(payload))
	full := append([]byte(header), payload...)

	sum := sha1.Sum(full)
	hash := fmt.Sprintf("%x", sum)

	dir := filepath.Join(objectsDir, hash[:2])
	path := filepath.Join(dir, hash[2:])

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return hash, nil // already present — objects are immutable
	}

	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, _ = w.Write(full)
	w.Close()

	tmp, err := os.CreateTemp(dir, "obj-*.tmp")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	_, writeErr := tmp.Write(buf.Bytes())
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		os.Remove(tmpPath)
		if writeErr != nil {
			return "", writeErr
		}
		if syncErr != nil {
			return "", syncErr
		}
		return "", closeErr
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	return hash, nil
}

// writeLooseObject is the original API kept for callers that don't need the SHA.
func writeLooseObject(objType int, payload []byte, objectsDir string) error {
	_, err := writeLooseObjectGetSHA(objType, payload, objectsDir)
	return err
}

// readLooseObject reads a loose object from objectsDir and returns its
// raw payload (without the "type size\0" header) and type constant.
func readLooseObject(sha, objectsDir string) ([]byte, int, error) {
	path := filepath.Join(objectsDir, sha[:2], sha[2:])
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	zr, err := zlib.NewReader(f)
	if err != nil {
		return nil, 0, err
	}
	defer zr.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(zr); err != nil {
		return nil, 0, err
	}
	full := buf.Bytes()

	nullIdx := bytes.IndexByte(full, 0)
	if nullIdx == -1 {
		return nil, 0, fmt.Errorf("malformed object %s", sha)
	}

	headerStr := string(full[:nullIdx])
	payload := full[nullIdx+1:]

	var typeID int
	switch {
	case len(headerStr) > 7 && headerStr[:7] == "commit ":
		typeID = objCommit
	case len(headerStr) > 5 && headerStr[:5] == "tree ":
		typeID = objTree
	case len(headerStr) > 5 && headerStr[:5] == "blob ":
		typeID = objBlob
	case len(headerStr) > 4 && headerStr[:4] == "tag ":
		typeID = objTag
	default:
		return nil, 0, fmt.Errorf("unknown object type in header %q", headerStr)
	}
	return payload, typeID, nil
}

// ── Streaming helpers ────────────────────────────────────────────────────────

// readPackObjectHeader decodes the variable-length object header.
func readPackObjectHeader(r io.Reader) (int, int, error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return 0, 0, err
	}
	objType := int((b[0] >> 4) & 0x7)
	size := int(b[0] & 0xf)
	shift := 4
	for b[0]&0x80 != 0 {
		if _, err := io.ReadFull(r, b); err != nil {
			return 0, 0, err
		}
		size |= int(b[0]&0x7f) << shift
		shift += 7
	}
	return objType, size, nil
}

// zlibDecompressExact decompresses one zlib stream from br and advances br
// to the exact byte after the compressed data ends.
//
// zlib.NewReader buffers reads internally, so after decompression the
// underlying reader's position overshoots. We solve this cleanly:
//  1. Drain all remaining bytes from br into a local slice.
//  2. Decompress from a fresh bytes.Reader over that slice.
//  3. After Close(), sub.Len() tells us how many bytes were NOT consumed.
//  4. Seek br to (totalSize - unconsumed) — the exact position after the stream.
func zlibDecompressExact(br *bytes.Reader) ([]byte, error) {
	startPos := br.Size() - int64(br.Len())

	remaining := make([]byte, br.Len())
	if _, err := io.ReadFull(br, remaining); err != nil {
		return nil, fmt.Errorf("buffering for zlib: %w", err)
	}

	sub := bytes.NewReader(remaining)
	zr, err := zlib.NewReader(sub)
	if err != nil {
		return nil, fmt.Errorf("zlib.NewReader: %w", err)
	}
	var out bytes.Buffer
	if _, err := out.ReadFrom(zr); err != nil {
		zr.Close()
		return nil, fmt.Errorf("zlib read: %w", err)
	}
	zr.Close()

	// Seek br to startPos + number of bytes zlib actually consumed.
	consumed := int64(len(remaining)) - int64(sub.Len())
	if _, err := br.Seek(startPos+consumed, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seeking after zlib: %w", err)
	}

	return out.Bytes(), nil
}


// packTypeToName maps a pack object type constant to its string name.
func packTypeToName(t int) string {
	switch t {
	case objCommit:
		return "commit"
	case objTree:
		return "tree"
	case objBlob:
		return "blob"
	case objTag:
		return "tag"
	default:
		return "unknown"
	}
}
