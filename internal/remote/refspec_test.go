package remote

import (
	"testing"
)

func TestParseRefspec(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantSrc   string
		wantDst   string
		wantForce bool
		wantErr   bool
	}{
		{
			name:      "standard wildcard refspec",
			input:     "refs/heads/*:refs/remotes/origin/*",
			wantSrc:   "refs/heads/*",
			wantDst:   "refs/remotes/origin/*",
			wantForce: false,
			wantErr:   false,
		},
		{
			name:      "forced refspec with plus prefix",
			input:     "+refs/heads/main:refs/remotes/origin/main",
			wantSrc:   "refs/heads/main",
			wantDst:   "refs/remotes/origin/main",
			wantForce: true,
			wantErr:   false,
		},
		{
			name:      "branch name only without colon",
			input:     "main",
			wantSrc:   "main",
			wantDst:   "",
			wantForce: false,
			wantErr:   false,
		},
		{
			name:    "empty string is rejected",
			input:   "",
			wantErr: true,
		},
		{
			name:    "colon with empty destination is rejected",
			input:   "refs/heads/main:",
			wantErr: true,
		},
		{
			name:      "whitespace around spec is trimmed",
			input:     "  refs/heads/dev:refs/remotes/origin/dev  ",
			wantSrc:   "refs/heads/dev",
			wantDst:   "refs/remotes/origin/dev",
			wantForce: false,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRefspec(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseRefspec(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.Src != tt.wantSrc {
				t.Errorf("Src = %q, want %q", got.Src, tt.wantSrc)
			}
			if got.Dst != tt.wantDst {
				t.Errorf("Dst = %q, want %q", got.Dst, tt.wantDst)
			}
			if got.Force != tt.wantForce {
				t.Errorf("Force = %v, want %v", got.Force, tt.wantForce)
			}
		})
	}
}

func TestExpandRefspec(t *testing.T) {
	remoteRefs := []RefInfo{
		{SHA: "aaa", Name: "refs/heads/main"},
		{SHA: "bbb", Name: "refs/heads/dev"},
		{SHA: "ccc", Name: "refs/tags/v1.0"},
	}

	tests := []struct {
		name      string
		spec      Refspec
		wantCount int
		wantFirst RefMapping
		wantErr   bool
	}{
		{
			name: "wildcard expands matching branches",
			spec: Refspec{
				Src: "refs/heads/*",
				Dst: "refs/remotes/origin/*",
			},
			wantCount: 2,
			wantFirst: RefMapping{
				Src: "refs/heads/main",
				Dst: "refs/remotes/origin/main",
			},
		},
		{
			name: "non-wildcard maps directly",
			spec: Refspec{
				Src: "refs/heads/main",
				Dst: "refs/remotes/origin/main",
			},
			wantCount: 1,
			wantFirst: RefMapping{
				Src: "refs/heads/main",
				Dst: "refs/remotes/origin/main",
			},
		},
		{
			name: "non-wildcard without dst is an error",
			spec: Refspec{
				Src: "refs/heads/main",
				Dst: "",
			},
			wantErr: true,
		},
		{
			name: "wildcard with no matches returns empty",
			spec: Refspec{
				Src: "refs/notes/*",
				Dst: "refs/remotes/origin/notes/*",
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandRefspec(tt.spec, remoteRefs)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ExpandRefspec() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != tt.wantCount {
				t.Fatalf("got %d mappings, want %d", len(got), tt.wantCount)
			}
			if tt.wantCount > 0 && got[0] != tt.wantFirst {
				t.Errorf("first mapping = %+v, want %+v", got[0], tt.wantFirst)
			}
		})
	}
}
