// Package tui manages the Terminal User Interface, layouts, user input, and screen rendering.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/druxorey/drxpkg/internal/pkglist"
	"github.com/druxorey/drxpkg/internal/pkgmgr"
	"github.com/druxorey/drxpkg/internal/util"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// handleSearchChange updates the filter criteria and triggers searches when the search input changes
func (ui *UI) handleSearchChange(text string) {
	if ui.ignoreSearchChange {
		return
	}
	term := strings.TrimSpace(text)
	ui.lastSearchTerm = term
	ui.pkgTable.Select(1, 0)
	ui.pkgTable.ScrollToBeginning()
	ui.performLocalSearch(term)
}

// performLocalSearch filters the local package cache based on the provided term and updates the UI
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

	var aurPkgs []pkgmgr.Package
	if !ui.conf.DisableAur {
		ui.aurPkgsMutex.RLock()
		for _, name := range ui.aurPkgsCache {
			nameLower := strings.ToLower(name)
			if strings.Contains(nameLower, termLower) {
				aurPkgs = append(aurPkgs, pkgmgr.Package{
					Name:   name,
					Source: "AUR",
				})
			}
		}
		ui.aurPkgsMutex.RUnlock()
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

	suffix := ""
	ui.aurPkgsMutex.RLock()
	if ui.aurCacheLoading {
		suffix = " (AUR cache loading...)"
	} else if !ui.aurCacheLoaded && !ui.conf.DisableAur {
		suffix = " (AUR cache load failed)"
	}
	ui.aurPkgsMutex.RUnlock()

	if len(resultList) == 0 {
		ui.setStatus("No packages found." + suffix)
	} else {
		if !ui.conf.DisableAur {
			ui.setStatus(fmt.Sprintf("Found %d packages (incl. AUR)%s.", len(resultList), suffix))
		} else {
			ui.setStatus(fmt.Sprintf("Found %d packages.", len(resultList)))
		}
	}
}

// forceSearch immediately triggers a search operation, bypassing any active debounces
func (ui *UI) forceSearch(term string) {
	ui.lastSearchTerm = term

	// Reset selection to the first item on forced search
	ui.pkgTable.Select(1, 0)
	ui.pkgTable.ScrollToBeginning()

	ui.performLocalSearch(term)
}

// renderPackageTable updates the visual table component with the current package list
func (ui *UI) renderPackageTable() {
	ui.isRendering = true
	defer func() { ui.isRendering = false }()

	selectedRow, _ := ui.pkgTable.GetSelection()

	ui.pkgTable.Clear()

	ui.pkgTable.SetCell(0, installColInst, tview.NewTableCell("Inst").SetTextColor(ui.theme.PrimaryColor).SetSelectable(false).SetMaxWidth(4))
	ui.pkgTable.SetCell(0, installColPackage, tview.NewTableCell("Package").SetTextColor(ui.theme.PrimaryColor).SetSelectable(false).SetExpansion(1))
	ui.pkgTable.SetCell(0, installColSource, tview.NewTableCell("Source           ").SetTextColor(ui.theme.PrimaryColor).SetSelectable(false).SetMaxWidth(17))

	for idx, p := range ui.shownPackages {
		row := idx + 1
		isSelected := (row == selectedRow) || (selectedRow <= 0 && row == 1)

		pkgNameHighlighted := highlightFuzzy(p.Name, ui.lastSearchTerm, isSelected)
		pkgCell := tview.NewTableCell(pkgNameHighlighted).SetTextColor(tcell.ColorDefault).SetExpansion(1)

		sourceColor := getSourceColor(p.Source)
		sourceCell := tview.NewTableCell(p.Source).SetTextColor(sourceColor).SetMaxWidth(12)

		installedStr := ""
		installedCell := tview.NewTableCell(installedStr).SetMaxWidth(10)
		if p.IsInstalled {
			installedStr = " ✓ "
			installedCell.SetText(installedStr).SetTextColor(tcell.ColorGreen)
		}

		ui.pkgTable.SetCell(row, installColInst, installedCell)
		ui.pkgTable.SetCell(row, installColPackage, pkgCell)
		ui.pkgTable.SetCell(row, installColSource, sourceCell)
	}

	var activeRow int
	if selectedRow > 0 && selectedRow <= len(ui.shownPackages) {
		activeRow = selectedRow
	} else if len(ui.shownPackages) > 0 {
		activeRow = 1
	}

	if activeRow > 0 {
		ui.pkgTable.Select(activeRow, 0)
		pkg := ui.shownPackages[activeRow-1]
		ui.selectedPkg = &pkg
		ui.loadPackageDetails(pkg)
	} else {
		ui.selectedPkg = nil
		if ui.detailsView != nil {
			ui.detailsView.Clear()
		}
	}

	ui.updateInstallRightFlex()
}

// loadPackageDetails fetches and displays extended information for a specific package
func (ui *UI) loadPackageDetails(pkg pkgmgr.Package) {
	if ui.detailsView == nil {
		return
	}
	ui.detailsView.Clear()
	ui.applyStandardBorder(ui.detailsView.Box, fmt.Sprintf(" Details: %s ", pkg.Name))

	ui.cacheMutex.RLock()
	cachedText, exists := ui.installDetailsCache[pkg.Name]
	ui.cacheMutex.RUnlock()

	if exists {
		ui.detailsView.SetText(cachedText)
		ui.detailsView.ScrollToBeginning()
		return
	}

	ui.detailsView.SetText("Fetching details...")

	go func() {
		details := ui.FetchAndBuildDetails(pkg.Name, pkg.Source)

		ui.cacheMutex.Lock()
		ui.installDetailsCache[pkg.Name] = details
		ui.cacheMutex.Unlock()

		ui.app.QueueUpdateDraw(func() {
			if ui.selectedPkg != nil && ui.selectedPkg.Name == pkg.Name {
				ui.detailsView.SetText(details)
				ui.detailsView.ScrollToBeginning()
			}
		})
	}()
}

// updateInstallRightFlex ensures the installation details layout is correctly initialized
func (ui *UI) updateInstallRightFlex() {
	if ui.installRightFlex == nil {
		return
	}
	if ui.installRightFlex.GetItemCount() != 1 {
		ui.installRightFlex.Clear()
		ui.installRightFlex.AddItem(ui.detailsView, 0, 1, false)
	}
}

// renderSelectedTable updates the list of packages selected for pending operations
func (ui *UI) renderSelectedTable(selectedPkgs []string) {
	if ui.selectedTable == nil {
		return
	}
	ui.selectedTable.Clear()
	for i, name := range selectedPkgs {
		cell := tview.NewTableCell(name).SetTextColor(tcell.ColorDefault).SetExpansion(1)
		ui.selectedTable.SetCell(i, 0, cell)
	}
	ui.selectedTable.SetTitle(fmt.Sprintf(" Selected Packages (%d) ", len(selectedPkgs)))

	if len(selectedPkgs) > 0 {
		row, _ := ui.selectedTable.GetSelection()
		if row < 0 || row >= len(selectedPkgs) {
			ui.selectedTable.Select(0, 0)
		}
	}
}

// promptInstall displays a confirmation dialog before initiating an installation command
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

// promptUninstall displays a confirmation dialog before initiating an uninstallation command
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

// performInstallOrUninstall handles the execution of system commands for package management
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

		ui.runHooks("install", true)

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
			for p := range strings.FieldsSeq(pkgName) {
				if isInstall {
					_ = pkglist.AddPackage(ui.conf.PackagesPath, ui.conf.PackagesFile, p)
					util.PrintSuccess("Package '%s' installed and added to %s.\n", p, ui.conf.PackagesFile)
				} else {
					_ = pkglist.RemovePackage(ui.conf.PackagesPath, ui.conf.PackagesFile, p)
					util.PrintSuccess("Package '%s' uninstalled and removed from %s.\n", p, ui.conf.PackagesFile)
				}
			}
		} else {
			util.PrintError("Command failed: %v\n", err)
		}

		ui.runHooks("install", false)

		fmt.Println("\nPress ENTER to return to drxpkg...")
		_, _ = os.Stdin.Read(make([]byte, 1))
	})

	_ = ui.reinitPacmanDbs()

	switch ui.activeTab {
	case tabInstall:
		ui.selectedInstall = make(map[string]bool)
		if ui.lastSearchTerm != "" {
			ui.forceSearch(ui.lastSearchTerm)
		}
	case tabUpdate:
		ui.updatePackages = nil
		ui.checkForUpdates()
	}
}

// highlightFuzzy applies color formatting to characters in the package name that match the search term
func highlightFuzzy(name, term string, isSelected bool) string {
	if isSelected {
		return "[white::b]" + name + "[-]"
	}
	if term == "" {
		return "[white]" + name + "[-]"
	}
	termLower := strings.ToLower(term)
	nameLower := strings.ToLower(name)

	var sb strings.Builder
	termIdx := 0
	inMatch := false

	matchColor := "blue::b"
	normalColor := "default::-"

	for i := 0; i < len(name); i++ {
		char := name[i]
		charLower := nameLower[i]

		isMatch := false
		if termIdx < len(termLower) && charLower == termLower[termIdx] {
			isMatch = true
			termIdx++
		}

		if isMatch {
			if !inMatch {
				if i > 0 {
					sb.WriteString("[-]")
				}
				sb.WriteString("[" + matchColor + "]")
				inMatch = true
			}
		} else {
			if inMatch || i == 0 {
				if i > 0 {
					sb.WriteString("[-]")
				}
				sb.WriteString("[" + normalColor + "]")
				inMatch = false
			}
		}
		sb.WriteByte(char)
	}
	sb.WriteString("[-]")
	return sb.String()
}

// updateTableFuzzyHighlights refreshes the highlighting for all rows in the package table
func (ui *UI) updateTableFuzzyHighlights(selectedRow int) {
	for idx, p := range ui.shownPackages {
		row := idx + 1
		cell := ui.pkgTable.GetCell(row, installColPackage)
		if cell != nil {
			isSelected := (row == selectedRow)
			cell.SetText(highlightFuzzy(p.Name, ui.lastSearchTerm, isSelected))
		}
	}
}

// moveSelectionDown updates the table selection to the next row and synchronizes the UI
func (ui *UI) moveSelectionDown() {
	if len(ui.shownPackages) == 0 {
		return
	}
	row, _ := ui.pkgTable.GetSelection()
	nextRow := row + 1
	if nextRow > len(ui.shownPackages) {
		nextRow = 1
	}
	ui.pkgTable.Select(nextRow, 0)
	ui.updateSearchInputFromSelection(nextRow)
}

// moveSelectionUp updates the table selection to the previous row and synchronizes the UI
func (ui *UI) moveSelectionUp() {
	if len(ui.shownPackages) == 0 {
		return
	}
	row, _ := ui.pkgTable.GetSelection()
	prevRow := row - 1
	if prevRow < 1 {
		prevRow = len(ui.shownPackages)
	}
	ui.pkgTable.Select(prevRow, 0)
	ui.updateSearchInputFromSelection(prevRow)
}

// updateSearchInputFromSelection syncs the search input field with the currently selected package
func (ui *UI) updateSearchInputFromSelection(row int) {
	if row > 0 && row <= len(ui.shownPackages) {
		pkgName := ui.shownPackages[row-1].Name
		ui.ignoreSearchChange = true
		ui.searchField.SetText(pkgName)
		ui.ignoreSearchChange = false

		pkg := ui.shownPackages[row-1]
		ui.selectedPkg = &pkg
		ui.loadPackageDetails(pkg)
	}
}

// getCanonicalPackageName attempts to resolve a package name to its correct case-sensitive version
func (ui *UI) getCanonicalPackageName(name string) (string, bool) {
	nameLower := strings.ToLower(name)

	for _, p := range ui.shownPackages {
		if strings.ToLower(p.Name) == nameLower {
			return p.Name, true
		}
	}

	ui.alpmMutex.Lock()
	defer ui.alpmMutex.Unlock()
	for _, cp := range ui.pkgsCache {
		if cp.NameLower == nameLower {
			return cp.Name, true
		}
	}

	return "", false
}

// attemptInstallExact verifies a package name exists before prompting the user for installation
func (ui *UI) attemptInstallExact(name string) {
	canonicalName, exists := ui.getCanonicalPackageName(name)
	if exists {
		ui.promptInstall(canonicalName)
		return
	}
	if ui.selectedPkg != nil {
		ui.promptInstall(ui.selectedPkg.Name)
	}
}
