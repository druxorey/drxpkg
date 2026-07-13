// Package pkgmgr provides package management backend operations for AUR and local Pacman (ALPM) databases.
package pkgmgr

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)


func TestGetPkgbuildUrl(t *testing.T) {
	tests := []struct {
		source   string
		base     string
		expected string
	}{
		{"AUR", "yay", "https://aur.archlinux.org/cgit/aur.git/plain/PKGBUILD?h=yay"},
		{"core", "linux", "https://gitlab.archlinux.org/archlinux/packaging/packages/linux/-/raw/main/PKGBUILD"},
		{"extra", "glibc+arch", "https://gitlab.archlinux.org/archlinux/packaging/packages/glibc-arch/-/raw/main/PKGBUILD"},
	}

	for _, test := range tests {
		url := GetPkgbuildURL(test.source, test.base)
		if url != test.expected {
			t.Errorf("GetPkgbuildUrl(%q, %q) = %q; expected %q", test.source, test.base, url, test.expected)
		}
	}
}


func TestGitlabUrlEncoding(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"tree", "unix-tree"},
		{"plus+name", "plus-name"},
		{"my_package", "my_package"},
		{"my__package", "my-package"},
	}

	for _, test := range tests {
		res := encodePackageGitlabURL(test.input)
		if res != test.expected {
			t.Errorf("encodePackageGitlabUrl(%q) = %q; expected %q", test.input, res, test.expected)
		}
	}
}

func TestGetPkgbuildContentValidation(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		contentType string
		body        string
		expectError bool
		expected    string
	}{
		{
			name:        "Valid raw PKGBUILD",
			statusCode:  200,
			contentType: "text/plain",
			body:        "pkgname=hello",
			expectError: false,
			expected:    "pkgname=hello",
		},
		{
			name:        "HTML redirect page / login page",
			statusCode:  200,
			contentType: "text/html; charset=utf-8",
			body:        "<html><body>Sign In</body></html>",
			expectError: true,
		},
		{
			name:        "404 Not Found status code",
			statusCode:  404,
			contentType: "text/plain",
			body:        "Not Found",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			content, err := GetPkgbuildContent(server.URL, 5*time.Second)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if content != tc.expected {
					t.Errorf("expected content %q, got %q", tc.expected, content)
				}
			}
		})
	}
}
