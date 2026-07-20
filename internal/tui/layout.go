package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/druxorey/drxpkg/internal/pkgmgr"
	"github.com/druxorey/drxpkg/internal/util"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Standard panel padding values
const (
	tablePaddingTop    = 1
	tablePaddingBottom = 1
	tablePaddingLeft   = 2
	tablePaddingRight  = 2

	textPaddingTop    = 1
	textPaddingBottom = 1
	textPaddingLeft   = 2
	textPaddingRight  = 2
)

// TUI Tab Indices
const (
	tabInstall = 0
	tabUpdate  = 1
	tabManage  = 2
	tabCount   = 3
)

// Install Package Table Columns
const (
	installColInst = iota
	installColPackage
	installColSource
)

// Update Table Columns
const (
	updateColSelect = iota
	updateColPackage
	updateColCurrent
	updateColArrow
	updateColNew
	updateColSource
)



// Settings Item Indices
const (
	settingIdxSavePath = iota
	settingIdxFileName
	settingIdxInstallCmd
	settingIdxUninstallCmd
	settingIdxUpgradeCmd
	settingIdxMaxResults
	settingIdxAurCb
	settingIdxHooksCb
	settingIdxBtnSave
	settingIdxBtnDefaults
	settingNumItems
)

// applyStandardBorder configures the border, title, and color of a tview.Box based on the theme.
func (ui *UI) applyStandardBorder(box *tview.Box, title string) {
	box.SetBorder(true).
		SetTitle(title).
		SetBorderColor(ui.theme.UnfocusedBorderColor).
		SetTitleColor(ui.theme.PrimaryColor)
}

// setupFocusBorder attaches focus and blur callbacks to shift border colors between focused and unfocused theme colors.
func (ui *UI) setupFocusBorder(widget FocusBorderable) {
	widget.SetFocusFunc(func() {
		widget.SetBorderColor(ui.theme.FocusedBorderColor)
	})
	widget.SetBlurFunc(func() {
		widget.SetBorderColor(ui.theme.UnfocusedBorderColor)
	})
}

// createStandardTable constructs a styled table widget using the default configuration.
func (ui *UI) createStandardTable(title string, fixedRows, fixedCols int) *tview.Table {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(fixedRows, fixedCols)
	
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(ui.theme.PrimaryColor).
		Foreground(ui.theme.SelectedTextColor).
		Attributes(tcell.AttrBold))
	
	ui.applyStandardBorder(table.Box, title)
	table.SetBorderPadding(tablePaddingTop, tablePaddingBottom, tablePaddingLeft, tablePaddingRight)
	ui.setupFocusBorder(table)
	
	return table
}

// createStandardTextView constructs a styled text view widget using the default configuration.
func (ui *UI) createStandardTextView(title string, wrap bool) *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(wrap).
		SetWordWrap(wrap)
	
	ui.applyStandardBorder(tv.Box, title)
	tv.SetBorderPadding(textPaddingTop, textPaddingBottom, textPaddingLeft, textPaddingRight)
	ui.setupFocusBorder(tv)
	
	return tv
}

// createCenteredGrid wraps a primitive in a centered 3x3 grid layout with specified dimensions.
func (ui *UI) createCenteredGrid(primitive tview.Primitive, width, height int) *tview.Grid {
	return tview.NewGrid().
		SetRows(0, height, 0).
		SetColumns(0, width, 0).
		AddItem(primitive, 1, 1, 1, 1, 0, 0, true)
}

// createStyledModal creates a tview.Modal preconfigured with theme-compliant colors.
func (ui *UI) createStyledModal(message string, buttons []string, doneFunc func(int, string)) *tview.Modal {
	modal := tview.NewModal().
		SetText(message).
		AddButtons(buttons).
		SetDoneFunc(doneFunc)
	
	modal.SetBackgroundColor(tcell.ColorBlack)
	modal.SetTextColor(tcell.ColorDefault)
	modal.SetButtonBackgroundColor(ui.theme.SelectedTextColor)
	modal.SetButtonTextColor(ui.theme.PrimaryColor)
	
	return modal
}

// runDiff runs `diff -u` on two string buffers using temporary files, correctly cleaning up resources.
func runDiff(localContent, remoteContent string) (string, error) {
	tmpLocal, err := os.CreateTemp("", "drxpkg-local-")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tmpLocal.Close()
		_ = os.Remove(tmpLocal.Name())
	}()

	if _, err := tmpLocal.WriteString(localContent); err != nil {
		return "", err
	}

	tmpRemote, err := os.CreateTemp("", "drxpkg-remote-")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tmpRemote.Close()
		_ = os.Remove(tmpRemote.Name())
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
		boxWidth := max(width-2, 40)
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

	if record.Source == "AUR" {
		fields = append(fields, struct {
			label string
			value string
		}{"Votes", strconv.Itoa(record.NumVotes)})
		fields = append(fields, struct {
			label string
			value string
		}{"Popularity", fmt.Sprintf("%.2f", record.Popularity)})
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
		w := max(width-2, 40)
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
