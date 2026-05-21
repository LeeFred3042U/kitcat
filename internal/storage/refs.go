package storage

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/repo"
)

// ReadBranchSHA returns the commit SHA of a local branch ref in refs/heads/.
func ReadBranchSHA(branch string) (string, error) {
	path := filepath.Join(repo.HeadsDir, branch)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

