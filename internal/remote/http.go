package remote

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Auth holds the credentials used for HTTP basic authentication.
type Auth struct {
	Username string
	Password string // personal access token for GitHub/GitLab/Gitea etc.
}

// RefInfo describes a single reference advertised by a remote.
type RefInfo struct {
	SHA  string
	Name string
}

// discoverRefs performs a Git smart HTTP ref discovery for the given service.
//
//	service = "git-upload-pack"  for fetch/clone
//	service = "git-receive-pack" for push
//
// It returns all refs advertised by the remote and the capabilities string.
func discoverRefs(remoteURL, service string, auth *Auth) ([]RefInfo, string, error) {
	url := strings.TrimSuffix(remoteURL, "/") + "/info/refs?service=" + service

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("building discovery request: %w", err)
	}
	req.Header.Set("User-Agent", "kitcat/1.0")
	if auth != nil {
		req.SetBasicAuth(auth.Username, auth.Password)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("discovery request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, "", fmt.Errorf("authentication required (401): check your credentials")
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, "", fmt.Errorf("access denied (403): check your token permissions")
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, "", fmt.Errorf("repository not found (404): check the URL")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected HTTP status %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading discovery response: %w", err)
	}

	return parseRefAdvertisement(bytes.NewReader(body), service)
}

// parseRefAdvertisement parses the pkt-line encoded ref advertisement.
//
// Response format:
//
//	PKT-LINE(# service=<service>\n)
//	flush
//	PKT-LINE(<sha> <refname>\0<capabilities>\n)
//	PKT-LINE(<sha> <refname>\n)
//	...
//	flush
func parseRefAdvertisement(r io.Reader, service string) ([]RefInfo, string, error) {
	// First line: "# service=<service>"
	first, err := decodePktLine(r)
	if err != nil {
		return nil, "", fmt.Errorf("reading service line: %w", err)
	}
	if first != "# service="+service {
		return nil, "", fmt.Errorf("unexpected service line: %q", first)
	}

	// Flush after service line.
	flush, err := decodePktLine(r)
	if err != nil {
		return nil, "", fmt.Errorf("reading post-service flush: %w", err)
	}
	if flush != "" {
		return nil, "", fmt.Errorf("expected flush after service line, got: %q", flush)
	}

	var refs []RefInfo
	var capabilities string
	first = ""

	for {
		line, err := decodePktLine(r)
		if err != nil {
			return nil, "", fmt.Errorf("reading ref line: %w", err)
		}
		if line == "" {
			break // flush = end of advertisement
		}

		// First ref line may contain capabilities after a NUL byte.
		nullIdx := strings.IndexByte(line, 0)
		if nullIdx != -1 {
			capabilities = line[nullIdx+1:]
			line = line[:nullIdx]
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		refs = append(refs, RefInfo{SHA: parts[0], Name: parts[1]})
		_ = first
	}

	return refs, capabilities, nil
}

// doUploadPack sends a git-upload-pack POST request and returns the response body.
// Used for fetch/clone to request specific objects from the remote.
func doUploadPack(remoteURL string, want []string, auth *Auth) (io.ReadCloser, error) {
	url := strings.TrimSuffix(remoteURL, "/") + "/git-upload-pack"

	var body bytes.Buffer
	for i, sha := range want {
		line := "want " + sha
		if i == 0 {
			// Advertise capabilities on the first want line only.
			line += " side-band-64k ofs-delta"
		}
		line += "\n"
		body.Write(encodePktLine(line))
	}
	body.Write(pktLineFlush)
	body.Write(encodePktLine("done\n"))

	req, err := http.NewRequest("POST", url, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-git-upload-pack-request")
	req.Header.Set("Accept", "application/x-git-upload-pack-result")
	req.Header.Set("User-Agent", "kitcat/1.0")
	if auth != nil {
		req.SetBasicAuth(auth.Username, auth.Password)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload-pack request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("upload-pack returned HTTP %d", resp.StatusCode)
	}
	return resp.Body, nil
}

// doReceivePack sends a git-receive-pack POST request to push objects.
func doReceivePack(remoteURL string, body []byte, auth *Auth) error {
	url := strings.TrimSuffix(remoteURL, "/") + "/git-receive-pack"

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-git-receive-pack-request")
	req.Header.Set("Accept", "application/x-git-receive-pack-result")
	req.Header.Set("User-Agent", "kitcat/1.0")
	if auth != nil {
		req.SetBasicAuth(auth.Username, auth.Password)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("receive-pack request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("push rejected (401): check your credentials")
	}
	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("push rejected (403): check your token has write access")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("receive-pack returned HTTP %d", resp.StatusCode)
	}

	// Read and check the server's report lines.
	return parseReceivePackResult(resp.Body)
}

// parseReceivePackResult parses the server's pkt-line response after a push
// and surfaces any error messages.
func parseReceivePackResult(r io.Reader) error {
	for {
		line, err := decodePktLine(r)
		if err != nil {
			// EOF is expected after the server closes the stream.
			return nil
		}
		if line == "" {
			continue // flush packet
		}
		// Server sends "unpack ok" or "unpack <error>" then per-ref status lines.
		if strings.HasPrefix(line, "unpack ") {
			status := strings.TrimPrefix(line, "unpack ")
			if status != "ok" {
				return fmt.Errorf("server rejected pack: %s", status)
			}
		}
		if strings.HasPrefix(line, "ng ") {
			// "ng <refname> <reason>" — server rejected this ref update.
			return fmt.Errorf("ref update rejected by server: %s", strings.TrimPrefix(line, "ng "))
		}
	}
}
