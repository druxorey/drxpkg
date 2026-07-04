// Package pkgmgr provides package management backend operations for AUR and local Pacman (ALPM) databases.
package pkgmgr

import (
	"sort"
	"strings"
	"testing"
)


// Helper function to sort packages with the same logic as UI.performSearch
func sortPackages(pkgs []Package, term string) []Package {
	var filtered []Package
	termLower := strings.ToLower(term)

	for _, p := range pkgs {
		nameLower := strings.ToLower(p.Name)

		// Filter out packages that don't match the query
		// For the test, we'll assume chromium and libcamera match "chrome" via description
		isMatch := strings.Contains(nameLower, termLower)
		if term == "chrome" && (p.Name == "chromium" || p.Name == "libcamera") {
			isMatch = true
		}

		if isMatch {
			filtered = append(filtered, p)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		a, b := filtered[i], filtered[j]
		if a.IsInstalled != b.IsInstalled {
			return a.IsInstalled
		}
		aScore := GetUnifiedScore(a, term)
		bScore := GetUnifiedScore(b, term)
		if aScore != bScore {
			return aScore > bScore
		}
		return a.Name < b.Name
	})

	return filtered
}


func TestPopularApplicationsSorting(t *testing.T) {
	// Mock database of popular Linux packages (mix of official and AUR)
	mockPool := []Package{
		// Zen related
		{Name: "zenith", Source: "extra", Votes: 0},
		{Name: "zenity", Source: "extra", Votes: 0},
		{Name: "zen-browser-bin", Source: "AUR", Votes: 450},
		{Name: "libzen", Source: "extra", Votes: 0},
		{Name: "autozen", Source: "AUR", Votes: 12},

		// Chrome related
		{Name: "chromium", Source: "extra", Votes: 0},
		{Name: "google-chrome", Source: "AUR", Votes: 2355},
		{Name: "chrome-remote-desktop", Source: "AUR", Votes: 127},
		{Name: "libcamera", Source: "extra", Votes: 0}, // Matches description-only
		{Name: "chrome-devtools-mcp", Source: "AUR", Votes: 0},

		// Slack related
		{Name: "slack-desktop", Source: "AUR", Votes: 520},
		{Name: "slack-cli", Source: "AUR", Votes: 15},
		{Name: "slack-term", Source: "AUR", Votes: 8},

		// Others
		{Name: "visual-studio-code-bin", Source: "AUR", Votes: 1540},
		{Name: "spotify", Source: "AUR", Votes: 3100},
		{Name: "vlc", Source: "extra", Votes: 0},
		{Name: "firefox", Source: "extra", Votes: 0},
		{Name: "gimp", Source: "extra", Votes: 0},
		{Name: "discord", Source: "extra", Votes: 0},
		{Name: "telegram-desktop", Source: "extra", Votes: 0},
	}

	t.Run("Query: zen", func(t *testing.T) {
		term := "zen"
		results := sortPackages(mockPool, term)

		// zen-browser-bin (starts-with, AUR, 450 votes) score: 30000 + 0 + 450*30 = 43500
		// zenith (starts-with, official) score: 30000 + 5000 = 35000
		// zenity (starts-with, official) score: 30000 + 5000 = 35000

		if results[0].Name != "zen-browser-bin" {
			t.Errorf("expected zen-browser-bin at rank 0, got %s", results[0].Name)
		}
		if results[1].Name != "zenith" && results[1].Name != "zenity" {
			t.Errorf("expected zenith or zenity at rank 1, got %s", results[1].Name)
		}
		if results[2].Name != "zenith" && results[2].Name != "zenity" {
			t.Errorf("expected zenith or zenity at rank 2, got %s", results[2].Name)
		}
	})

	t.Run("Query: chrome", func(t *testing.T) {
		term := "chrome"
		results := sortPackages(mockPool, term)

		// google-chrome (contains, AUR, 2355 votes) score: 10000 + 0 + 2355*30 = 80650
		// chrome-remote-desktop (starts-with, AUR, 127 votes) score: 30000 + 0 + 127*30 = 33810
		// chrome-devtools-mcp (starts-with, AUR, 0 votes) score: 30000 + 0 + 0 = 30000
		// chromium (description-only, official) score: 1000 + 5000 = 6000
		// libcamera (description-only, official) score: 1000 + 5000 = 6000

		// Verify order:
		// 1. google-chrome
		// 2. chrome-remote-desktop
		// 3. chrome-devtools-mcp
		// 4. chromium
		// 5. libcamera

		expected := []string{"google-chrome", "chrome-remote-desktop", "chrome-devtools-mcp", "chromium", "libcamera"}
		for idx, name := range expected {
			if results[idx].Name != name {
				t.Errorf("at index %d: expected %s, got %s", idx, name, results[idx].Name)
			}
		}
	})

	t.Run("Query: chrome with installed packages", func(t *testing.T) {
		term := "chrome"
		customPool := []Package{
			{Name: "chromium", Source: "extra", Votes: 0, IsInstalled: false},
			{Name: "google-chrome", Source: "AUR", Votes: 2355, IsInstalled: false},
			{Name: "chrome-remote-desktop", Source: "AUR", Votes: 127, IsInstalled: false},
			{Name: "my-chrome-theme", Source: "AUR", Votes: 0, IsInstalled: true}, // Installed!
		}

		results := sortPackages(customPool, term)

		if results[0].Name != "my-chrome-theme" {
			t.Errorf("expected installed package my-chrome-theme to be at rank 0, got %s", results[0].Name)
		}

		if results[1].Name != "google-chrome" {
			t.Errorf("expected google-chrome at rank 1, got %s", results[1].Name)
		}
		if results[2].Name != "chrome-remote-desktop" {
			t.Errorf("expected chrome-remote-desktop at rank 2, got %s", results[2].Name)
		}
		if results[3].Name != "chromium" {
			t.Errorf("expected chromium at rank 3, got %s", results[3].Name)
		}
	})

	t.Run("Query: slack", func(t *testing.T) {
		term := "slack"
		results := sortPackages(mockPool, term)

		// slack-desktop (starts-with, AUR, 520 votes) score: 30000 + 520 = 30520
		// slack-cli (starts-with, AUR, 15 votes) score: 30000 + 15 = 30015
		// slack-term (starts-with, AUR, 8 votes) score: 30000 + 8 = 30008

		if results[0].Name != "slack-desktop" {
			t.Errorf("expected slack-desktop at rank 0, got %s", results[0].Name)
		}
		if results[1].Name != "slack-cli" {
			t.Errorf("expected slack-cli at rank 1, got %s", results[1].Name)
		}
		if results[2].Name != "slack-term" {
			t.Errorf("expected slack-term at rank 2, got %s", results[2].Name)
		}
	})
}


func TestGetAurScore(t *testing.T) {
	term := "chrome"

	pExact := Package{Name: "chrome", Source: "AUR", Votes: 10}
	pPrefixHigh := Package{Name: "chrome-bin", Source: "AUR", Votes: 1000}
	pPrefixLow := Package{Name: "chrome-git", Source: "AUR", Votes: 5}
	pSubHigh := Package{Name: "google-chrome", Source: "AUR", Votes: 5000}
	pSubLow := Package{Name: "my-chrome-theme", Source: "AUR", Votes: 0}

	scoreExact := GetUnifiedScore(pExact, term)
	scorePrefixHigh := GetUnifiedScore(pPrefixHigh, term)
	scorePrefixLow := GetUnifiedScore(pPrefixLow, term)
	scoreSubHigh := GetUnifiedScore(pSubHigh, term)
	scoreSubLow := GetUnifiedScore(pSubLow, term)

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
		aScore := GetUnifiedScore(a, term)
		bScore := GetUnifiedScore(b, term)
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
