package tui

import (
	"sort"
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

func TestGetAurScore(t *testing.T) {
	term := "chrome"
	
	pExact := Package{Name: "chrome", Source: "AUR", Votes: 10}
	pPrefixHigh := Package{Name: "chrome-bin", Source: "AUR", Votes: 1000}
	pPrefixLow := Package{Name: "chrome-git", Source: "AUR", Votes: 5}
	pSubHigh := Package{Name: "google-chrome", Source: "AUR", Votes: 5000}
	pSubLow := Package{Name: "my-chrome-theme", Source: "AUR", Votes: 0}

	scoreExact := getUnifiedScore(pExact, term)
	scorePrefixHigh := getUnifiedScore(pPrefixHigh, term)
	scorePrefixLow := getUnifiedScore(pPrefixLow, term)
	scoreSubHigh := getUnifiedScore(pSubHigh, term)
	scoreSubLow := getUnifiedScore(pSubLow, term)

	if scoreExact <= scorePrefixHigh {
		t.Errorf("expected scoreExact (%f) to be higher than scorePrefixHigh (%f)", scoreExact, scorePrefixHigh)
	}
	if scorePrefixHigh <= scorePrefixLow {
		t.Errorf("expected scorePrefixHigh (%f) to be higher than scorePrefixLow (%f)", scorePrefixHigh, scorePrefixLow)
	}
	if scorePrefixLow <= scoreSubHigh {
		t.Errorf("expected scorePrefixLow (%f) to be higher than scoreSubHigh (%f)", scorePrefixLow, scoreSubHigh)
	}
	if scoreSubHigh <= scoreSubLow {
		t.Errorf("expected scoreSubHigh (%f) to be higher than scoreSubLow (%f)", scoreSubHigh, scoreSubLow)
	}
}

func TestAurSorting(t *testing.T) {
	term := "chrome"
	
	pExact := Package{Name: "chrome", Source: "AUR", Votes: 10}
	pPrefixHigh := Package{Name: "chrome-bin", Source: "AUR", Votes: 1000}
	pPrefixLow := Package{Name: "chrome-git", Source: "AUR", Votes: 5}
	pSubHigh := Package{Name: "google-chrome", Source: "AUR", Votes: 5000}
	pSubLow := Package{Name: "my-chrome-theme", Source: "AUR", Votes: 0}

	pkgs := []Package{pSubLow, pPrefixLow, pPrefixHigh, pExact, pSubHigh}

	sort.Slice(pkgs, func(i, j int) bool {
		a, b := pkgs[i], pkgs[j]
		if a.IsInstalled != b.IsInstalled {
			return a.IsInstalled
		}
		aScore := getUnifiedScore(a, term)
		bScore := getUnifiedScore(b, term)
		if aScore != bScore {
			return aScore > bScore
		}
		return a.Name < b.Name
	})

	expectedOrder := []string{"chrome", "chrome-bin", "chrome-git", "google-chrome", "my-chrome-theme"}
	for idx, p := range pkgs {
		if p.Name != expectedOrder[idx] {
			t.Errorf("at index %d: expected %s, got %s", idx, expectedOrder[idx], p.Name)
		}
	}
}

