// Package pkgmgr provides package management backend operations for AUR and local Pacman (ALPM) databases.
package pkgmgr

import (
	"errors"
	"math"
	"strings"

	"github.com/Jguer/go-alpm/v2"
	pconf "github.com/Morganamilo/go-pacmanconf"
)


func InitPacmanDbs(dbPath, confPath string) (*alpm.Handle, error) {
	h, err := alpm.Initialize("/", dbPath)
	if err != nil {
		return nil, err
	}

	conf, _, err := pconf.ParseFile(confPath)
	if err != nil {
		_ = h.Release()
		return nil, err
	}

	for _, repo := range conf.Repos {
		_, err := h.RegisterSyncDB(repo.Name, 0)
		if err != nil {
			_ = h.Release()
			return nil, err
		}
	}
	_ = h.SetIgnorePkgs(conf.IgnorePkg)
	_ = h.SetIgnoreGroups(conf.IgnoreGroup)

	return h, nil
}


func SearchRepos(h *alpm.Handle, term string, maxResults int) ([]Package, []Package, error) {
	packages := []Package{}
	installed := []Package{}

	if h == nil {
		return packages, installed, errors.New("alpm handle is nil")
	}
	dbs, err := h.SyncDBs()
	if err != nil {
		return packages, installed, err
	}
	local, err := h.LocalDB()
	if err != nil {
		return packages, installed, err
	}

	searchDbs := append(dbs.Slice(), local)
	term = strings.ToLower(term)

	counter := 0
	for _, db := range searchDbs {
		for _, pkg := range db.PkgCache().Slice() {
			if counter >= maxResults {
				break
			}

			if strings.Contains(strings.ToLower(pkg.Name()), term) ||
				strings.Contains(strings.ToLower(pkg.Description()), term) {
				p := Package{
					Name:         pkg.Name(),
					Source:       db.Name(),
					IsInstalled:  local.Pkg(pkg.Name()) != nil,
					LastModified: int(pkg.BuildDate().Unix()),
					Popularity:   math.MaxFloat64,
				}
				if db != local {
					packages = append(packages, p)
				} else {
					installed = append(installed, p)
				}

				counter++
			}
		}
	}
	return packages, installed, nil
}


func IsPackageInstalled(h *alpm.Handle, pkg string) bool {
	if h == nil {
		return false
	}
	local, err := h.LocalDB()
	if err != nil {
		return false
	}
	return local.Pkg(pkg) != nil
}


func InfoPacman(h *alpm.Handle, pkgs ...string) SearchResults {
	r := SearchResults{
		Results: []InfoRecord{},
	}

	if h == nil {
		r.Error = "alpm handle is nil"
		return r
	}

	dbs, err := h.SyncDBs()
	if err != nil {
		r.Error = err.Error()
		return r
	}

	local, err := h.LocalDB()
	if err != nil {
		r.Error = err.Error()
		return r
	}
	dbslice := append(dbs.Slice(), local)

	for _, pkg := range pkgs {
		for _, db := range dbslice {
			p := db.Pkg(pkg)
			if p == nil {
				continue
			}

			deps := []string{}
			makedeps := []string{}
			odeps := []string{}
			cdeps := []string{}
			prov := []string{}
			conf := []string{}

			for _, d := range p.Depends().Slice() {
				deps = append(deps, d.String())
			}
			for _, d := range p.MakeDepends().Slice() {
				makedeps = append(makedeps, d.String())
			}
			for _, d := range p.OptionalDepends().Slice() {
				odeps = append(odeps, d.String())
			}
			for _, d := range p.CheckDepends().Slice() {
				cdeps = append(cdeps, d.String())
			}
			for _, pr := range p.Provides().Slice() {
				prov = append(prov, pr.String())
			}
			for _, c := range p.Conflicts().Slice() {
				conf = append(conf, c.String())
			}

			i := InfoRecord{
				Name:         p.Name(),
				Description:  p.Description(),
				Provides:     prov,
				Conflicts:    conf,
				Version:      p.Version(),
				License:      p.Licenses().Slice(),
				Maintainer:   p.Packager(),
				Depends:      deps,
				MakeDepends:  makedeps,
				OptDepends:   odeps,
				CheckDepends: cdeps,
				URL:          p.URL(),
				LastModified: int(p.BuildDate().UTC().Unix()),
				Source:       db.Name(),
				Architecture: p.Architecture(),
				PackageBase:  p.Base(),
				IsIgnored:    p.ShouldIgnore(),
			}

			if lpkg := local.Pkg(p.Name()); lpkg != nil {
				i.LocalVersion = lpkg.Version()
			}
			if db.Name() == "local" {
				i.Description = p.Description() + "\n* Package not found in repositories/AUR *"
			}

			r.Results = append(r.Results, i)
			break
		}
	}

	AddLocalSatisfiers(h, r.Results...)
	return r
}


func AddLocalSatisfiers(h *alpm.Handle, pkgs ...InfoRecord) {
	if h == nil {
		return
	}
	local, err := h.LocalDB()
	if err != nil {
		return
	}

	for i := range len(pkgs) {
		depList := []struct {
			deptype string
			deps    []string
		}{
			{"dep", pkgs[i].Depends},
			{"opt", pkgs[i].OptDepends},
			{"make", pkgs[i].MakeDepends},
			{"check", pkgs[i].CheckDepends},
		}

		satisfiers := []DependencySatisfier{}
		for _, entry := range depList {
			for _, dep := range entry.deps {
				sat := DependencySatisfier{
					DepName:   dep,
					DepType:   entry.deptype,
					Installed: false,
				}
				found, _ := local.PkgCache().FindSatisfier(dep)
				if found != nil {
					sat.Satisfier = found.Name()
					sat.Installed = true
				}
				satisfiers = append(satisfiers, sat)
			}
		}
		pkgs[i].DepsAndSatisfiers = satisfiers
	}
}
