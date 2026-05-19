//go:build !windows

package remote

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

var errKeychainUnavailable = errors.New("keychain backend unavailable")

func keychainGet(hostname string) (*Auth, error) {
	switch runtime.GOOS {
	case "darwin":
		return keychainGetDarwin(hostname)
	case "linux":
		return keychainGetLinux(hostname)
	default:
		// Windows is implemented in a separate file with build tags.
		return nil, nil
	}
}

func keychainSet(hostname string, auth *Auth) error {
	switch runtime.GOOS {
	case "darwin":
		return keychainSetDarwin(hostname, auth)
	case "linux":
		return keychainSetLinux(hostname, auth)
	default:
		return fmt.Errorf("%w: install a supported keychain helper or use env vars", errKeychainUnavailable)
	}
}

func keychainDelete(hostname string) error {
	switch runtime.GOOS {
	case "darwin":
		return keychainDeleteDarwin(hostname)
	case "linux":
		return keychainDeleteLinux(hostname)
	default:
		return fmt.Errorf("%w: delete not supported on this platform", errKeychainUnavailable)
	}
}

func keychainListHostnames() ([]string, error) {
	switch runtime.GOOS {
	case "linux":
		return keychainListLinux()
	case "darwin":
		// The `security` CLI does not provide a clean, stable way to enumerate all
		// matching generic-password items without parsing `dump-keychain`, which is
		// brittle. Keep list as best-effort only.
		return nil, fmt.Errorf("%w: listing is not supported on macOS backend", errKeychainUnavailable)
	default:
		return nil, fmt.Errorf("%w: listing is not supported on this platform", errKeychainUnavailable)
	}
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func runTrimmed(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		if s == "" {
			return "", err
		}
		return "", fmt.Errorf("%s: %s", err.Error(), s)
	}
	return s, nil
}
