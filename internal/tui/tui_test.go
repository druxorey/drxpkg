package tui

import (
	"testing"
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
		url := GetPkgbuildUrl(test.source, test.base)
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
		res := encodePackageGitlabUrl(test.input)
		if res != test.expected {
			t.Errorf("encodePackageGitlabUrl(%q) = %q; expected %q", test.input, res, test.expected)
		}
	}
}
