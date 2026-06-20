// Package pkglist provides functionality to read, write, and manipulate the tracked packages list file.
package pkglist

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPkglistLoadSave(t *testing.T) {
	tempDir := t.TempDir()

	pm := NewPackageMap()
	pm["server"] = []string{"nginx", "docker"}
	pm["desktop"] = []string{"firefox"}
	pm["minimal"] = []string{"htop"}
	pm["new"] = []string{"neovim"}

	if err := Save(tempDir, pm); err != nil {
		t.Fatalf("failed to save pkglist: %v", err)
	}

	expectedFile := filepath.Join(tempDir, PackagesFileName)
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Fatalf("expected packages file to exist at %s", expectedFile)
	}

	loaded, err := Load(tempDir)
	if err != nil {
		t.Fatalf("failed to load pkglist: %v", err)
	}

	if len(loaded["server"]) != 2 {
		t.Errorf("expected 2 server packages, got %d", len(loaded["server"]))
	}
	if loaded["desktop"][0] != "firefox" {
		t.Errorf("expected firefox in desktop list, got %s", loaded["desktop"][0])
	}

	cat, full, found := FindPackageLocation("nginx", loaded)
	if !found || cat != "server" || full != "nginx" {
		t.Errorf("failed to find nginx, got cat=%s, full=%s, found=%t", cat, full, found)
	}

	cat, full, found = FindPackageLocation("firefox-bin", loaded)
	if !found || cat != "desktop" || full != "firefox" {
		t.Errorf("failed to find firefox-bin mapping to firefox, got cat=%s, full=%s, found=%t", cat, full, found)
	}

	if err := AddPackage(tempDir, "git"); err != nil {
		t.Fatalf("failed to add package: %v", err)
	}
	loaded, _ = Load(tempDir)
	if !slicesContains(loaded["new"], "git") {
		t.Errorf("expected git to be added to new list, got %v", loaded["new"])
	}

	if err := AddPackage(tempDir, "nginx"); err != nil {
		t.Fatalf("failed duplicate add test: %v", err)
	}
	loaded, _ = Load(tempDir)
	if len(loaded["new"]) != 2 {
		t.Errorf("expected 2 new packages after duplicate add, got %d", len(loaded["new"]))
	}

	if err := RemovePackage(tempDir, "firefox"); err != nil {
		t.Fatalf("failed to remove package: %v", err)
	}
	loaded, _ = Load(tempDir)
	if len(loaded["desktop"]) != 0 {
		t.Errorf("expected desktop list to be empty after remove, got %v", loaded["desktop"])
	}
}

func slicesContains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
