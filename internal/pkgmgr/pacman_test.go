// Package pkgmgr provides package management backend operations for AUR and local Pacman (ALPM) databases.
package pkgmgr

import (
	"testing"
)

func TestPacmanNilHandle(t *testing.T) {
	// Test behavior of ALPM wrapper functions under a nil handle
	infoRes := InfoPacman(nil, "any-package")
	if infoRes.Error != "alpm handle is nil" {
		t.Errorf("expected 'alpm handle is nil' error, got: %s", infoRes.Error)
	}

	// Should not panic
	AddLocalSatisfiers(nil, InfoRecord{
		Depends: []string{"dep"},
	})
}
