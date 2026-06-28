// Package tui manages the Terminal User Interface, layouts, user input, and screen rendering.
package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/druxorey/drxpkg/internal/pkglist"
	"github.com/druxorey/drxpkg/internal/pkgmgr"
	"github.com/druxorey/drxpkg/internal/util"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *UI) handleSearchChange(text string) {
	term := strings.TrimSpace(text)
	ui.lastSearchTerm = term

	// Instantly update local search results
	ui.performLocalSearch(term)

	// Schedule debounced remote AUR search
	ui.scheduleAurSearch(term)
}

func (ui *UI) performLocalSearch(term string) {
	if term == "" {
		ui.shownPackages = nil
		ui.renderPackageTable()
		ui.setStatus("")
		return
	}

	termLower := strings.ToLower(term)
	var reposPkgs []pkgmgr.Package
	var localPkgs []pkgmgr.Package

	ui.alpmMutex.Lock()
	for _, cp := range ui.pkgsCache {
		if strings.Contains(cp.NameLower, termLower) ||
			strings.Contains(cp.DescriptionLower, termLower) {
			if cp.Source == "local" {
				localPkgs = append(localPkgs, cp.Package)
			} else {
				reposPkgs = append(reposPkgs, cp.Package)
			}
		}
	}
	ui.alpmMutex.Unlock()

	allPkgs := append(reposPkgs, localPkgs...)

	uniqueMap := make(map[string]pkgmgr.Package)
	for _, p := range allPkgs {
		existing, exists := uniqueMap[p.Name]
		if !exists || (!existing.IsInstalled && p.IsInstalled) {
			uniqueMap[p.Name] = p
		}
	}

	var resultList []pkgmgr.Package
	for _, p := range uniqueMap {
		resultList = append(resultList, p)
	}

	sort.Slice(resultList, func(i, j int) bool {
		a, b := resultList[i], resultList[j]
		if a.IsInstalled != b.IsInstalled {
			return a.IsInstalled
		}
		aScore := pkgmgr.GetUnifiedScore(a, term)
		bScore := pkgmgr.GetUnifiedScore(b, term)
		if aScore != bScore {
			return aScore > bScore
		}
		return a.Name < b.Name
	})

	if len(resultList) > ui.conf.MaxResults {
		resultList = resultList[:ui.conf.MaxResults]
	}

	ui.shownPackages = resultList
	ui.renderPackageTable()

	if len(resultList) == 0 {
		ui.setStatus("No packages found.")
	} else {
		ui.setStatus(fmt.Sprintf("Found %d packages.", len(resultList)))
	}
}

func (ui *UI) scheduleAurSearch(term string) {
	ui.searchMutex.Lock()
	defer ui.searchMutex.Unlock()

	if ui.searchCancel != nil {
		ui.searchCancel()
		ui.searchCancel = nil
	}

	if ui.searchTimer != nil {
		ui.searchTimer.Stop()
		ui.searchTimer = nil
	}

	if len(term) < 3 || ui.conf.DisableAur {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	ui.searchCancel = cancel

	ui.searchTimer = time.AfterFunc(200*time.Millisecond, func() {
		ui.runAurSearch(ctx, term)
	})
}

func (ui *UI) runAurSearch(ctx context.Context, term string) {
	ui.app.QueueUpdateDraw(func() {
		if ctx.Err() == nil {
			ui.setStatus("Searching AUR...")
		}
	})

	aurPkgs, err := pkgmgr.SearchAur(ctx, "", term, 5000, 2000)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		ui.app.QueueUpdateDraw(func() {
			ui.setStatus("[red]AUR Search error: " + err.Error())
		})
		return
	}

	installedMap := make(map[string]bool)
	ui.alpmMutex.Lock()
	for _, cp := range ui.pkgsCache {
		if cp.IsInstalled {
			installedMap[cp.Name] = true
		}
	}
	ui.alpmMutex.Unlock()

	for idx := range aurPkgs {
		aurPkgs[idx].IsInstalled = installedMap[aurPkgs[idx].Name]
	}

	if ctx.Err() != nil {
		return
	}

	ui.app.QueueUpdateDraw(func() {
		if ctx.Err() != nil {
			return
		}

		termLower := strings.ToLower(term)
		var reposPkgs []pkgmgr.Package
		var localPkgs []pkgmgr.Package

		ui.alpmMutex.Lock()
		for _, cp := range ui.pkgsCache {
			if strings.Contains(cp.NameLower, termLower) ||
				strings.Contains(cp.DescriptionLower, termLower) {
				if cp.Source == "local" {
					localPkgs = append(localPkgs, cp.Package)
				} else {
					reposPkgs = append(reposPkgs, cp.Package)
				}
			}
		}
		ui.alpmMutex.Unlock()

		allPkgs := append(reposPkgs, localPkgs...)
		allPkgs = append(allPkgs, aurPkgs...)

		uniqueMap := make(map[string]pkgmgr.Package)
		for _, p := range allPkgs {
			existing, exists := uniqueMap[p.Name]
			if !exists || (!existing.IsInstalled && p.IsInstalled) {
				uniqueMap[p.Name] = p
			}
		}

		var resultList []pkgmgr.Package
		for _, p := range uniqueMap {
			resultList = append(resultList, p)
		}

		sort.Slice(resultList, func(i, j int) bool {
			a, b := resultList[i], resultList[j]
			if a.IsInstalled != b.IsInstalled {
				return a.IsInstalled
			}
			aScore := pkgmgr.GetUnifiedScore(a, term)
			bScore := pkgmgr.GetUnifiedScore(b, term)
			if aScore != bScore {
				return aScore > bScore
			}
			return a.Name < b.Name
		})

		if len(resultList) > ui.conf.MaxResults {
			resultList = resultList[:ui.conf.MaxResults]
		}

		ui.shownPackages = resultList
		ui.renderPackageTable()

		if len(resultList) == 0 {
			ui.setStatus("No packages found.")
		} else {
			ui.setStatus(fmt.Sprintf("Found %d packages (incl. AUR).", len(resultList)))
		}
	})
}

func (ui *UI) forceSearch(term string) {
	ui.searchMutex.Lock()
	defer ui.searchMutex.Unlock()

	if ui.searchCancel != nil {
		ui.searchCancel()
		ui.searchCancel = nil
	}

	if ui.searchTimer != nil {
		ui.searchTimer.Stop()
		ui.searchTimer = nil
	}

	ui.lastSearchTerm = term
	ui.performLocalSearch(term)

	if ui.conf.DisableAur || term == "" {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	ui.searchCancel = cancel

	go ui.runAurSearch(ctx, term)
}

func (ui *UI) renderPackageTable() {
	ui.isRendering = true
	defer func() { ui.isRendering = false }()

	selectedRow, _ := ui.pkgTable.GetSelection()

	ui.pkgTable.Clear()

	// Header row
	ui.pkgTable.SetCell(0, 0, tview.NewTableCell("").SetSelectable(false).SetMaxWidth(8))
	ui.pkgTable.SetCell(0, 1, tview.NewTableCell("Package").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetExpansion(1))
	ui.pkgTable.SetCell(0, 2, tview.NewTableCell("Source").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetMaxWidth(12))
	ui.pkgTable.SetCell(0, 3, tview.NewTableCell("Installed").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetMaxWidth(10))
	ui.pkgTable.SetCell(0, 4, tview.NewTableCell("Reputation").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetMaxWidth(12))

	for idx, p := range ui.shownPackages {
		row := idx + 1
		isHighlighted := false
		if ui.inVisualMode && ui.activeTab == 0 {
			minRow := min(ui.visualStartRow, ui.visualEndRow)
			maxRow := max(ui.visualStartRow, ui.visualEndRow)
			if row >= minRow && row <= maxRow {
				isHighlighted = true
			}
		}

		selStr := "   "
		if ui.selectedInstall[p.Name] {
			selStr = "  x"
		}
		selCell := tview.NewTableCell(selStr).SetMaxWidth(8).SetAlign(tview.AlignLeft)
		if ui.selectedInstall[p.Name] {
			selCell.SetTextColor(tcell.ColorGreen)
		}

		pkgCell := tview.NewTableCell(p.Name).SetTextColor(tcell.ColorDefault).SetExpansion(1)

		sourceColor := getSourceColor(p.Source)
		sourceCell := tview.NewTableCell(p.Source).SetTextColor(sourceColor).SetMaxWidth(12)

		installedStr := "No"
		installedCell := tview.NewTableCell(installedStr).SetMaxWidth(10)
		if p.IsInstalled {
			installedStr = "Yes"
			installedCell.SetText(installedStr).
				SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen).Attributes(tcell.AttrBold))
		} else {
			installedCell.SetTextColor(tcell.ColorGray)
		}

		reputationStr := ""
		if p.Source == "AUR" {
			reputationStr = strconv.Itoa(p.Votes)
		}
		reputationCell := tview.NewTableCell(reputationStr).SetTextColor(tcell.ColorDefault).SetMaxWidth(12)

		if isHighlighted {
			bgColor := tcell.NewHexColor(0x1a3a5c)
			selCell.SetBackgroundColor(bgColor)
			pkgCell.SetBackgroundColor(bgColor)
			sourceCell.SetBackgroundColor(bgColor)
			installedCell.SetBackgroundColor(bgColor)
			reputationCell.SetBackgroundColor(bgColor)
		}

		ui.pkgTable.SetCell(row, 0, selCell)
		ui.pkgTable.SetCell(row, 1, pkgCell)
		ui.pkgTable.SetCell(row, 2, sourceCell)
		ui.pkgTable.SetCell(row, 3, installedCell)
		ui.pkgTable.SetCell(row, 4, reputationCell)
	}

	if selectedRow > 0 && selectedRow <= len(ui.shownPackages) {
		ui.pkgTable.Select(selectedRow, 0)
	} else if len(ui.shownPackages) > 0 {
		ui.pkgTable.ScrollToBeginning()
		ui.pkgTable.Select(1, 0)
	}
}

func (ui *UI) loadPackageDetails(pkg pkgmgr.Package) {
	ui.detailsView.Clear()
	ui.detailsView.SetTitle(fmt.Sprintf(" Details: %s ", pkg.Name))

	go func() {
		var info pkgmgr.SearchResults
		if pkg.Source == "AUR" {
			info = pkgmgr.InfoAur("", 5000, pkg.Name)
		} else {
			ui.alpmMutex.Lock()
			info = pkgmgr.InfoPacman(ui.alpmHandle, pkg.Name)
			ui.alpmMutex.Unlock()
		}

		ui.app.QueueUpdateDraw(func() {
			if len(info.Results) == 0 {
				ui.detailsView.SetText("[red]Error fetching details")
				return
			}

			record := info.Results[0]
			fields := []struct {
				label string
				value string
			}{
				{"Description", record.Description},
				{"Version", record.Version},
				{"Local Ver", record.LocalVersion},
				{"Source", record.Source},
				{"Architecture", record.Architecture},
				{"URL", record.URL},
				{"Licenses", strings.Join(record.License, ", ")},
				{"Maintainer", record.Maintainer},
			}

			var sb strings.Builder
			for _, f := range fields {
				if f.value == "" {
					continue
				}
				if f.label == "Description" {
					fmt.Fprintf(&sb, "[blue]%s:[-]\n%s\n\n", f.label, f.value)
				} else {
					fmt.Fprintf(&sb, "[blue]%s:[-] %s\n", f.label, f.value)
				}
			}

			// Dependencies
			if len(record.Depends) > 0 {
				fmt.Fprintf(&sb, "\n[blue]Dependencies:[-]\n%s\n", strings.Join(record.Depends, ", "))
			}

			ui.detailsView.SetText(sb.String())
			ui.detailsView.ScrollToBeginning()
		})
	}()
}

func (ui *UI) installOrUninstallPackage(pkg pkgmgr.Package) {
	cmdStr := ui.conf.InstallCommand
	isInstall := true
	if pkg.IsInstalled {
		cmdStr = ui.conf.UninstallCommand
		isInstall = false
	}

	ui.app.Suspend(func() {
		// Clean terminal screen using standard ANSI code
		fmt.Print("\033[H\033[2J")

		var fullCommand string
		if strings.Contains(cmdStr, "{pkg}") {
			fullCommand = strings.ReplaceAll(cmdStr, "{pkg}", pkg.Name)
		} else {
			fullCommand = cmdStr + " " + pkg.Name
		}

		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}

		fmt.Printf("Running: %s\n\n", fullCommand)
		cmd := exec.Command(shell, "-c", fullCommand)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err == nil {
			if isInstall {
				_ = pkglist.AddPackage(ui.conf.PackagesPath, ui.conf.PackagesFile, pkg.Name)
				util.PrintSuccess("Package '%s' installed and added to %s.\n", pkg.Name, ui.conf.PackagesFile)
			} else {
				_ = pkglist.RemovePackage(ui.conf.PackagesPath, ui.conf.PackagesFile, pkg.Name)
				util.PrintSuccess("Package '%s' uninstalled and removed from %s.\n", pkg.Name, ui.conf.PackagesFile)
			}
			fmt.Println("\nPress ENTER to return to drxpkg...")
			_, _ = os.Stdin.Read(make([]byte, 1))
		} else {
			util.PrintError("Command failed: %v\nPress ENTER to return to drxpkg...", err)
			_, _ = os.Stdin.Read(make([]byte, 1))
		}
	})

	_ = ui.reinitPacmanDbs()
	if ui.lastSearchTerm != "" {
		ui.forceSearch(ui.lastSearchTerm)
	}
}

func (ui *UI) showConfirmation(message string, onConfirm func()) {
	prevFocus := ui.app.GetFocus()
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			ui.pages.RemovePage("confirmation")
			ui.app.SetFocus(prevFocus)
			if buttonLabel == "Yes" {
				onConfirm()
			}
		})
	modal.SetBackgroundColor(tcell.ColorBlack)
	modal.SetTextColor(tcell.ColorDefault)
	modal.SetButtonBackgroundColor(tcell.ColorBlue)
	modal.SetButtonTextColor(tcell.ColorWhite)
	ui.pages.AddPage("confirmation", modal, true, true)
}

func (ui *UI) promptInstall(pkgName string) {
	pkgs := strings.Fields(pkgName)
	var message string
	if len(pkgs) > 1 {
		message = fmt.Sprintf("Are you sure you want to install these %d packages?", len(pkgs))
	} else {
		message = fmt.Sprintf("Are you sure you want to install package '%s'?", pkgName)
	}
	ui.showConfirmation(message, func() {
		ui.performInstallOrUninstall(pkgName, true)
	})
}

func (ui *UI) promptUninstall(pkgName string) {
	pkgs := strings.Fields(pkgName)
	var message string
	if len(pkgs) > 1 {
		message = fmt.Sprintf("Are you sure you want to uninstall these %d packages?", len(pkgs))
	} else {
		message = fmt.Sprintf("Are you sure you want to uninstall package '%s'?", pkgName)
	}
	ui.showConfirmation(message, func() {
		ui.performInstallOrUninstall(pkgName, false)
	})
}

func (ui *UI) performInstallOrUninstall(pkgName string, isInstall bool) {
	cmdStr := ui.conf.InstallCommand
	if !isInstall {
		cmdStr = ui.conf.UninstallCommand
	}

	ui.app.Suspend(func() {
		fmt.Print("\033[H\033[2J")

		var fullCommand string
		if strings.Contains(cmdStr, "{pkg}") {
			fullCommand = strings.ReplaceAll(cmdStr, "{pkg}", pkgName)
		} else {
			fullCommand = cmdStr + " " + pkgName
		}

		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}

		fmt.Printf("Running: %s\n\n", fullCommand)
		cmd := exec.Command(shell, "-c", fullCommand)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err == nil {
			pkgs := strings.Fields(pkgName)
			for _, p := range pkgs {
				if isInstall {
					_ = pkglist.AddPackage(ui.conf.PackagesPath, ui.conf.PackagesFile, p)
					util.PrintSuccess("Package '%s' installed and added to %s.\n", p, ui.conf.PackagesFile)
				} else {
					_ = pkglist.RemovePackage(ui.conf.PackagesPath, ui.conf.PackagesFile, p)
					util.PrintSuccess("Package '%s' uninstalled and removed from %s.\n", p, ui.conf.PackagesFile)
				}
			}
			fmt.Println("\nPress ENTER to return to drxpkg...")
			_, _ = os.Stdin.Read(make([]byte, 1))
		} else {
			util.PrintError("Command failed: %v\nPress ENTER to return to drxpkg...", err)
			_, _ = os.Stdin.Read(make([]byte, 1))
		}
	})

	_ = ui.reinitPacmanDbs()
	if ui.activeTab == 0 {
		ui.selectedInstall = make(map[string]bool)
		if ui.lastSearchTerm != "" {
			ui.forceSearch(ui.lastSearchTerm)
		}
	} else if ui.activeTab == 1 {
		ui.updatePackages = nil
		ui.checkForUpdates()
	}
}
