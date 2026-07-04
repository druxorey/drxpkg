// Package pkglist provides functionality to read, write, and manipulate the tracked packages list file.
package pkglist

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)


func TestPkglistLoadSave(t *testing.T) {
	tempDir := t.TempDir()

	pm := NewPackageMap()
	pm["server_packages"] = []string{"nginx", "docker"}
	pm["desktop_packages"] = []string{"firefox"}
	pm["minimal_packages"] = []string{"htop"}
	pm["new_packages"] = []string{"neovim"}
	pm["lol"] = []string{"package_lol"}
	pm["legacy"] = []string{"package_legacy"}
	pm["juanito5454_packagescool"] = []string{"cool_package"}

	if err := Save(tempDir, "packages.list", pm); err != nil {
		t.Fatalf("failed to save pkglist: %v", err)
	}

	expectedFile := filepath.Join(tempDir, DefaultPackagesFileName)
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Fatalf("expected packages file to exist at %s", expectedFile)
	}

	loaded, err := Load(tempDir, "packages.list")
	if err != nil {
		t.Fatalf("failed to load pkglist: %v", err)
	}

	if len(loaded["server_packages"]) != 2 {
		t.Errorf("expected 2 server packages, got %d", len(loaded["server_packages"]))
	}
	if loaded["desktop_packages"][0] != "firefox" {
		t.Errorf("expected firefox in desktop list, got %s", loaded["desktop_packages"][0])
	}
	if loaded["lol"][0] != "package_lol" {
		t.Errorf("expected package_lol in lol list, got %s", loaded["lol"][0])
	}

	cat, full, found := FindPackageLocation("nginx", loaded)
	if !found || cat != "server_packages" || full != "nginx" {
		t.Errorf("failed to find nginx, got cat=%s, full=%s, found=%t", cat, full, found)
	}

	cat, full, found = FindPackageLocation("firefox-bin", loaded)
	if !found || cat != "desktop_packages" || full != "firefox" {
		t.Errorf("failed to find firefox-bin mapping to firefox, got cat=%s, full=%s, found=%t", cat, full, found)
	}

	cat, full, found = FindPackageLocation("cool_package", loaded)
	if !found || cat != "juanito5454_packagescool" || full != "cool_package" {
		t.Errorf("failed to find cool_package, got cat=%s, full=%s, found=%t", cat, full, found)
	}

	if err := AddPackage(tempDir, "packages.list", "git"); err != nil {
		t.Fatalf("failed to add package: %v", err)
	}
	loaded, _ = Load(tempDir, "packages.list")
	if !slicesContains(loaded["new_packages"], "git") {
		t.Errorf("expected git to be added to new list, got %v", loaded["new_packages"])
	}

	if err := AddPackage(tempDir, "packages.list", "nginx"); err != nil {
		t.Fatalf("failed duplicate add test: %v", err)
	}
	loaded, _ = Load(tempDir, "packages.list")
	if slicesContains(loaded["new_packages"], "nginx") {
		t.Errorf("expected nginx to not be added to new_packages as it already exists in server_packages")
	}

	if err := RemovePackage(tempDir, "packages.list", "firefox"); err != nil {
		t.Fatalf("failed to remove package: %v", err)
	}
	loaded, _ = Load(tempDir, "packages.list")
	if len(loaded["desktop_packages"]) != 0 {
		t.Errorf("expected desktop list to be empty after remove, got %v", loaded["desktop_packages"])
	}

	if err := RemovePackage(tempDir, "packages.list", "cool_package"); err != nil {
		t.Fatalf("failed to remove package from custom array: %v", err)
	}
	loaded, _ = Load(tempDir, "packages.list")
	if len(loaded["juanito5454_packagescool"]) != 0 {
		t.Errorf("expected juanito5454_packagescool list to be empty after remove, got %v", loaded["juanito5454_packagescool"])
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


func TestPkglistArrayOrder(t *testing.T) {
	tempDir := t.TempDir()

	// Write initial file with specific order and new_packages not at the end
	initialContent := `#!/bin/bash

new_packages=(
	zpkg
	apkg
)

lol=(
	xpkg
	cpkg
)

legacy=(
	bpkg
)
`
	filePath := filepath.Join(tempDir, "packages.list")
	if err := os.WriteFile(filePath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}

	// Load
	pm, err := Load(tempDir, "packages.list")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	// Verify loaded package counts and contents (unsorted since load just reads them as is)
	if len(pm["new_packages"]) != 2 || pm["new_packages"][0] != "zpkg" {
		t.Errorf("expected loaded packages, got %v", pm["new_packages"])
	}

	// Save back to file (this should sort packages alphabetically inside each array,
	// keep lol before legacy, and move new_packages to the end).
	if err := Save(tempDir, "packages.list", pm); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Read raw saved content to verify the order of the arrays and the sorting of packages
	savedBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	savedContent := string(savedBytes)

	expectedOrder := []string{"lol", "legacy", "new_packages"}
	// Check order of occurrences of array definitions
	var indices []int
	for _, arr := range expectedOrder {
		idx := strings.Index(savedContent, arr+"=(")
		if idx == -1 {
			t.Fatalf("expected array definition for %s not found in saved content: %s", arr, savedContent)
		}
		indices = append(indices, idx)
	}

	for i := 0; i < len(indices)-1; i++ {
		if indices[i] > indices[i+1] {
			t.Errorf("incorrect array order: %s should come before %s in %s", expectedOrder[i], expectedOrder[i+1], savedContent)
		}
	}

	// Verify packages are sorted alphabetically inside the arrays.
	idxCpkg := strings.Index(savedContent, "cpkg")
	idxXpkg := strings.Index(savedContent, "xpkg")
	if idxCpkg == -1 || idxXpkg == -1 || idxCpkg > idxXpkg {
		t.Errorf("lol packages not sorted alphabetically or not found: cpkg index %d, xpkg index %d", idxCpkg, idxXpkg)
	}

	idxApkg := strings.Index(savedContent, "apkg")
	idxZpkg := strings.Index(savedContent, "zpkg")
	if idxApkg == -1 || idxZpkg == -1 || idxApkg > idxZpkg {
		t.Errorf("new_packages packages not sorted alphabetically or not found: apkg index %d, zpkg index %d", idxApkg, idxZpkg)
	}
}
