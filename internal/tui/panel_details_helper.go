package tui

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/druxorey/drxpkg/internal/pkgmgr"
	"github.com/druxorey/drxpkg/internal/util"
)

func getPkgbuildContentWithTimeout(url string, timeout time.Duration) (string, error) {
	client := http.Client{Timeout: timeout}
	resp, err := client.Get(url)
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

func runDiff(localContent, remoteContent string) (string, error) {
	tmpLocal, err := os.CreateTemp("", "drxpkg-local-")
	if err != nil {
		return "", err
	}
	defer func() {
		if err := tmpLocal.Close(); err != nil {
			util.PrintError("Failed to close response body: %v", err)
		}
		if err := os.Remove(tmpLocal.Name()); err != nil {
			util.PrintError("Failed to remove temporary file %s: %v", tmpLocal.Name(), err)
		}
	}()

	if _, err := tmpLocal.WriteString(localContent); err != nil {
		return "", err
	}

	tmpRemote, err := os.CreateTemp("", "drxpkg-remote-")
	if err != nil {
		return "", err
	}
	defer func() {
		if err := tmpLocal.Close(); err != nil {
			util.PrintError("Failed to close response body: %v", err)
		}
		if err := os.Remove(tmpLocal.Name()); err != nil {
			util.PrintError("Failed to remove temporary file %s: %v", tmpLocal.Name(), err)
		}
	}()

	if _, err := tmpRemote.WriteString(remoteContent); err != nil {
		return "", err
	}

	cmd := exec.Command("diff", "-u", tmpLocal.Name(), tmpRemote.Name())
	output, _ := cmd.CombinedOutput()
	return string(output), nil
}

func formatDiff(diffText string) string {
	lines := strings.Split(diffText, "\n")
	for i, line := range lines {
		escaped := strings.ReplaceAll(line, "[", "[[")
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			lines[i] = "[yellow]" + escaped + "[-]"
		} else if strings.HasPrefix(line, "@@") {
			lines[i] = "[cyan]" + escaped + "[-]"
		} else if strings.HasPrefix(line, "-") {
			lines[i] = "[red]" + escaped + "[-]"
		} else if strings.HasPrefix(line, "+") {
			lines[i] = "[green]" + escaped + "[-]"
		} else {
			lines[i] = escaped
		}
	}
	return strings.Join(lines, "\n")
}

// FetchAndBuildDetails fetches package details, retrieves PKGBUILD, performs diff if needed, and builds a formatted string for display.
func (ui *UI) FetchAndBuildDetails(name, source string) string {
	var info pkgmgr.SearchResults
	if source == "AUR" {
		info = pkgmgr.InfoAur("", 5000, name)
	} else {
		ui.alpmMutex.Lock()
		info = pkgmgr.InfoPacman(ui.alpmHandle, name)
		ui.alpmMutex.Unlock()
	}

	var sb strings.Builder

	if len(info.Results) == 0 {
		fmt.Fprintf(&sb, "[blue]Package:[-] %s\n", name)
		fmt.Fprintf(&sb, "[blue]Source:[-] %s\n\n", source)
		fmt.Fprintf(&sb, "[red]Error: Failed to fetch details[-]\n")
		return sb.String()
	}

	record := info.Results[0]

	// Warning if flagged out of date
	if record.OutOfDate > 0 {
		fmt.Fprintf(&sb, "[red]------------------------------------------------------------------[-]\n")
		fmt.Fprintf(&sb, "[red]WARNING:[-] Flagged OUT OF DATE in the AUR.\n")
		fmt.Fprintf(&sb, "It is recommended to avoid installing/updating this package or uninstall it.\n")
		fmt.Fprintf(&sb, "[red]------------------------------------------------------------------[-]\n\n")
	}

	// Prepare fields
	localVerVal := record.LocalVersion
	if record.LocalVersion == "" {
		localVerVal = "None"
	}

	maintainerVal := record.Maintainer
	if record.Maintainer == "" {
		maintainerVal = "[red::b]Orphan[-:-:-]"
	}

	if record.Description != "" {
		fmt.Fprintf(&sb, "[blue]Description:[-]\n%s\n\n", record.Description)
	}

	fields := []struct {
		label string
		value string
	}{
		{"Local Ver", localVerVal},
		{"Remote Ver", record.Version},
		{"Source", record.Source},
		{"Architecture", record.Architecture},
		{"URL", record.URL},
		{"Licenses", strings.Join(record.License, ", ")},
		{"Maintainer", maintainerVal},
	}

	for _, f := range fields {
		if f.value == "" {
			continue
		}
		fmt.Fprintf(&sb, "[blue]%s:[-] %s\n", f.label, f.value)
	}

	// Dependencies
	if len(record.Depends) > 0 {
		fmt.Fprintf(&sb, "\n[blue]Dependencies:[-]\n%s\n", strings.Join(record.Depends, ", "))
	}

	// Get PKGBUILD base and path
	pkgBase := record.PackageBase
	if pkgBase == "" {
		pkgBase = name
	}

	var localPKGBUILD string
	var remotePKGBUILD string

	if source == "AUR" {
		home, err := os.UserHomeDir()
		if err == nil {
			localPath := filepath.Join(home, ".cache/yay", pkgBase, "PKGBUILD")
			data, err := os.ReadFile(localPath)
			if err == nil {
				localPKGBUILD = string(data)
			}
		}
	}

	// Fetch remote PKGBUILD
	remoteURL := pkgmgr.GetPkgbuildURL(source, pkgBase)
	if remoteURL != "" {
		remotePKGBUILD, _ = getPkgbuildContentWithTimeout(remoteURL, 5*time.Second)
	}

	// Render PKGBUILD / PKGBUILD Diff
	if source == "AUR" {
		isDifferentVersion := record.LocalVersion != "" && record.LocalVersion != record.Version
		
		// Determine whether to show diff or just remote PKGBUILD
		var showDiff bool
		var diffOut string
		var err error
		if isDifferentVersion && localPKGBUILD != "" && remotePKGBUILD != "" {
			diffOut, err = runDiff(localPKGBUILD, remotePKGBUILD)
			if err == nil && diffOut != "" {
				showDiff = true
			}
		}

		if showDiff {
			fmt.Fprintf(&sb, "\n[yellow]----------------- PKGBUILD Diff -----------------[-]\n")
			fmt.Fprintf(&sb, "%s", formatDiff(diffOut))
		} else {
			fmt.Fprintf(&sb, "\n[yellow]----------------- PKGBUILD -----------------[-]\n")
			if remotePKGBUILD == "" {
				fmt.Fprintf(&sb, "[red]Failed to fetch remote PKGBUILD from AUR cgit.[-]\n")
			} else {
				fmt.Fprintf(&sb, "%s", formatDiff(remotePKGBUILD))
			}
		}
	} else {
		// Repository packages
		if remotePKGBUILD != "" {
			fmt.Fprintf(&sb, "\n[yellow]----------------- PKGBUILD -----------------[-]\n")
			fmt.Fprintf(&sb, "%s", formatDiff(remotePKGBUILD))
		} else {
			fmt.Fprintf(&sb, "\n[gray]No PKGBUILD available for repository packages.[-]\n")
		}
	}

	return sb.String()
}
