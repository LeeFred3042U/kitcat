package remote

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LeeFred3042U/kitcat/internal/core"
	"github.com/LeeFred3042U/kitcat/internal/repo"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// PushOptions holds all parameters for a push operation.
type PushOptions struct {
	// RemoteName is the name of the configured remote (e.g. "origin").
	// If RemoteURL is also set, RemoteURL takes precedence.
	RemoteName string
	// RemoteURL overrides the URL looked up from config.
	RemoteURL string
	// Branch is the local branch to push. Defaults to the current branch.
	Branch string
	// Auth holds optional HTTP basic-auth credentials.
	Auth *Auth
	// Force skips the fast-forward check and forces the remote ref update.
	Force bool
	// SetUpstream writes branch.<branch>.remote/merge config after success.
	SetUpstream bool
}

// Push sends local commits for a branch to a remote repository.
//
// Steps:
//  1. Resolve the remote URL from options or config.
//  2. Resolve the local branch tip SHA.
//  3. Discover the remote's current ref state via git-receive-pack.
//  4. Fast-forward check (unless --force).
//  5. Collect objects the remote does not yet have.
//  6. Build a packfile containing those objects.
//  7. Send the pkt-line update request + packfile via HTTP POST.
//  8. Update the remote-tracking ref on success.
func Push(opts PushOptions) error {
	// ── 1. Resolve remote URL ────────────────────────────────────────────
	remoteURL := opts.RemoteURL
	remoteName := opts.RemoteName
	if remoteURL == "" {
		if remoteName == "" {
			remoteName = "origin"
		}
		url, found, err := core.GetConfig("remote." + remoteName + ".url")
		if err != nil {
			return fmt.Errorf("reading remote config: %w", err)
		}
		if !found {
			return fmt.Errorf("remote %q not configured; set it with: kitcat config remote.%s.url <url>", remoteName, remoteName)
		}
		remoteURL = url
	}

	hostname, _ := hostnameFromRemoteURL(remoteURL)
	auth := opts.Auth
	// If explicit auth wasn't provided, try keychain silently (no prompting yet).
	if (auth == nil || auth.Username == "" || auth.Password == "") && hostname != "" {
		if kc, _ := keychainGet(hostname); kc != nil {
			auth = kc
		}
	}

	// ── 2. Resolve local branch ──────────────────────────────────────────
	branch := opts.Branch
	if branch == "" {
		b, err := core.CurrentBranch()
		if err != nil {
			return fmt.Errorf("determining current branch: %w", err)
		}
		branch = b
	}

	localSHA, err := storage.ReadBranchSHA(branch)
	if err != nil {
		return fmt.Errorf("reading local branch %q: %w", branch, err)
	}
	if localSHA == "" {
		return fmt.Errorf("branch %q has no commits", branch)
	}

	// ── 3. Discover remote refs ──────────────────────────────────────────
	fmt.Printf("Pushing to %s\n", remoteURL)

	refs, _, err := discoverRefs(remoteURL, "git-receive-pack", auth)
	if err != nil {
		if (errors.Is(err, ErrAuthRequired) || errors.Is(err, ErrAuthDenied)) && hostname != "" {
			auth, err = ResolveAuth(remoteURL, opts.Auth)
			if err != nil {
				return err
			}
			refs, _, err = discoverRefs(remoteURL, "git-receive-pack", auth)
		}
		return fmt.Errorf("ref discovery: %w", err)
	}

	// Find the remote's current SHA for this branch.
	remoteRefName := "refs/heads/" + branch
	remoteSHA := zeroSHA // empty string means new branch
	for _, r := range refs {
		if r.Name == remoteRefName {
			remoteSHA = r.SHA
			break
		}
	}

	if remoteSHA == localSHA {
		fmt.Printf("Everything up-to-date\n")
		return nil
	}

	// ── 4. Fast-forward check ────────────────────────────────────────────
	if !opts.Force && remoteSHA != zeroSHA {
		if !isAncestor(remoteSHA, localSHA) {
			return fmt.Errorf(
				"push rejected: remote %s is not an ancestor of local %s\n"+
					"hint: pull first or use --force to overwrite",
				remoteSHA[:8], localSHA[:8],
			)
		}
	}

	// ── 5. Collect objects to send ───────────────────────────────────────
	stopAt := remoteSHA
	if stopAt == zeroSHA {
		stopAt = ""
	}

	objects, err := collectObjectsForPush(localSHA, stopAt)
	if err != nil {
		return fmt.Errorf("collecting objects: %w", err)
	}
	fmt.Printf("Writing objects: %d\n", len(objects))

	// ── 6. Build packfile ────────────────────────────────────────────────
	pack, err := buildPackfile(objects)
	if err != nil {
		return fmt.Errorf("building packfile: %w", err)
	}

	// ── 7. Send update request + packfile ────────────────────────────────
	body := buildReceivePackBody(remoteSHA, localSHA, remoteRefName, pack)
	if err := doReceivePack(remoteURL, body, auth); err != nil {
		if (errors.Is(err, ErrAuthRequired) || errors.Is(err, ErrAuthDenied)) && hostname != "" {
			auth, err2 := ResolveAuth(remoteURL, opts.Auth)
			if err2 != nil {
				return fmt.Errorf("receive-pack: %w", err)
			}
			if err := doReceivePack(remoteURL, body, auth); err != nil {
				return fmt.Errorf("receive-pack: %w", err)
			}
		} else {
			return fmt.Errorf("receive-pack: %w", err)
		}
	} else {
		// success
	}

	// ── 8. Update remote-tracking ref ───────────────────────────────────
	if remoteName == "" {
		remoteName = "origin"
	}
	trackingRef := filepath.Join(repo.Dir, "refs", "remotes", remoteName, branch)
	if err := os.MkdirAll(filepath.Dir(trackingRef), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(trackingRef, []byte(localSHA+"\n"), 0o644); err != nil {
		return fmt.Errorf("updating remote-tracking ref: %w", err)
	}

	fmt.Printf("Branch '%s' -> '%s/%s'\n", branch, remoteName, branch)

	// Optionally set upstream tracking.
	if opts.SetUpstream {
		if err := core.SetConfig("branch."+branch+".remote", remoteName, false); err != nil {
			return fmt.Errorf("setting upstream remote: %w", err)
		}
		if err := core.SetConfig("branch."+branch+".merge", "refs/heads/"+branch, false); err != nil {
			return fmt.Errorf("setting upstream merge ref: %w", err)
		}
	}

	if hostname != "" && auth != nil && auth.Username != "" && auth.Password != "" {
		_ = OfferSave(hostname, auth)
	}
	return nil
}

// zeroSHA is the null SHA used to indicate a non-existent ref.
const zeroSHA = "0000000000000000000000000000000000000000"

// buildReceivePackBody assembles the pkt-line command section followed by
// the raw packfile bytes, ready for POSTing to /git-receive-pack.
//
// Command format:
//
//	PKT-LINE(<old-sha> <new-sha> <refname>\0<capabilities>\n)
//	flush-pkt
//	[packfile bytes]
func buildReceivePackBody(oldSHA, newSHA, refName string, pack []byte) []byte {
	var buf bytes.Buffer

	// Capabilities we support (side-band-64k is NOT included — we use
	// the simple framing without a side-band multiplexer).
	caps := "report-status delete-refs ofs-delta"

	line := fmt.Sprintf("%s %s %s\x00%s\n", oldSHA, newSHA, refName, caps)
	buf.Write(encodePktLine(line))
	buf.Write(pktLineFlush)
	buf.Write(pack)

	return buf.Bytes()
}

// isAncestor returns true if candidate is an ancestor of tip by walking
// the local commit graph.
func isAncestor(candidate, tip string) bool {
	visited := make(map[string]bool)
	queue := []string{tip}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if curr == candidate {
			return true
		}
		if visited[curr] {
			continue
		}
		visited[curr] = true

		_, objType, err := readRawObject(curr)
		if err != nil || objType != objCommit {
			continue
		}

		raw, _, _ := readRawObject(curr)
		info := parseRawCommit(raw)
		for _, p := range info.parentSHAs {
			if !visited[p] {
				queue = append(queue, p)
			}
		}
	}
	return false
}
