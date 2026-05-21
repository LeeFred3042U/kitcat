package remote

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/core"
	"github.com/LeeFred3042U/kitcat/internal/repo"
)

type FetchOptions struct {
	RemoteName string
	RemoteURL  string
	Auth       *Auth
	Refspecs   []string
	Prune      bool
}

type RefUpdate struct {
	RemoteRef string
	OldSHA    string
	NewSHA    string
}

type FetchResult struct {
	Updated []RefUpdate
	Pruned  []string
}

// Fetch updates remote-tracking refs and downloads any missing objects.
func Fetch(opts FetchOptions) (FetchResult, error) {
	var res FetchResult

	remoteName := opts.RemoteName
	if remoteName == "" {
		remoteName = "origin"
	}

	remoteURL := opts.RemoteURL
	if remoteURL == "" {
		url, found, err := core.GetConfig("remote." + remoteName + ".url")
		if err != nil {
			return res, fmt.Errorf("reading remote config: %w", err)
		}
		if !found {
			return res, fmt.Errorf("remote %q not configured; set it with: kitcat config remote.%s.url <url>", remoteName, remoteName)
		}
		remoteURL = url
	}

	hostname, _ := hostnameFromRemoteURL(remoteURL)
	auth := opts.Auth
	if (auth == nil || auth.Username == "" || auth.Password == "") && hostname != "" {
		if kc, _ := keychainGet(hostname); kc != nil {
			auth = kc
		}
	}

	refs, _, err := discoverRefs(remoteURL, "git-upload-pack", auth)
	if err != nil && (errors.Is(err, ErrAuthRequired) || errors.Is(err, ErrAuthDenied)) && hostname != "" {
		auth, err = ResolveAuth(remoteURL, opts.Auth)
		if err != nil {
			return res, err
		}
		refs, _, err = discoverRefs(remoteURL, "git-upload-pack", auth)
	}
	if err != nil {
		return res, fmt.Errorf("ref discovery: %w", err)
	}

	refMap := make(map[string]string, len(refs))
	for _, r := range refs {
		refMap[r.Name] = r.SHA
	}

	refspecs := opts.Refspecs
	if len(refspecs) == 0 {
		if v, found, _ := core.GetConfig("remote." + remoteName + ".fetch"); found && strings.TrimSpace(v) != "" {
			refspecs = []string{v}
		} else {
			refspecs = []string{"+refs/heads/*:refs/remotes/" + remoteName + "/*"}
		}
	}

	var mappings []RefMapping
	for _, rs := range refspecs {
		spec, err := ParseRefspec(rs)
		if err != nil {
			return res, err
		}
		ms, err := ExpandRefspec(spec, refs)
		if err != nil {
			return res, err
		}
		mappings = append(mappings, ms...)
	}

	// Read local remote-tracking refs for this remote.
	localTracking, err := readRemoteTrackingRefs(remoteName)
	if err != nil {
		return res, err
	}

	// Determine which refs need updating.
	wantSet := make(map[string]bool)
	for _, m := range mappings {
		newSHA, ok := refMap[m.Src]
		if !ok || newSHA == "" || newSHA == zeroSHA {
			continue
		}
		oldSHA := localTracking[m.Dst]
		if oldSHA != newSHA {
			res.Updated = append(res.Updated, RefUpdate{
				RemoteRef: m.Src,
				OldSHA:    oldSHA,
				NewSHA:    newSHA,
			})
			wantSet[newSHA] = true
		}
	}

	// Download objects only if there are updates.
	if len(wantSet) > 0 {
		var wantSHAs []string
		for sha := range wantSet {
			wantSHAs = append(wantSHAs, sha)
		}
		sort.Strings(wantSHAs)

		body, err := doUploadPack(remoteURL, wantSHAs, auth)
		if err != nil && (errors.Is(err, ErrAuthRequired) || errors.Is(err, ErrAuthDenied)) && hostname != "" {
			auth, err2 := ResolveAuth(remoteURL, opts.Auth)
			if err2 != nil {
				return res, fmt.Errorf("upload-pack: %w", err)
			}
			body, err = doUploadPack(remoteURL, wantSHAs, auth)
		}
		if err != nil {
			return res, fmt.Errorf("upload-pack: %w", err)
		}
		defer body.Close()

		packData, err := demultiplexSideband(body)
		if err != nil {
			return res, fmt.Errorf("demuxing sideband: %w", err)
		}
		if err := unpackPackfile(bytes.NewReader(packData), repo.ObjectsDir); err != nil {
			return res, fmt.Errorf("unpacking objects: %w", err)
		}
	}

	// Write updated remote-tracking refs.
	for _, u := range res.Updated {
		// Map back from remote ref name to local tracking ref path.
		// For the minimal refspec support we only write the heads->remotes mapping.
		remoteBranch := strings.TrimPrefix(u.RemoteRef, "refs/heads/")
		if remoteBranch == u.RemoteRef {
			continue
		}
		trackingRef := filepath.Join(repo.Dir, "refs", "remotes", remoteName, remoteBranch)
		if err := os.MkdirAll(filepath.Dir(trackingRef), 0o755); err != nil {
			return res, err
		}
		if err := os.WriteFile(trackingRef, []byte(u.NewSHA+"\n"), 0o644); err != nil {
			return res, fmt.Errorf("writing remote-tracking ref: %w", err)
		}
	}

	// Prune stale remote-tracking refs.
	if opts.Prune {
		pruned, err := pruneRemoteTrackingRefs(remoteName, refMap)
		if err != nil {
			return res, err
		}
		res.Pruned = pruned
	}

	if hostname != "" && auth != nil && auth.Username != "" && auth.Password != "" {
		_ = OfferSave(hostname, auth)
	}

	return res, nil
}

func readRemoteTrackingRefs(remoteName string) (map[string]string, error) {
	out := make(map[string]string)
	root := filepath.Join(repo.Dir, "refs", "remotes", remoteName)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		sha := strings.TrimSpace(string(b))
		// Use refname-like key ("refs/remotes/<remote>/<branch>") for comparisons.
		rel, err := filepath.Rel(repo.Dir, path)
		if err != nil {
			return nil
		}
		refName := filepath.ToSlash(rel)
		out[refName] = sha
		return nil
	})
	return out, nil
}

func pruneRemoteTrackingRefs(remoteName string, remoteRefMap map[string]string) ([]string, error) {
	root := filepath.Join(repo.Dir, "refs", "remotes", remoteName)
	var pruned []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(repo.Dir, path)
		if err != nil {
			return nil
		}
		refName := filepath.ToSlash(rel) // refs/remotes/<remote>/...

		// Only prune branch tracking refs and only if remote no longer advertises it.
		if !strings.HasPrefix(refName, "refs/remotes/"+remoteName+"/") {
			return nil
		}
		branch := strings.TrimPrefix(refName, "refs/remotes/"+remoteName+"/")
		if branch == "" {
			return nil
		}
		if _, ok := remoteRefMap["refs/heads/"+branch]; ok {
			return nil
		}
		_ = os.Remove(path)
		pruned = append(pruned, refName)
		return nil
	})
	sort.Strings(pruned)
	return pruned, nil
}

