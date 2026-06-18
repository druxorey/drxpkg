package tui

import (
	"sort"
	"testing"

	"github.com/druxorey/drxpkg/internal/config"
	"github.com/rivo/tview"
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

	if scoreExact <= scoreSubHigh {
		t.Errorf("expected scoreExact (%f) to be higher than scoreSubHigh (%f)", scoreExact, scoreSubHigh)
	}
	if scoreSubHigh <= scorePrefixHigh {
		t.Errorf("expected scoreSubHigh (%f) to be higher than scorePrefixHigh (%f)", scoreSubHigh, scorePrefixHigh)
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

	expectedOrder := []string{"chrome", "google-chrome", "chrome-bin", "chrome-git", "my-chrome-theme"}
	for idx, p := range pkgs {
		if p.Name != expectedOrder[idx] {
			t.Errorf("at index %d: expected %s, got %s", idx, expectedOrder[idx], p.Name)
		}
	}
}

func TestPerformLocalSearch(t *testing.T) {
	conf := &config.Settings{
		MaxResults: 10,
	}

	ui := &UI{
		conf:       conf,
		pkgTable:   tview.NewTable(),
		statusText: tview.NewTextView(),
		pkgsCache: []cachedPkg{
			{
				Package:          Package{Name: "firefox", Source: "extra", IsInstalled: false},
				Description:      "Fast, Private and Safe Web Browser",
				NameLower:        "firefox",
				DescriptionLower: "fast, private and safe web browser",
			},
			{
				Package:          Package{Name: "firefox-developer-edition", Source: "extra", IsInstalled: true},
				Description:      "Developer edition of Firefox",
				NameLower:        "firefox-developer-edition",
				DescriptionLower: "developer edition of firefox",
			},
			{
				Package:          Package{Name: "chromium", Source: "extra", IsInstalled: false},
				Description:      "A web browser built for speed, simplicity, and security",
				NameLower:        "chromium",
				DescriptionLower: "a web browser built for speed, simplicity, and security",
			},
		},
	}

	// Test exact match / filtering
	ui.performLocalSearch("firefox")
	if len(ui.shownPackages) != 2 {
		t.Fatalf("expected 2 packages matching 'firefox', got %d", len(ui.shownPackages))
	}

	// The installed package (firefox-developer-edition) should be sorted first
	if ui.shownPackages[0].Name != "firefox-developer-edition" {
		t.Errorf("expected firefox-developer-edition to be first, got %s", ui.shownPackages[0].Name)
	}

	// Test description match
	ui.performLocalSearch("speed")
	if len(ui.shownPackages) != 1 {
		t.Fatalf("expected 1 package matching 'speed', got %d", len(ui.shownPackages))
	}
	if ui.shownPackages[0].Name != "chromium" {
		t.Errorf("expected chromium, got %s", ui.shownPackages[0].Name)
	}

	// Test empty term
	ui.performLocalSearch("")
	if len(ui.shownPackages) != 0 {
		t.Errorf("expected 0 packages on empty search, got %d", len(ui.shownPackages))
	}
}


