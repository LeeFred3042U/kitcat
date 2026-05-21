package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/core"
	"github.com/LeeFred3042U/kitcat/internal/repo"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// PullOptions configures a pull operation (fetch + integrate).
type PullOptions struct {
	RemoteName string // Remote name; defaults to "origin".
	Branch     string // Override branch; empty = current branch.
	Auth       *Auth
	Rebase     bool // Rebase onto upstream instead of merging.
	FFOnly     bool // Fail if not fast-forwardable.
	NoFF       bool // Always create a merge commit even if fast-forwardable.
}

// PullResult describes the outcome of a pull operation.
type PullResult struct {
	AlreadyUpToDate bool
	FastForwarded   bool
	Merged          bool
	Rebased         bool
	LocalSHA        string // SHA before update.
	RemoteSHA       string // SHA after update.
}

// Pull performs a fetch followed by integration (fast-forward, merge, or rebase)
// of the upstream branch into the current local branch.
func Pull(opts PullOptions) (PullResult, error) {
	var res PullResult

	remoteName := opts.RemoteName
	if remoteName == "" {
		remoteName = "origin"
	}

	// 1. Resolve current branch.
	branch := opts.Branch
	if branch == "" {
		b, err := core.CurrentBranch()
		if err != nil {
			return res, fmt.Errorf("cannot determine current branch: %w", err)
		}
		branch = b
	}

	// 2. Fetch from remote.
	_, err := Fetch(FetchOptions{
		RemoteName: remoteName,
		Auth:       opts.Auth,
	})
	if err != nil {
		return res, fmt.Errorf("fetch: %w", err)
	}

	// 3. Read upstream configuration: branch.<branch>.merge
	mergeRef, found, err := core.GetConfig(fmt.Sprintf("branch.%s.merge", branch))
	if err != nil {
		return res, fmt.Errorf("reading upstream config: %w", err)
	}
	if !found || strings.TrimSpace(mergeRef) == "" {
		return res, fmt.Errorf("no upstream configured for branch %q; use 'kitcat push --set-upstream' first", branch)
	}

	// 4. Derive the remote-tracking ref path.
	// mergeRef is typically "refs/heads/main" — strip prefix to get "main".
	upstreamBranch := strings.TrimPrefix(mergeRef, "refs/heads/")
	trackingRefPath := filepath.Join(repo.Dir, "refs", "remotes", remoteName, upstreamBranch)

	// 5. Read the remote-tracking SHA.
	trackingData, err := os.ReadFile(trackingRefPath)
	if err != nil {
		return res, fmt.Errorf("remote-tracking ref not found for %s/%s: %w", remoteName, upstreamBranch, err)
	}
	remoteSHA := strings.TrimSpace(string(trackingData))
	if remoteSHA == "" {
		return res, fmt.Errorf("empty remote-tracking ref at %s", trackingRefPath)
	}

	// 6. Read the local branch SHA.
	localSHA, err := storage.ReadBranchSHA(branch)
	if err != nil {
		return res, fmt.Errorf("reading local branch SHA: %w", err)
	}

	res.LocalSHA = localSHA
	res.RemoteSHA = remoteSHA

	// 7. Already up to date?
	if localSHA == remoteSHA {
		res.AlreadyUpToDate = true
		return res, nil
	}

	// 8. Check if remote is a fast-forward of local.
	ff := isAncestor(localSHA, remoteSHA)

	if ff && !opts.NoFF {
		// Fast-forward: advance branch pointer and update workspace.
		if err := core.UpdateWorkspaceAndIndex(remoteSHA); err != nil {
			return res, fmt.Errorf("updating workspace: %w", err)
		}
		branchRefPath := filepath.Join(repo.HeadsDir, branch)
		if err := os.WriteFile(branchRefPath, []byte(remoteSHA+"\n"), 0o644); err != nil {
			return res, fmt.Errorf("advancing branch pointer: %w", err)
		}
		_ = core.ReflogAppend("refs/heads/"+branch, localSHA, remoteSHA, "pull: fast-forward")
		res.FastForwarded = true
		return res, nil
	}

	// 9. Not fast-forwardable (or --no-ff forced).
	if opts.FFOnly {
		return res, fmt.Errorf("not a fast-forward; refusing to pull (--ff-only)")
	}

	// For merge and rebase we write a temporary local branch ref pointing
	// at the remote-tracking SHA so the existing core.Merge / core.Rebase
	// functions can resolve it. The ref is cleaned up after the operation.
	tmpBranch := "KITCAT_PULL_HEAD"
	tmpRefPath := filepath.Join(repo.HeadsDir, tmpBranch)
	if err := os.MkdirAll(filepath.Dir(tmpRefPath), 0o755); err != nil {
		return res, err
	}
	if err := os.WriteFile(tmpRefPath, []byte(remoteSHA), 0o644); err != nil {
		return res, fmt.Errorf("writing temporary merge ref: %w", err)
	}
	defer os.Remove(tmpRefPath)

	if opts.Rebase {
		if err := core.Rebase(tmpBranch, false); err != nil {
			return res, fmt.Errorf("rebase: %w", err)
		}
		res.Rebased = true
		return res, nil
	}

	// Default: merge.
	if err := core.Merge(tmpBranch); err != nil {
		return res, fmt.Errorf("merge: %w", err)
	}
	res.Merged = true
	return res, nil
}

