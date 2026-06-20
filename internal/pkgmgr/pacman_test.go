// Package pkgmgr provides package management backend operations for AUR and local Pacman (ALPM) databases.
package pkgmgr

import (
	"testing"
)

func TestInitPacmanDbsInvalidPath(t *testing.T) {
	// Initialize with invalid paths and expect errors
	handle, err := InitPacmanDbs("/nonexistent/db", "/nonexistent/pacman.conf")
	if err == nil {
		if handle != nil {
			_ = handle.Release()
		}
		t.Fatalf("expected error when initializing with invalid paths, but got nil")
	}
}

func TestPacmanNilHandle(t *testing.T) {
	// Test behavior of ALPM wrapper functions under a nil handle
	if IsPackageInstalled(nil, "any-package") {
		t.Errorf("IsPackageInstalled should return false on a nil handle")
	}

	infoRes := InfoPacman(nil, "any-package")
	if infoRes.Error != "alpm handle is nil" {
		t.Errorf("expected 'alpm handle is nil' error, got: %s", infoRes.Error)
	}

	// Should not panic
	AddLocalSatisfiers(nil, InfoRecord{
		Depends: []string{"dep"},
	})
}
