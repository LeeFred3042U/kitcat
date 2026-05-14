package remote

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LeeFred3042U/kitcat/internal/repo"
)

// Object type constants matching the Git packfile spec.
const (
	objCommit = 1
	objTree   = 2
	objBlob   = 3
	objTag    = 4
)

// packObject holds a raw (uncompressed, no header) object payload plus metadata.
type packObject struct {
	hash    string
	objType int
	data    []byte
}

// buildPackfile constructs a binary Git packfile from the provided objects.
//
// Packfile layout:
//
//	"PACK"           4 bytes magic
//	version          4 bytes big-endian uint32 = 2
//	object count     4 bytes big-endian uint32
//	[objects]        variable — each object has a size-encoded header + zlib payload
//	SHA-1 checksum   20 bytes over everything above
func buildPackfile(objects []packObject) ([]byte, error) {
	var buf bytes.Buffer

	// Magic + version + count
	buf.Write([]byte("PACK"))
	versionBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(versionBytes, 2)
	buf.Write(versionBytes)

	countBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(countBytes, uint32(len(objects)))
	buf.Write(countBytes)

	for _, obj := range objects {
		header := encodeObjectHeader(obj.objType, len(obj.data))
		buf.Write(header)

		// Payload is zlib-compressed.
		var compressed bytes.Buffer
		w := zlib.NewWriter(&compressed)
		if _, err := w.Write(obj.data); err != nil {
			return nil, fmt.Errorf("compressing object %s: %w", obj.hash, err)
		}
		if err := w.Close(); err != nil {
			return nil, fmt.Errorf("finalizing zlib for %s: %w", obj.hash, err)
		}
		buf.Write(compressed.Bytes())
	}

	// Trailing SHA-1 over the entire packfile content.
	sum := sha1.Sum(buf.Bytes())
	buf.Write(sum[:])

	return buf.Bytes(), nil
}

// encodeObjectHeader encodes the variable-length object header used inside packfiles.
//
// First byte layout:  1 t t t s s s s   (MSB=continuation, 3 bits type, 4 bits size)
// Subsequent bytes:   1 s s s s s s s   (MSB=continuation, 7 bits size)
func encodeObjectHeader(objType, size int) []byte {
	var buf []byte

	// First byte: top bit = continuation, bits 6-4 = type, bits 3-0 = low 4 size bits.
	b := byte((objType&0x7)<<4) | byte(size&0xf)
	size >>= 4

	for size > 0 {
		buf = append(buf, b|0x80) // set continuation bit
		b = byte(size & 0x7f)
		size >>= 7
	}
	buf = append(buf, b)
	return buf
}

// collectObjectsForPush walks the commit graph starting from headSHA and
// collects all objects (commits, trees, blobs) that are not yet on the remote.
//
// knownSHA is the remote's current tip ("0"*40 if the branch is new). Any
// object reachable from knownSHA is assumed to already exist on the remote
// and is excluded from the returned set.
func collectObjectsForPush(headSHA, knownSHA string) ([]packObject, error) {
	visited := make(map[string]bool)
	var objects []packObject

	// Walk the local commit graph and stop at anything already known to the remote.
	if err := walkCommit(headSHA, knownSHA, visited, &objects); err != nil {
		return nil, err
	}
	return objects, nil
}

// walkCommit recursively collects a commit and its reachable tree/blob objects.
func walkCommit(sha, stopSHA string, visited map[string]bool, objects *[]packObject) error {
	if sha == "" || sha == stopSHA || visited[sha] {
		return nil
	}
	visited[sha] = true

	raw, objType, err := readRawObject(sha)
	if err != nil {
		return fmt.Errorf("reading object %s: %w", sha, err)
	}

	*objects = append(*objects, packObject{hash: sha, objType: objType, data: raw})

	if objType == objCommit {
		parsed := parseRawCommit(raw)
		// Recurse into the tree.
		if err := walkTree(parsed.treeSHA, visited, objects); err != nil {
			return err
		}
		// Recurse into all parents.
		for _, parent := range parsed.parentSHAs {
			if err := walkCommit(parent, stopSHA, visited, objects); err != nil {
				return err
			}
		}
	}
	return nil
}

// walkTree recursively collects a tree object and all its blob/subtree children.
func walkTree(sha string, visited map[string]bool, objects *[]packObject) error {
	if sha == "" || visited[sha] {
		return nil
	}
	visited[sha] = true

	raw, _, err := readRawObject(sha)
	if err != nil {
		return fmt.Errorf("reading tree %s: %w", sha, err)
	}
	*objects = append(*objects, packObject{hash: sha, objType: objTree, data: raw})

	// Parse tree entries to find blobs and subtrees.
	entries := parseRawTree(raw)
	for _, entry := range entries {
		if visited[entry.sha] {
			continue
		}
		if entry.isTree {
			if err := walkTree(entry.sha, visited, objects); err != nil {
				return err
			}
		} else {
			if err := collectBlob(entry.sha, visited, objects); err != nil {
				return err
			}
		}
	}
	return nil
}

// collectBlob reads a blob and appends it to objects.
func collectBlob(sha string, visited map[string]bool, objects *[]packObject) error {
	if visited[sha] {
		return nil
	}
	visited[sha] = true

	raw, _, err := readRawObject(sha)
	if err != nil {
		return fmt.Errorf("reading blob %s: %w", sha, err)
	}
	*objects = append(*objects, packObject{hash: sha, objType: objBlob, data: raw})
	return nil
}

// readRawObject reads and decompresses an object from the local object store,
// returning its raw payload (without the "type size\0" header) and its type constant.
func readRawObject(sha string) ([]byte, int, error) {
	path := filepath.Join(repo.ObjectsDir, sha[:2], sha[2:])
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	r, err := zlib.NewReader(f)
	if err != nil {
		return nil, 0, err
	}
	defer r.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		return nil, 0, err
	}
	full := buf.Bytes()

	// Header format: "<type> <size>\0"
	before, after, ok := bytes.Cut(full, []byte{0})
	if !ok {
		return nil, 0, fmt.Errorf("malformed object %s", sha)
	}

	headerStr := string(before)
	payload := after

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
		return nil, 0, fmt.Errorf("unknown object type in %q", headerStr)
	}

	return payload, typeID, nil
}

// rawCommitInfo holds the fields needed for graph traversal.
type rawCommitInfo struct {
	treeSHA    string
	parentSHAs []string
}

// parseRawCommit extracts tree and parent SHAs from a raw commit payload.
func parseRawCommit(data []byte) rawCommitInfo {
	var info rawCommitInfo
	lines := bytes.SplitSeq(data, []byte("\n"))
	for line := range lines {
		if len(line) == 0 {
			break // blank line separates headers from message
		}
		if bytes.HasPrefix(line, []byte("tree ")) {
			info.treeSHA = string(line[5:])
		} else if bytes.HasPrefix(line, []byte("parent ")) {
			info.parentSHAs = append(info.parentSHAs, string(line[7:]))
		}
	}
	return info
}

// treeEntry holds the parsed fields of a single tree entry.
type treeEntry struct {
	sha    string
	isTree bool
}

// parseRawTree parses the binary tree payload and returns its entries.
//
// Tree entry format (repeated, no separator):
//
//	"<mode> <name>\0<20-byte-binary-sha>"
func parseRawTree(data []byte) []treeEntry {
	var entries []treeEntry
	i := 0
	for i < len(data) {
		// Find the null separator between "<mode> <name>" and the binary SHA.
		nullIdx := bytes.IndexByte(data[i:], 0)
		if nullIdx == -1 || i+nullIdx+21 > len(data) {
			break
		}
		modeAndName := string(data[i : i+nullIdx])
		i += nullIdx + 1

		sha := fmt.Sprintf("%x", data[i:i+20])
		i += 20

		// Mode starts with "4" for trees (040000), "1" for files (100644, 100755, 120000).
		isTree := len(modeAndName) > 0 && modeAndName[0] == '4'
		_ = modeAndName // name not needed for traversal

		entries = append(entries, treeEntry{sha: sha, isTree: isTree})
	}
	return entries
}
