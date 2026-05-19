package remote

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// ResolveAuth resolves credentials for a remote.
//
// Resolution order:
//  1. explicit (flags/env) if provided and complete
//  2. OS keychain lookup by hostname
//  3. interactive prompt (username + hidden token)
func ResolveAuth(remoteURL string, explicit *Auth) (*Auth, error) {
	if explicit != nil && explicit.Username != "" && explicit.Password != "" {
		explicit.Source = AuthSourceExplicit
		return explicit, nil
	}

	hostname, err := hostnameFromRemoteURL(remoteURL)
	if err != nil {
		return nil, err
	}

	if auth, err := keychainGet(hostname); err != nil {
		return nil, err
	} else if auth != nil && auth.Username != "" && auth.Password != "" {
		auth.Source = AuthSourceKeychain
		return auth, nil
	}

	auth, err := promptForAuth(hostname)
	if err != nil {
		return nil, err
	}
	auth.Source = AuthSourcePrompt
	return auth, nil
}

// OfferSave prompts to store auth in the OS keychain.
// Intended to be called by the command after a successful operation.
func OfferSave(hostname string, auth *Auth) error {
	if auth == nil || auth.Username == "" || auth.Password == "" {
		return nil
	}

	// Only offer save for interactively entered credentials.
	if auth.Source != AuthSourcePrompt {
		return nil
	}

	fmt.Printf("Save credentials to keychain? [y/N] ")
	in := bufio.NewReader(os.Stdin)
	line, _ := in.ReadString('\n')
	line = strings.TrimSpace(line)
	if !strings.EqualFold(line, "y") && !strings.EqualFold(line, "yes") {
		return nil
	}

	if err := keychainSet(hostname, auth); err != nil {
		return err
	}
	_ = rememberCredentialHost(hostname)
	return nil
}

func hostnameFromRemoteURL(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("remote URL is empty")
	}

	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		// common case: scheme-less URL; retry with https://
		u2, err2 := url.Parse("https://" + raw)
		if err2 != nil || u2.Hostname() == "" {
			if err != nil {
				return "", fmt.Errorf("parsing remote URL %q: %w", raw, err)
			}
			return "", fmt.Errorf("parsing remote URL %q: invalid URL", raw)
		}
		u = u2
	}

	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("parsing remote URL %q: missing hostname", raw)
	}
	return host, nil
}

func promptForAuth(hostname string) (*Auth, error) {
	in := bufio.NewReader(os.Stdin)

	fmt.Printf("Username for '%s': ", hostname)
	user, err := in.ReadString('\n')
	if err != nil {
		return nil, err
	}
	user = strings.TrimSpace(user)
	if user == "" {
		return nil, errors.New("empty username")
	}

	fmt.Printf("Token for '%s': ", hostname)
	pw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(string(pw))
	if token == "" {
		return nil, errors.New("empty token")
	}

	return &Auth{Username: user, Password: token}, nil
}

// SaveToKeychain stores credentials for a hostname in the OS keychain.
func SaveToKeychain(hostname string, auth *Auth) error {
	if auth == nil || auth.Username == "" || auth.Password == "" {
		return errors.New("missing username or token")
	}
	if err := keychainSet(hostname, auth); err != nil {
		return err
	}
	_ = rememberCredentialHost(hostname)
	return nil
}

// DeleteFromKeychain removes stored credentials for a hostname.
func DeleteFromKeychain(hostname string) error {
	if err := keychainDelete(hostname); err != nil {
		return err
	}
	_ = forgetCredentialHost(hostname)
	return nil
}

// ListCredentialHosts lists hostnames with stored credentials.
func ListCredentialHosts() ([]string, error) {
	hosts, err := listRememberedCredentialHosts()
	if err != nil {
		// If the index doesn't exist, treat as empty.
		return []string{}, nil
	}
	return hosts, nil
}
