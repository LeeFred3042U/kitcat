package remote

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func credentialHostsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".kitcat/credential_hosts"
	}
	return filepath.Join(home, ".kitcat", "credential_hosts")
}

func rememberCredentialHost(hostname string) error {
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return nil
	}

	hosts, _ := listRememberedCredentialHosts()
	for _, h := range hosts {
		if h == hostname {
			return nil
		}
	}
	hosts = append(hosts, hostname)
	sort.Strings(hosts)

	path := credentialHostsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(hosts, "\n")+"\n"), 0o644)
}

func forgetCredentialHost(hostname string) error {
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return nil
	}

	hosts, _ := listRememberedCredentialHosts()
	out := hosts[:0]
	for _, h := range hosts {
		if h != hostname {
			out = append(out, h)
		}
	}
	path := credentialHostsPath()
	if len(out) == 0 {
		_ = os.Remove(path)
		return nil
	}
	return os.WriteFile(path, []byte(strings.Join(out, "\n")+"\n"), 0o644)
}

func listRememberedCredentialHosts() ([]string, error) {
	path := credentialHostsPath()
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var hosts []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		h := strings.TrimSpace(sc.Text())
		if h != "" {
			hosts = append(hosts, h)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	sort.Strings(hosts)
	return hosts, nil
}

