// Package pkgmgr provides package management backend operations for AUR and local Pacman (ALPM) databases.
package pkgmgr

import (
	"strings"
)

// GetUnifiedScore calculates a search relevance score for packages.
func GetUnifiedScore(p Package, term string) float64 {
	nameLower := strings.ToLower(p.Name)
	termLower := strings.ToLower(term)

	var matchScore float64
	if nameLower == termLower {
		matchScore = 1000000.0
	} else if strings.HasPrefix(nameLower, termLower) {
		matchScore = 30000.0
	} else if strings.Contains(nameLower, termLower) {
		matchScore = 10000.0
	} else {
		matchScore = 1000.0 // Description match
	}

	// Source trust bonus (+5,000 for official repositories)
	var sourceBonus float64
	if p.Source != "AUR" && p.Source != "local" {
		sourceBonus = 5000.0
	}

	// Reputation (AUR votes) heavily weighted
	reputation := float64(p.Votes) * 30.0

	// Name length tie-breaker (shorter names get a small bonus)
	nameLen := len(p.Name)
	if nameLen == 0 {
		nameLen = 1
	}
	lengthBonus := 100.0 / float64(nameLen)

	return matchScore + sourceBonus + reputation + lengthBonus
}
