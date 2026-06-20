// Package pkgmgr provides package management backend operations for AUR and local Pacman (ALPM) databases.
package pkgmgr

import (
	"testing"
)

func TestParseUpdateLine(t *testing.T) {
	tests := []struct {
		line     string
		expected *UpdatePackage
		hasError bool
	}{
		{
			line: "brave-bin 1:1.91.172-1 -> 1:1.91.175-1",
			expected: &UpdatePackage{
				Name:         "brave-bin",
				LocalVersion: "1:1.91.172-1",
				NewVersion:   "1:1.91.175-1",
				Selected:     true,
			},
			hasError: false,
		},
		{
			line: "yay 12.6.0-1 -> 13.0.0-1",
			expected: &UpdatePackage{
				Name:         "yay",
				LocalVersion: "12.6.0-1",
				NewVersion:   "13.0.0-1",
				Selected:     true,
			},
			hasError: false,
		},
		{
			line:     "invalid line format",
			expected: nil,
			hasError: true,
		},
		{
			line:     "yay 1.0.0",
			expected: nil,
			hasError: true,
		},
	}

	for _, test := range tests {
		res, err := ParseUpdateLine(test.line)
		if (err != nil) != test.hasError {
			t.Errorf("ParseUpdateLine(%q) returned error = %v; expected error = %v", test.line, err != nil, test.hasError)
		}
		if res != nil && test.expected != nil {
			if res.Name != test.expected.Name || res.LocalVersion != test.expected.LocalVersion || res.NewVersion != test.expected.NewVersion || res.Selected != test.expected.Selected {
				t.Errorf("ParseUpdateLine(%q) = %+v; expected %+v", test.line, res, test.expected)
			}
		}
	}
}
