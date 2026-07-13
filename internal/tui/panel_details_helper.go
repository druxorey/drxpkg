package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/druxorey/drxpkg/internal/pkgmgr"
	"github.com/druxorey/drxpkg/internal/util"
)


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

// FetchAndBuildDetails fetches package details, retrieves PKGBUILD, performs diff if needed, and builds a formatted string for display.
func (ui *UI) FetchAndBuildDetails(name, source string) string {
	var info pkgmgr.SearchResults
	if source == "AUR" {
		info = pkgmgr.InfoAur("", 5000, name)
		ui.alpmMutex.Lock()
		pkgmgr.AddLocalSatisfiers(ui.alpmHandle, info.Results...)
		ui.alpmMutex.Unlock()
	} else {
		ui.alpmMutex.Lock()
		info = pkgmgr.InfoPacman(ui.alpmHandle, name)
		ui.alpmMutex.Unlock()
	}

	var sb strings.Builder

	if len(info.Results) == 0 {
		fmt.Fprintf(&sb, "[-:-:-][blue]Package:[-] %s\n", name)
		fmt.Fprintf(&sb, "[blue]Source:[-] %s\n\n", source)
		fmt.Fprintf(&sb, "[red]Error: Failed to fetch details[-]\n")
		return sb.String()
	}

	record := info.Results[0]

	var width int
	if ui.activeTab == 0 && ui.detailsView != nil {
		_, _, width, _ = ui.detailsView.GetInnerRect()
	} else if ui.activeTab == 1 && ui.updateDetails != nil {
		_, _, width, _ = ui.updateDetails.GetInnerRect()
	}
	if width <= 0 {
		width = 80 // fallback default
	}

	if record.OutOfDate > 0 {
		boxWidth := max(width - 2, 40)
		fillLine := func(text string) string {
			if len(text) > boxWidth {
				text = text[:boxWidth]
			}
			spaces := boxWidth - len(text)
			return text + strings.Repeat(" ", spaces)
		}

		fmt.Fprintf(&sb, "[-:-:-][white:red:b]%s[-:-:-]\n", fillLine(""))
		fmt.Fprintf(&sb, "[white:red:b]%s[-:-:-]\n", fillLine("  WARNING: Flagged OUT OF DATE in the AUR."))
		fmt.Fprintf(&sb, "[white:red:b]%s[-:-:-]\n", fillLine("  It is recommended to avoid installing/updating this package"))
		fmt.Fprintf(&sb, "[white:red:b]%s[-:-:-]\n", fillLine("  or uninstall it."))
		fmt.Fprintf(&sb, "[white:red:b]%s[-:-:-]\n\n", fillLine(""))
	}

	localVerVal := record.LocalVersion
	if record.LocalVersion == "" {
		localVerVal = "None"
	}

	maintainerVal := record.Maintainer
	if record.Maintainer == "" {
		maintainerVal = "[red::b]Orphan[-:-:-]"
	}

	if record.Description != "" {
		fmt.Fprintf(&sb, "[-:-:-][blue]%-15s[-] %s\n\n", "Description", record.Description)
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
		fmt.Fprintf(&sb, "[-:-:-][blue]%-15s[-] %s\n", f.label, f.value)
	}

	if len(record.DepencenciesAndSatisfiers) > 0 {
		fmt.Fprintf(&sb, "\n")
		for idx, dep := range record.DepencenciesAndSatisfiers {
			var check string
			if dep.Installed {
				check = "[green][✓][-]"
			} else {
				check = "[ ]"
			}

			depStr := dep.DepName
			if dep.DepType != "dep" {
				depStr = fmt.Sprintf("%s (%s)", dep.DepName, dep.DepType)
			}

			if idx == 0 {
				fmt.Fprintf(&sb, "[-:-:-][blue]%-15s[-] %s %s\n", "Dependencies", check, depStr)
			} else {
				fmt.Fprintf(&sb, "[-:-:-]%-15s %s %s\n", "", check, depStr)
			}
		}
	} else if len(record.Depends) > 0 {
		fmt.Fprintf(&sb, "\n")
		for idx, dep := range record.Depends {
			if idx == 0 {
				fmt.Fprintf(&sb, "[-:-:-][blue]%-15s[-] [ ] %s\n", "Dependencies", dep)
			} else {
				fmt.Fprintf(&sb, "[-:-:-]%-15s [ ] %s\n", "", dep)
			}
		}
	}

	// Helper to print centered title and a solid yellow horizontal line
	printDivider := func(title string) {
		w := max(width - 2, 40)
		padding := max(((w - len(title)) / 2), 0)
		centerTitle := strings.Repeat(" ", padding) + title
		
		fmt.Fprintf(&sb, "\n[-:-:-][yellow]%s[-:-:-]\n", centerTitle)
		fmt.Fprintf(&sb, "[yellow]%s[-:-:-]\n", strings.Repeat("─", w))
	}

	resolvedSource := source
	if source == "local" {
		resolvedSource = ui.getPackageSource(name)
	}

	pkgBase := record.PackageBase
	if pkgBase == "" {
		pkgBase = name
	}

	var localPKGBUILD string
	var remotePKGBUILD string

	if resolvedSource == "AUR" {
		home, err := os.UserHomeDir()
		if err == nil {
			localPath := filepath.Join(home, ".cache/yay", pkgBase, "PKGBUILD")
			data, err := os.ReadFile(localPath)
			if err == nil {
				localPKGBUILD = string(data)
			}
		}
	}

	remoteURL := pkgmgr.GetPkgbuildURL(resolvedSource, pkgBase)
	if remoteURL != "" {
		remotePKGBUILD, _ = pkgmgr.GetPkgbuildContent(remoteURL, 5*time.Second)
	}

	if resolvedSource == "AUR" {
		isDifferentVersion := record.LocalVersion != "" && record.LocalVersion != record.Version
		
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
			printDivider("PKGBUILD Diff")
			fmt.Fprintf(&sb, "%s", util.FormatDiff(diffOut))
		} else {
			printDivider("PKGBUILD")
			if remotePKGBUILD == "" {
				if source == "local" {
					fmt.Fprintf(&sb, "[-:-:-][red]No PKGBUILD available (failed to fetch from AUR cgit).[-:-:-]\n")
				} else {
					fmt.Fprintf(&sb, "[-:-:-][red]Failed to fetch remote PKGBUILD from AUR cgit.[-:-:-]\n")
				}
			} else {
				fmt.Fprintf(&sb, "%s", util.FormatPKGBUILD(remotePKGBUILD))
			}
		}
	} else {
		if remotePKGBUILD != "" {
			printDivider("PKGBUILD")
			fmt.Fprintf(&sb, "%s", util.FormatPKGBUILD(remotePKGBUILD))
		} else {
			fmt.Fprintf(&sb, "\n[-:-:-][gray]No PKGBUILD available for repository packages.[-:-:-]\n")
		}
	}

	return sb.String()
}
