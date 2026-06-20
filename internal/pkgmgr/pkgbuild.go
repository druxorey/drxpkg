// Package pkgmgr provides package management backend operations for AUR and local Pacman (ALPM) databases.
package pkgmgr

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"github.com/druxorey/drxpkg/internal/util"
)

const (
	URLAurPkgbuild  = "https://aur.archlinux.org/cgit/aur.git/plain/PKGBUILD?h=%s"
	URLRepoPkgbuild = "https://gitlab.archlinux.org/archlinux/packaging/packages/%s/-/raw/main/PKGBUILD"
)

type RegexReplace struct {
	repl  string
	match *regexp.Regexp
}

var gitlabRepl = []RegexReplace{
	{repl: `$1-$2`, match: regexp.MustCompile(`([a-zA-Z0-9]+)\+([a-zA-Z]+)`)},
	{repl: `plus`, match: regexp.MustCompile(`\+`)},
	{repl: `-`, match: regexp.MustCompile(`[^a-zA-Z0-9_\-\.]`)},
	{repl: `-`, match: regexp.MustCompile(`[_\-]{2,}`)},
	{repl: `unix-tree`, match: regexp.MustCompile(`^tree$`)},
}

func GetPkgbuildURL(source, base string) string {
	if source != "AUR" {
		return fmt.Sprintf(URLRepoPkgbuild, encodePackageGitlabURL(base))
	}
	return fmt.Sprintf(URLAurPkgbuild, base)
}

func GetPkgbuildContent(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			util.PrintError("Failed to close response body: %v", err)
		}
	}()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func encodePackageGitlabURL(pkgname string) string {
	for _, regex := range gitlabRepl {
		pkgname = regex.match.ReplaceAllString(pkgname, regex.repl)
	}
	return pkgname
}

