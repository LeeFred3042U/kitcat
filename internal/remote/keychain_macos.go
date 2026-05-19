//go:build !windows

package remote

import (
	"fmt"
	"os/exec"
	"strings"
)

// macOS implementation using the `security` CLI (avoids cgo).
//
// Storage format:
// - generic password item
// - service: "kitcat"
// - account: hostname
// - password: "<username>\n<token>"

const darwinService = "kitcat"

func keychainGetDarwin(hostname string) (*Auth, error) {
	if !commandExists("security") {
		return nil, nil
	}

	// `security find-generic-password -s kitcat -a <hostname> -w`
	// prints the password to stdout if found.
	out, err := runTrimmed("security", "find-generic-password", "-s", darwinService, "-a", hostname, "-w")
	if err != nil {
		// Not found is not an error for our resolution chain.
		// security returns a non-zero exit code.
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

func keychainSetDarwin(hostname string, auth *Auth) error {
	if !commandExists("security") {
		return fmt.Errorf("%w: 'security' not found", errKeychainUnavailable)
	}
	if auth == nil || auth.Username == "" || auth.Password == "" {
		return nil
	}

	// Delete any existing entry; ignore errors.
	_ = exec.Command("security", "delete-generic-password", "-s", darwinService, "-a", hostname).Run()

	secret := auth.Username + "\n" + auth.Password
	_, err := runTrimmed("security", "add-generic-password", "-U", "-s", darwinService, "-a", hostname, "-w", secret)
	return err
}

func keychainDeleteDarwin(hostname string) error {
	if !commandExists("security") {
		return fmt.Errorf("%w: 'security' not found", errKeychainUnavailable)
	}
	_, err := runTrimmed("security", "delete-generic-password", "-s", darwinService, "-a", hostname)
	return err
}

