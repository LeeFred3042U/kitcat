package remote

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscoverRefs_HTTPStatusErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		checkErr   func(error) bool
		errSubstr  string
	}{
		{
			name:       "401 maps to ErrAuthRequired",
			statusCode: http.StatusUnauthorized,
			checkErr:   func(err error) bool { return errors.Is(err, ErrAuthRequired) },
		},
		{
			name:       "403 maps to ErrAuthDenied",
			statusCode: http.StatusForbidden,
			checkErr:   func(err error) bool { return errors.Is(err, ErrAuthDenied) },
		},
		{
			name:       "404 returns repository not found",
			statusCode: http.StatusNotFound,
			checkErr:   func(err error) bool { return strings.Contains(err.Error(), "repository not found") },
		},
		{
			name:       "500 returns unexpected status",
			statusCode: http.StatusInternalServerError,
			checkErr:   func(err error) bool { return strings.Contains(err.Error(), "unexpected HTTP status 500") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			_, _, err := discoverRefs(server.URL, "git-upload-pack", nil)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !tt.checkErr(err) {
				t.Errorf("error = %v, did not match expected check", err)
			}
		})
	}
}

func TestDiscoverRefs_BasicAuthSent(t *testing.T) {
	var gotUser, gotPass string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, _ = r.BasicAuth()
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	auth := &Auth{Username: "testuser", Password: "testtoken"}
	_, _, _ = discoverRefs(server.URL, "git-upload-pack", auth)

	if gotUser != "testuser" {
		t.Errorf("BasicAuth username = %q, want %q", gotUser, "testuser")
	}
	if gotPass != "testtoken" {
		t.Errorf("BasicAuth password = %q, want %q", gotPass, "testtoken")
	}
}
