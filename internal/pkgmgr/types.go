// Package pkgmgr provides package management backend operations for AUR and local Pacman (ALPM) databases.
package pkgmgr

import (
	"fmt"
	"strings"
)

type SearchResults struct {
	Error       string       `json:"error,omitempty"`
	Resultcount int          `json:"resultcount"`
	Results     []InfoRecord `json:"results"`
	Type        string       `json:"type"`
	Version     int          `json:"version"`
}

type InfoRecord struct {
	CheckDepends      []string `json:"CheckDepends,omitempty"`
	Conflicts         []string `json:"Conflicts,omitempty"`
	Depends           []string `json:"Depends,omitempty"`
	Description       string   `json:"Description"`
	FirstSubmitted    int      `json:"FirstSubmitted"`
	Groups            []string `json:"Groups,omitempty"`
	ID                int      `json:"ID"`
	Keywords          []string `json:"Keywords"`
	LastModified      int      `json:"LastModified"`
	License           []string `json:"License"`
	Maintainer        string   `json:"Maintainer"`
	MakeDepends       []string `json:"MakeDepends,omitempty"`
	Name              string   `json:"Name"`
	NumVotes          int      `json:"NumVotes"`
	OptDepends        []string `json:"OptDepends,omitempty"`
	OutOfDate         int      `json:"OutOfDate"`
	PackageBase       string   `json:"PackageBase"`
	PackageBaseID     int      `json:"PackageBaseID"`
	Popularity        float64  `json:"Popularity"`
	Provides          []string `json:"Provides,omitempty"`
	Replaces          []string `json:"Replaces,omitempty"`
	RequiredBy        []string `json:"RequiredBy,omitempty"`
	URL               string   `json:"URL"`
	URLPath           string   `json:"URLPath"`
	Version           string   `json:"Version"`
	LocalVersion      string
	Source            string `json:"Source"`
	Architecture      string `json:"Architecture"`
	IsIgnored         bool
	DepsAndSatisfiers []DependencySatisfier
}

type DependencySatisfier struct {
	DepType   string
	DepName   string
	Satisfier string
	Installed bool
}

type Package struct {
	Name         string
	Source       string
	IsInstalled  bool
	LastModified int
	Popularity   float64
	Votes        int
}

type UpdatePackage struct {
	Name         string
	LocalVersion string
	NewVersion   string
	Source       string
	Selected     bool
	NotInAur     bool
	OutOfDate    bool
}

func ParseUpdateLine(line string) (*UpdatePackage, error) {
	// Format: <pkgname> <current_version> -> <new_version>
	parts := strings.Fields(line)
	if len(parts) < 4 || parts[2] != "->" {
		return nil, fmt.Errorf("invalid line format: %s", line)
	}
	return &UpdatePackage{
		Name:         parts[0],
		LocalVersion: parts[1],
		NewVersion:   parts[3],
		Selected:     true,
	}, nil
}
