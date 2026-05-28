package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsValidRefName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "simple branch name", input: "main", want: true},
		{name: "hyphenated name", input: "feature-login", want: true},
		{name: "path traversal rejected", input: "../etc/passwd", want: false},
		{name: "backslash rejected", input: "feature\\branch", want: false},
		{name: "forward slash rejected", input: "feature/branch", want: false},
		{name: "space in name rejected", input: "my branch", want: false},
		{name: "dot-dot in middle rejected", input: "a..b", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidRefName(tt.input)
			if got != tt.want {
				t.Errorf("IsValidRefName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsSafePath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "relative safe path", input: "src/main.go", want: true},
		{name: "simple filename", input: "README.md", want: true},
		{name: "absolute path rejected", input: "/etc/passwd", want: false},
		{name: "parent traversal rejected", input: "../secret", want: false},
		{name: "nested traversal rejected", input: "a/../../b", want: false},
		{name: "dot-only is safe", input: ".", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSafePath(tt.input)
			if got != tt.want {
				t.Errorf("IsSafePath(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitKey(t *testing.T) {
	tests := []struct {
		name           string
		fullKey        string
		wantSection    string
		wantSubsection string
		wantKey        string
		wantErr        bool
	}{
		{
			name:           "two-part key",
			fullKey:        "user.name",
			wantSection:    "user",
			wantSubsection: "",
			wantKey:        "name",
		},
		{
			name:           "three-part key with subsection",
			fullKey:        "remote.origin.url",
			wantSection:    "remote",
			wantSubsection: "origin",
			wantKey:        "url",
		},
		{
			name:           "four-part key joins middle as subsection",
			fullKey:        "branch.feature.login.merge",
			wantSection:    "branch",
			wantSubsection: "feature.login",
			wantKey:        "merge",
		},
		{
			name:    "single word is invalid",
			fullKey: "nosection",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sec, sub, key, err := splitKey(tt.fullKey)
			if (err != nil) != tt.wantErr {
				t.Fatalf("splitKey(%q) error = %v, wantErr %v", tt.fullKey, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if sec != tt.wantSection {
				t.Errorf("section = %q, want %q", sec, tt.wantSection)
			}
			if sub != tt.wantSubsection {
				t.Errorf("subsection = %q, want %q", sub, tt.wantSubsection)
			}
			if key != tt.wantKey {
				t.Errorf("key = %q, want %q", key, tt.wantKey)
			}
		})
	}
}

func TestParseSectionHeader(t *testing.T) {
	tests := []struct {
		name           string
		line           string
		wantSection    string
		wantSubsection string
		wantIsSection  bool
	}{
		{
			name:           "simple section",
			line:           "[core]",
			wantSection:    "core",
			wantSubsection: "",
			wantIsSection:  true,
		},
		{
			name:           "section with quoted subsection",
			line:           `[remote "origin"]`,
			wantSection:    "remote",
			wantSubsection: "origin",
			wantIsSection:  true,
		},
		{
			name:           "section with quoted subsection and branch",
			line:           `[branch "main"]`,
			wantSection:    "branch",
			wantSubsection: "main",
			wantIsSection:  true,
		},
		{
			name:          "not a section header",
			line:          "url = https://github.com",
			wantIsSection: false,
		},
		{
			name:          "empty string",
			line:          "",
			wantIsSection: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sec, sub, ok := parseSectionHeader(tt.line)
			if ok != tt.wantIsSection {
				t.Fatalf("parseSectionHeader(%q) isSection = %v, want %v", tt.line, ok, tt.wantIsSection)
			}
			if !ok {
				return
			}
			if sec != tt.wantSection {
				t.Errorf("section = %q, want %q", sec, tt.wantSection)
			}
			if sub != tt.wantSubsection {
				t.Errorf("subsection = %q, want %q", sub, tt.wantSubsection)
			}
		})
	}
}

func setupConfigFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	kitcatDir := filepath.Join(tmpDir, ".kitcat")
	if err := os.MkdirAll(kitcatDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(kitcatDir, "config")
	if content != "" {
		if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	origDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origDir) })
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	return tmpDir
}
