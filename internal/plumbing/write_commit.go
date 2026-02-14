package plumbing

import (
	"bytes"
	"fmt"
	"time"
)

// CommitOptions contains the metadata required to construct a commit object.
// Fields map directly to the canonical Git commit header structure.
type CommitOptions struct {
	Tree      string   // SHA-1 of root tree
	Parents   []string // SHA-1 of parent commits
	Author    string   // "Name <email>"
	Committer string   // "Name <email>"
	Message   string   // commit message
}

// CommitTree creates a commit object from the provided options and writes it
// to object storage, returning the resulting commit hash.
func CommitTree(opts CommitOptions) (string, error) {
	var buf bytes.Buffer

	// Write mandatory tree reference followed by zero or more parent links.
	// Parent order is preserved to maintain deterministic history graphs.
	buf.WriteString(fmt.Sprintf("tree %s\n", opts.Tree))
	for _, p := range opts.Parents {
		buf.WriteString(fmt.Sprintf("parent %s\n", p))
	}

	now := time.Now()
	seconds := now.Unix()
	offset := now.Format("-0700") // timezone offset like "+0530"

	// Fallback author ensures commits remain constructible even when identity
	// configuration is missing.
	author := opts.Author
	if author == "" {
		author = "KitKat User <user@kitkat>"
	}
	buf.WriteString(fmt.Sprintf("author %s %d %s\n", author, seconds, offset))

	// Committer defaults to author to mirror Git’s behavior for simple commits.
	committer := opts.Committer
	if committer == "" {
		committer = author
	}
	buf.WriteString(fmt.Sprintf("committer %s %d %s\n\n", committer, seconds, offset))

	buf.WriteString(opts.Message)

	// Commit messages must end with a newline; missing it changes the object hash
	// and breaks compatibility with canonical Git formatting.
	if !bytes.HasSuffix(buf.Bytes(), []byte("\n")) {
		buf.WriteByte('\n')
	}

	// Writing the object persists it under object storage and returns its hash.
	return HashAndWriteObject(buf.Bytes(), "commit")
}
