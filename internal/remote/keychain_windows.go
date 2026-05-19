//go:build windows

package remote

import (
	"errors"
	"strings"

	"github.com/danieljoos/wincred"
)

const winPrefix = "kitcat:"

func keychainGet(hostname string) (*Auth, error) {
	cred, err := wincred.GetGenericCredential(winPrefix + hostname)
	if err != nil {
		if errors.Is(err, wincred.ErrElementNotFound) {
			return nil, nil
		}
		return nil, err
	}
	user := strings.TrimSpace(cred.UserName)
	token := strings.TrimSpace(string(cred.CredentialBlob))
	if user == "" || token == "" {
		return nil, nil
	}
	return &Auth{Username: user, Password: token}, nil
}

func keychainSet(hostname string, auth *Auth) error {
	if auth == nil || auth.Username == "" || auth.Password == "" {
		return nil
	}
	cred := wincred.NewGenericCredential(winPrefix + hostname)
	cred.UserName = auth.Username
	cred.CredentialBlob = []byte(auth.Password)
	return cred.Write()
}

func keychainDelete(hostname string) error {
	// Delete returns ErrElementNotFound if missing; ignore.
	err := wincred.DeleteGenericCredential(winPrefix + hostname)
	if errors.Is(err, wincred.ErrElementNotFound) {
		return nil
	}
	return err
}

func keychainListHostnames() ([]string, error) {
	creds, err := wincred.List()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var out []string
	for _, c := range creds {
		if c == nil || c.TargetName == "" {
			continue
		}
		if !strings.HasPrefix(c.TargetName, winPrefix) {
			continue
		}
		host := strings.TrimPrefix(c.TargetName, winPrefix)
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, host)
	}
	return out, nil
}

