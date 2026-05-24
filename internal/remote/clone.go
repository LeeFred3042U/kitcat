package remote

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/core"
	"github.com/LeeFred3042U/kitcat/internal/repo"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// CloneOptions holds all parameters for a clone operation.
type CloneOptions struct {
	// RemoteURL is the HTTPS URL of the repository to clone.
	RemoteURL string
	// Dir is the local destination directory. If empty, it is inferred
	// from the last path segment of RemoteURL (stripping ".git").
	Dir string
	// Auth holds optional HTTP basic-auth credentials.
	Auth *Auth
	// Branch is the branch to check out after cloning. Defaults to the
	// remote's HEAD branch (usually "main" or "master").
	Branch string
}

// Clone downloads a remote repository and materialises it on disk.
//
// Steps:
//  1. Resolve destination directory and create it.
//  2. Initialise a fresh kitcat repo inside it.
//  3. Discover remote refs via the smart HTTP protocol.
//  4. Request all objects for the desired branch via git-upload-pack.
//  5. Unpack the received packfile into the local object store.
//  6. Write remote-tracking refs and the local branch pointer.
//  7. Check out the working tree.
//  8. Record the remote URL in .kitcat/config.
func Clone(opts CloneOptions) error {
	// ── 1. Resolve destination directory ────────────────────────────────
	dir := opts.Dir
	if dir == "" {
		dir = inferDirName(opts.RemoteURL)
	}
	if dir == "" {
		return fmt.Errorf("cannot infer directory name from URL %q; use --dir", opts.RemoteURL)
	}

	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("destination %q already exists", dir)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	// If anything fails after this point we remove the partially-created dir.
	success := false
	defer func() {
		if !success {
			os.RemoveAll(dir)
		}
	}()

	// ── 2. cd into the new directory and initialise a kitcat repo ───────
	originalDir, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("entering destination directory: %w", err)
	}
	defer os.Chdir(originalDir) //nolint:errcheck

	if err := core.Init(); err != nil {
		return fmt.Errorf("init failed: %w", err)
	}

	// ── 3. Discover remote refs ──────────────────────────────────────────
	fmt.Printf("Cloning into '%s'...\n", dir)
	fmt.Println("remote: Counting objects...")

	hostname, _ := hostnameFromRemoteURL(opts.RemoteURL)
	auth := opts.Auth
	// If explicit auth wasn't provided, try keychain silently (no prompting yet).
	if (auth == nil || auth.Username == "" || auth.Password == "") && hostname != "" {
		if kc, _ := keychainGet(hostname); kc != nil {
			auth = kc
		}
	}

	refs, _, err := discoverRefs(opts.RemoteURL, "git-upload-pack", auth)
	if err != nil {
		// If the remote requires auth, prompt and retry once.
		if (errors.Is(err, ErrAuthRequired) || errors.Is(err, ErrAuthDenied)) && hostname != "" {
			auth, err = ResolveAuth(opts.RemoteURL, opts.Auth)
			if err != nil {
				return err
			}
			refs, _, err = discoverRefs(opts.RemoteURL, "git-upload-pack", auth)
		}
		return fmt.Errorf("ref discovery: %w", err)
	}
	if len(refs) == 0 {
		return fmt.Errorf("remote repository appears to be empty")
	}

	// ── 4. Determine which branch to check out ───────────────────────────
	branch := opts.Branch
	headSHA := ""

	// Build a map of refname → SHA for fast lookup.
	refMap := make(map[string]string, len(refs))
	for _, r := range refs {
		refMap[r.Name] = r.SHA
	}

	if branch == "" {
		// Follow the remote's HEAD symbolic ref.
		if sha, ok := refMap["HEAD"]; ok {
			// Find which branch HEAD points to.
			for _, r := range refs {
				if r.SHA == sha && strings.HasPrefix(r.Name, "refs/heads/") {
					branch = strings.TrimPrefix(r.Name, "refs/heads/")
					headSHA = sha
					break
				}
			}
		}
		// Fallback: try common default names.
		if branch == "" {
			for _, name := range []string{"main", "master"} {
				if sha, ok := refMap["refs/heads/"+name]; ok {
					branch = name
					headSHA = sha
					break
				}
			}
		}
	} else {
		headSHA = refMap["refs/heads/"+branch]
	}

	if headSHA == "" {
		return fmt.Errorf("branch %q not found on remote", branch)
	}

	// ── 5. Collect all SHAs to fetch ────────────────────────────────────
	// We want every ref tip so that all history is available locally.
	var wantSHAs []string
	seen := make(map[string]bool)
	for _, r := range refs {
		if r.SHA == "" || r.SHA == strings.Repeat("0", 40) {
			continue
		}
		if !seen[r.SHA] {
			seen[r.SHA] = true
			wantSHAs = append(wantSHAs, r.SHA)
		}
	}

	// ── 6. Fetch packfile ────────────────────────────────────────────────
	body, err := doUploadPack(opts.RemoteURL, wantSHAs, auth)
	if err != nil && (errors.Is(err, ErrAuthRequired) || errors.Is(err, ErrAuthDenied)) && hostname != "" {
		// Auth required for upload-pack; prompt and retry once.
		auth, err2 := ResolveAuth(opts.RemoteURL, opts.Auth)
		if err2 != nil {
			return fmt.Errorf("upload-pack: %w", err)
		}
		body, err = doUploadPack(opts.RemoteURL, wantSHAs, auth)
	}
	if err != nil {
		return fmt.Errorf("upload-pack: %w", err)
	}
	defer body.Close()

	// The server response is sideband-multiplexed: each pkt-line chunk is
	// prefixed with a 1-byte channel ID. Demultiplex to get the raw packfile.
	packData, err := demultiplexSideband(body)
	if err != nil {
		return fmt.Errorf("demuxing sideband: %w", err)
	}

	objectsDir := filepath.Join(repo.ObjectsDir)
	if err := unpackPackfile(bytes.NewReader(packData), objectsDir); err != nil {
		return fmt.Errorf("unpacking objects: %w", err)
	}
	fmt.Println("remote: done.")

	// ── 7. Write refs ────────────────────────────────────────────────────
	// Local branch pointer.
	branchFile := filepath.Join(repo.HeadsDir, branch)
	if err := os.MkdirAll(filepath.Dir(branchFile), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(branchFile, []byte(headSHA+"\n"), 0o644); err != nil {
		return fmt.Errorf("writing branch ref: %w", err)
	}

	// Remote-tracking refs under refs/remotes/origin/.
	for _, r := range refs {
		if !strings.HasPrefix(r.Name, "refs/heads/") {
			continue
		}
		remoteBranch := strings.TrimPrefix(r.Name, "refs/heads/")
		trackingRef := filepath.Join(repo.Dir, "refs", "remotes", "origin", remoteBranch)
		if err := os.MkdirAll(filepath.Dir(trackingRef), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(trackingRef, []byte(r.SHA+"\n"), 0o644); err != nil {
			return fmt.Errorf("writing remote-tracking ref: %w", err)
		}
	}

	// Set HEAD to the checked-out branch.
	headContent := fmt.Sprintf("ref: refs/heads/%s\n", branch)
	if err := os.WriteFile(repo.HeadPath, []byte(headContent), 0o644); err != nil {
		return fmt.Errorf("writing HEAD: %w", err)
	}

	// ── 8. Record remote URL in config ───────────────────────────────────
	if err := core.SetConfig("remote.origin.url", opts.RemoteURL, false); err != nil {
		return fmt.Errorf("writing remote config: %w", err)
	}
	if err := core.SetConfig("remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*", false); err != nil {
		return fmt.Errorf("writing fetch refspec: %w", err)
	}
	if err := core.SetConfig("branch."+branch+".remote", "origin", false); err != nil {
		return fmt.Errorf("writing branch remote config: %w", err)
	}
	if err := core.SetConfig("branch."+branch+".merge", "refs/heads/"+branch, false); err != nil {
		return fmt.Errorf("writing branch merge config: %w", err)
	}

	// ── 9. Check out the working tree ────────────────────────────────────
	commit, err := storage.FindCommit(headSHA)
	if err != nil {
		return fmt.Errorf("resolving HEAD commit: %w", err)
	}
	if err := core.CheckoutTree(commit.TreeHash); err != nil {
		return fmt.Errorf("checking out tree: %w", err)
	}
	if err := core.ReadTree(commit.TreeHash); err != nil {
		return fmt.Errorf("updating index: %w", err)
	}

	fmt.Printf("Branch '%s' set up to track 'origin/%s'.\n", branch, branch)

	// Offer to save creds only if we actually used authenticated access.
	if hostname != "" && auth != nil && auth.Username != "" && auth.Password != "" {
		_ = OfferSave(hostname, auth)
	}

	success = true
	return nil
}

// inferDirName derives a local directory name from a remote URL by taking
// the last path segment and stripping the ".git" suffix when present.
//
//	"https://github.com/user/myrepo.git" → "myrepo"
//	"https://github.com/user/myrepo"     → "myrepo"
func inferDirName(rawURL string) string {
	// Strip trailing slashes.
	u := strings.TrimRight(rawURL, "/")
	// Take the last segment.
	if idx := strings.LastIndexAny(u, "/"); idx >= 0 {
		u = u[idx+1:]
	}
	// Strip .git suffix.
	u = strings.TrimSuffix(u, ".git")
	return u
}

// demultiplexSideband reads a git-upload-pack response that uses the
// side-band-64k multiplexing protocol and reassembles the raw packfile.
//
// The response is a sequence of pkt-lines. Each pkt-line payload begins
// with a 1-byte channel identifier:
//
//	\x01  — packfile data  (reassemble into the output buffer)
//	\x02  — progress message (print to stderr, discard from pack stream)
//	\x03  — fatal error from server
//
// Lines without a channel byte (e.g. "NAK\n") are control lines and
// are skipped.
func demultiplexSideband(r io.Reader) ([]byte, error) {
	var pack bytes.Buffer

	// We need to read raw pkt-lines including their binary payload,
	// so we use the low-level length-prefix decoder directly.
	lenBuf := make([]byte, 4)
	for {
		_, err := io.ReadFull(r, lenBuf)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return nil, fmt.Errorf("reading sideband pkt-line length: %w", err)
		}

		var length int
		for _, c := range lenBuf {
			length <<= 4
			switch {
			case c >= '0' && c <= '9':
				length |= int(c - '0')
			case c >= 'a' && c <= 'f':
				length |= int(c-'a') + 10
			case c >= 'A' && c <= 'F':
				length |= int(c-'A') + 10
			}
		}

		// Flush packet — end of a section, but not necessarily end of stream.
		if length == 0 {
			continue
		}

		payload := make([]byte, length-4)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, fmt.Errorf("reading sideband payload: %w", err)
		}

		if len(payload) == 0 {
			continue
		}

		switch payload[0] {
		case 0x01:
			// Packfile data channel — accumulate.
			pack.Write(payload[1:])
		case 0x02:
			// Progress message — print and discard.
			fmt.Fprintf(os.Stderr, "remote: %s", payload[1:])
		case 0x03:
			// Error from server.
			return nil, fmt.Errorf("server error: %s", strings.TrimSpace(string(payload[1:])))
		default:
			// Control line (e.g. "NAK\n") — no channel byte, skip.
			// These appear before the sideband data starts.
		}
	}

	if pack.Len() == 0 {
		return nil, fmt.Errorf("no packfile data received from server")
	}
	return pack.Bytes(), nil
}
