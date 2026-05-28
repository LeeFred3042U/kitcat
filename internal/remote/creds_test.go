package remote

import (
	"testing"
)

func TestHostnameFromRemoteURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantHost string
		wantErr  bool
	}{
		{
			name:     "standard HTTPS GitHub URL",
			url:      "https://github.com/user/repo.git",
			wantHost: "github.com",
		},
		{
			name:     "HTTPS with custom port",
			url:      "https://gitlab.internal.co:8443/group/proj.git",
			wantHost: "gitlab.internal.co",
		},
		{
			name:     "HTTPS URL with embedded auth user",
			url:      "https://token@github.com/user/repo.git",
			wantHost: "github.com",
		},
		{
			name:     "schemeless URL gets https:// prepended",
			url:      "github.com/user/repo.git",
			wantHost: "github.com",
		},
		{
			name:    "empty URL returns error",
			url:     "",
			wantErr: true,
		},
		{
			name:    "whitespace-only URL returns error",
			url:     "   ",
			wantErr: true,
		},
		{
			name:     "HTTP URL without .git suffix",
			url:      "http://gitea.local/org/project",
			wantHost: "gitea.local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hostnameFromRemoteURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Fatalf("hostnameFromRemoteURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.wantHost {
				t.Errorf("hostname = %q, want %q", got, tt.wantHost)
			}
		})
	}
}

func TestResolveAuth_ExplicitWins(t *testing.T) {
	explicit := &Auth{Username: "alice", Password: "tok123"}
	got, err := ResolveAuth("https://github.com/user/repo", explicit)
	if err != nil {
		t.Fatalf("ResolveAuth() unexpected error: %v", err)
	}
	if got.Username != "alice" || got.Password != "tok123" {
		t.Errorf("got user=%q pass=%q, want alice/tok123", got.Username, got.Password)
	}
	if got.Source != AuthSourceExplicit {
		t.Errorf("Source = %q, want %q", got.Source, AuthSourceExplicit)
	}
}
