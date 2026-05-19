//go:build !windows

package remote

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Linux implementation using `secret-tool` (libsecret / Secret Service).
//
// We store the secret as "<username>\n<token>" with attributes:
//   application=kitcat
//   hostname=<hostname>

const (
	linuxAppAttr = "application"
	linuxAppVal  = "kitcat"
	linuxHostKey = "hostname"
)

func keychainGetLinux(hostname string) (*Auth, error) {
	if !commandExists("secret-tool") {
		return nil, nil
	}
	out, err := runTrimmed("secret-tool", "lookup", linuxAppAttr, linuxAppVal, linuxHostKey, hostname)
	if err != nil {
		// Not found is not an error for our chain.
		return nil, nil
	}
	user, token, ok := strings.Cut(out, "\n")
	if !ok {
		return nil, nil
	}
	user = strings.TrimSpace(user)
	token = strings.TrimSpace(token)
	if user == "" || token == "" {
		return nil, nil
	}
	return &Auth{Username: user, Password: token}, nil
}

func keychainSetLinux(hostname string, auth *Auth) error {
	if !commandExists("secret-tool") {
		return fmt.Errorf("%w: 'secret-tool' not found (install libsecret)", errKeychainUnavailable)
	}
	if auth == nil || auth.Username == "" || auth.Password == "" {
		return nil
	}

	// `secret-tool store --label=... application kitcat hostname github.com`
	// reads secret from stdin.
	secret := auth.Username + "\n" + auth.Password
	label := "kitcat:" + hostname
	cmd := exec.Command("secret-tool", "store", "--label="+label, linuxAppAttr, linuxAppVal, linuxHostKey, hostname)
	cmd.Stdin = bytes.NewBufferString(secret)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := strings.TrimSpace(string(out))
		if s == "" {
			return err
		}
		return fmt.Errorf("%s: %s", err.Error(), s)
	}
	return nil
}

func keychainDeleteLinux(hostname string) error {
	if !commandExists("secret-tool") {
		return fmt.Errorf("%w: 'secret-tool' not found (install libsecret)", errKeychainUnavailable)
	}
	// `secret-tool clear` returns non-zero if nothing was removed; treat as success.
	_, _ = runTrimmed("secret-tool", "clear", linuxAppAttr, linuxAppVal, linuxHostKey, hostname)
	return nil
}

func keychainListLinux() ([]string, error) {
	if !commandExists("secret-tool") {
		return nil, fmt.Errorf("%w: 'secret-tool' not found (install libsecret)", errKeychainUnavailable)
	}

	// `secret-tool search application kitcat` prints matching items and their attributes.
	// Output format isn't strictly documented, but it commonly includes lines like:
	//   attribute.hostname = github.com
	out, err := runTrimmed("secret-tool", "search", linuxAppAttr, linuxAppVal)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var hosts []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "attribute."+linuxHostKey) {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		host := strings.TrimSpace(parts[1])
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		hosts = append(hosts, host)
	}
	return hosts, nil
}

