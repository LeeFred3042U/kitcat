package remote

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/repo"
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
	if remoteURL == "" {
		remoteName := opts.RemoteName
		if remoteName == "" {
			remoteName = "origin"
		}
		url, found, err := readRemoteURL(remoteName)
		if err != nil {
			return fmt.Errorf("reading remote config: %w", err)
		}
		if !found {
			return fmt.Errorf("remote %q not configured; set it with: kitcat config remote.%s.url <url>", remoteName, remoteName)
		}
		remoteURL = url
	}

	// ── 2. Resolve local branch ──────────────────────────────────────────
	branch := opts.Branch
	if branch == "" {
		b, err := currentBranch()
		if err != nil {
			return fmt.Errorf("determining current branch: %w", err)
		}
		branch = b
	}

	localSHA, err := readLocalBranchSHA(branch)
	if err != nil {
		return fmt.Errorf("reading local branch %q: %w", branch, err)
	}
	if localSHA == "" {
		return fmt.Errorf("branch %q has no commits", branch)
	}

	// ── 3. Discover remote refs ──────────────────────────────────────────
	fmt.Printf("Pushing to %s\n", remoteURL)

	refs, _, err := discoverRefs(remoteURL, "git-receive-pack", opts.Auth)
	if err != nil {
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
	if err := doReceivePack(remoteURL, body, opts.Auth); err != nil {
		return fmt.Errorf("receive-pack: %w", err)
	}

	// ── 8. Update remote-tracking ref ───────────────────────────────────
	trackingRef := filepath.Join(repo.Dir, "refs", "remotes", "origin", branch)
	if err := os.MkdirAll(filepath.Dir(trackingRef), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(trackingRef, []byte(localSHA+"\n"), 0o644); err != nil {
		return fmt.Errorf("updating remote-tracking ref: %w", err)
	}

	fmt.Printf("Branch '%s' -> 'origin/%s'\n", branch, branch)
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

// readRemoteURL reads "remote.<name>.url" from the kitcat config file.
func readRemoteURL(remoteName string) (string, bool, error) {
	// Use the same INI parser the rest of kitcat uses (core.GetConfig).
	// We call it indirectly through the config file to avoid an import
	// cycle between remote and core.
	configPath := filepath.Join(repo.Dir, "config")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}

	// Parse INI looking for [remote "<name>"] → url = ...
	lines := strings.Split(string(data), "\n")
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			sec, sub, _ := parseINISectionHeader(trimmed)
			inSection = (sec == "remote" && sub == remoteName)
			continue
		}
		if inSection {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == "url" {
				return strings.TrimSpace(parts[1]), true, nil
			}
		}
	}
	return "", false, nil
}

// parseINISectionHeader parses "[section]" or "[section "subsection"]".
func parseINISectionHeader(line string) (section, subsection string, ok bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return
	}
	content := line[1 : len(line)-1]
	parts := strings.SplitN(content, " ", 2)
	section = parts[0]
	if len(parts) > 1 {
		subsection = strings.Trim(parts[1], "\"")
	}
	ok = true
	return
}

// currentBranch reads HEAD and returns the active branch name.
func currentBranch() (string, error) {
	data, err := os.ReadFile(repo.HeadPath)
	if err != nil {
		return "", err
	}
	ref := strings.TrimSpace(string(data))
	if trimmed, ok := strings.CutPrefix(ref, "ref: refs/heads/"); ok {
		return trimmed, nil
	}
	return "", fmt.Errorf("HEAD is detached; specify a branch explicitly")
}

// readLocalBranchSHA returns the commit SHA of a local branch.
func readLocalBranchSHA(branch string) (string, error) {
	path := filepath.Join(repo.HeadsDir, branch)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
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
