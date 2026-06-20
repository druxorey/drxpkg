// Package tui manages the Terminal User Interface, layouts, user input, and screen rendering.
package tui

import (
	"testing"

	"github.com/druxorey/drxpkg/internal/config"
	"github.com/druxorey/drxpkg/internal/pkgmgr"
	"github.com/rivo/tview"
)

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
				Package:          pkgmgr.Package{Name: "firefox", Source: "extra", IsInstalled: false},
				Description:      "Fast, Private and Safe Web Browser",
				NameLower:        "firefox",
				DescriptionLower: "fast, private and safe web browser",
			},
			{
				Package:          pkgmgr.Package{Name: "firefox-developer-edition", Source: "extra", IsInstalled: true},
				Description:      "Developer edition of Firefox",
				NameLower:        "firefox-developer-edition",
				DescriptionLower: "developer edition of firefox",
			},
			{
				Package:          pkgmgr.Package{Name: "chromium", Source: "extra", IsInstalled: false},
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
