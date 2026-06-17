package tui

import (
	"sort"
	"strings"
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

	scoreExact := getAurScore(pExact, term)
	scorePrefixHigh := getAurScore(pPrefixHigh, term)
	scorePrefixLow := getAurScore(pPrefixLow, term)
	scoreSubHigh := getAurScore(pSubHigh, term)
	scoreSubLow := getAurScore(pSubLow, term)

	if scoreSubHigh <= scoreExact {
		t.Errorf("expected scoreSubHigh (%f) to be higher than scoreExact (%f)", scoreSubHigh, scoreExact)
	}
	if scoreExact <= scorePrefixHigh {
		t.Errorf("expected scoreExact (%f) to be higher than scorePrefixHigh (%f)", scoreExact, scorePrefixHigh)
	}
	if scorePrefixHigh <= scorePrefixLow {
		t.Errorf("expected scorePrefixHigh (%f) to be higher than scorePrefixLow (%f)", scorePrefixHigh, scorePrefixLow)
	}
	if scorePrefixLow <= scoreSubLow {
		t.Errorf("expected scorePrefixLow (%f) to be higher than scoreSubLow (%f)", scorePrefixLow, scoreSubLow)
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
		termLower := strings.ToLower(term)
		aNameLower := strings.ToLower(a.Name)
		bNameLower := strings.ToLower(b.Name)

		// 1. AUR-specific blended reputation sorting
		if a.Source == "AUR" && b.Source == "AUR" {
			aScore := getAurScore(a, term)
			bScore := getAurScore(b, term)
			if aScore != bScore {
				return aScore > bScore
			}
		}

		// 2. Exact match
		aExact := aNameLower == termLower
		bExact := bNameLower == termLower
		if aExact != bExact {
			return aExact
		}

		// 3. Starts with search term
		aStarts := strings.HasPrefix(aNameLower, termLower)
		bStarts := strings.HasPrefix(bNameLower, termLower)
		if aStarts != bStarts {
			return aStarts
		}

		// 4. Official repository priority (anything not AUR/local is official)
		aOfficial := a.Source != "AUR" && a.Source != "local"
		bOfficial := b.Source != "AUR" && b.Source != "local"
		if aOfficial != bOfficial {
			return aOfficial
		}

		// 5. Shorter name length first (relevance)
		if len(a.Name) != len(b.Name) {
			return len(a.Name) < len(b.Name)
		}

		// 6. Alphabetical fallback
		return a.Name < b.Name
	})

	expectedOrder := []string{"google-chrome", "chrome", "chrome-bin", "chrome-git", "my-chrome-theme"}
	for idx, p := range pkgs {
		if p.Name != expectedOrder[idx] {
			t.Errorf("at index %d: expected %s, got %s", idx, expectedOrder[idx], p.Name)
		}
	}
}

