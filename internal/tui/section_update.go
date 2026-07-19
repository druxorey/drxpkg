// Package tui manages the Terminal User Interface, layouts, user input, and screen rendering.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/druxorey/drxpkg/internal/config"
	"github.com/druxorey/drxpkg/internal/pkgmgr"
	"github.com/druxorey/drxpkg/internal/util"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// setupUpdatePage initializes the UI components and input handlers for the update page
func (ui *UI) setupUpdatePage() {
	ui.updateTable = ui.createStandardTable(" Updates ", 1, 0)
	ui.updateDetails = ui.createStandardTextView(" Details ", true)

	ui.updateTable.SetSelectionChangedFunc(func(row, column int) {
		if ui.isRendering {
			return
		}
		if ui.inVisualMode {
			ui.visualEndRow = row
			ui.renderUpdateTable()
		}
		if row <= 0 || row > len(ui.updatePackages) {
			ui.selectedUpdate = nil
			ui.updateDetails.Clear()
			return
		}
		pkg := ui.updatePackages[row-1]
		ui.selectedUpdate = &pkg
		ui.loadUpdateDetails(pkg)
	})

	ui.updateTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, _ := ui.updateTable.GetSelection()

		if ui.handleTableVimNavigation(event, ui.updateTable, len(ui.updatePackages)) {
			return nil
		}

		if event.Key() == tcell.KeyTAB {
			ui.app.SetFocus(ui.updateDetails)
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			if ui.inVisualMode {
				ui.inVisualMode = false
				ui.updateStatusDisplay()
				ui.renderUpdateTable()
				return nil
			}
		}

		if ui.handleVisualModeToggle(event, ui.updateTable, ui.renderUpdateTable) {
			return nil
		}

		if event.Key() == tcell.KeyRune && event.Rune() == ' ' {
			if ui.inVisualMode {
				minRow := min(ui.visualStartRow, ui.visualEndRow)
				maxRow := max(ui.visualStartRow, ui.visualEndRow)
				for r := minRow; r <= maxRow; r++ {
					if r > 0 && r <= len(ui.updatePackages) {
						if !ui.updatePackages[r-1].NotInAur {
							ui.updatePackages[r-1].Selected = !ui.updatePackages[r-1].Selected
						}
					}
				}
				ui.inVisualMode = false
				ui.updateStatusDisplay()
				ui.renderUpdateTable()
			} else {
				if row > 0 && row <= len(ui.updatePackages) {
					ui.togglePackageSelection(row - 1)
					ui.updateTable.Select(row, 0)
				}
			}
			return nil
		}
		if event.Key() == tcell.KeyRune && (event.Rune() == 'a' || event.Rune() == 'A') {
			ui.toggleSelectAll()
			return nil
		}
		if event.Key() == tcell.KeyRune && (event.Rune() == 'i' || event.Rune() == 'I') {
			ui.runUpgradeProcess()
			return nil
		}
		if event.Key() == tcell.KeyEnter {
			if ui.inVisualMode {
				minRow := min(ui.visualStartRow, ui.visualEndRow)
				maxRow := max(ui.visualStartRow, ui.visualEndRow)
				for r := minRow; r <= maxRow; r++ {
					if r > 0 && r <= len(ui.updatePackages) {
						if !ui.updatePackages[r-1].NotInAur {
							ui.updatePackages[r-1].Selected = !ui.updatePackages[r-1].Selected
						}
					}
				}
				ui.inVisualMode = false
				ui.updateStatusDisplay()
				ui.renderUpdateTable()
			} else {
				ui.runUpgradeProcess()
			}
			return nil
		}
		return event
	})

	ui.updateDetails.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB || event.Key() == tcell.KeyBacktab {
			ui.app.SetFocus(ui.updateTable)
			return nil
		}
		return event
	})

	ui.updatePageFlex = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ui.updateTable, 0, 1, true).
		AddItem(ui.updateDetails, 0, 1, false)
}

// renderUpdateTable populates and renders the updates table with package information and status
func (ui *UI) renderUpdateTable() {
	ui.isRendering = true
	defer func() { ui.isRendering = false }()

	selectedRow, _ := ui.updateTable.GetSelection()

	ui.updateTable.Clear()

	// Header row
	ui.updateTable.SetCell(0, updateColSelect, tview.NewTableCell("").SetSelectable(false).SetMaxWidth(8))
	ui.updateTable.SetCell(0, updateColPackage, tview.NewTableCell("Package").SetTextColor(ui.theme.PrimaryColor).SetSelectable(false).SetExpansion(1))
	ui.updateTable.SetCell(0, updateColCurrent, tview.NewTableCell("Current").SetTextColor(ui.theme.PrimaryColor).SetSelectable(false).SetMaxWidth(20))
	ui.updateTable.SetCell(0, updateColArrow, tview.NewTableCell("").SetTextColor(ui.theme.PrimaryColor).SetSelectable(false).SetMaxWidth(4))
	ui.updateTable.SetCell(0, updateColNew, tview.NewTableCell("New").SetTextColor(ui.theme.PrimaryColor).SetSelectable(false).SetMaxWidth(20))
	ui.updateTable.SetCell(0, updateColSource, tview.NewTableCell("Source").SetTextColor(ui.theme.PrimaryColor).SetSelectable(false).SetMaxWidth(12))

	for idx, p := range ui.updatePackages {
		row := idx + 1
		isHighlighted := false
		if ui.inVisualMode && ui.activeTab == 1 {
			minRow := min(ui.visualStartRow, ui.visualEndRow)
			maxRow := max(ui.visualStartRow, ui.visualEndRow)
			if row >= minRow && row <= maxRow {
				isHighlighted = true
			}
		}

		selStr := "   "
		if p.Selected {
			selStr = "  x"
		}
		selCell := tview.NewTableCell(selStr).SetMaxWidth(8).SetAlign(tview.AlignLeft)
		if p.Selected {
			selCell.SetTextColor(tcell.ColorGreen)
		}

		pkgCell := tview.NewTableCell(p.Name).SetExpansion(1)
		currCell := tview.NewTableCell(p.LocalVersion).SetMaxWidth(20)
		arrowCell := tview.NewTableCell("->").SetTextColor(tcell.ColorGray).SetMaxWidth(4)
		newCell := tview.NewTableCell(p.NewVersion).SetMaxWidth(20)

		sourceColor := getSourceColor(p.Source)
		sourceCell := tview.NewTableCell(p.Source).SetTextColor(sourceColor).SetMaxWidth(12)

		if p.NotInAur {
			pkgCell.SetTextColor(tcell.ColorGray)
			currCell.SetTextColor(tcell.ColorGray)
			arrowCell.SetTextColor(tcell.ColorGray)
			newCell.SetTextColor(tcell.ColorGray)
			sourceCell.SetTextColor(tcell.ColorGray)
			sourceCell.SetText("Not in AUR")
			selCell.SetText("   ")
		} else if p.OutOfDate {
			pkgCell.SetTextColor(tcell.ColorRed)
			currCell.SetTextColor(tcell.ColorRed)
			arrowCell.SetTextColor(tcell.ColorRed)
			newCell.SetTextColor(tcell.ColorRed)
			sourceCell.SetTextColor(tcell.ColorRed)
		} else {
			if p.Selected {
				pkgCell.SetTextColor(tcell.ColorDefault)
				currCell.SetTextColor(tcell.ColorDefault)
				newCell.SetTextColor(tcell.ColorGreen)
			} else {
				pkgCell.SetTextColor(tcell.ColorGray)
				currCell.SetTextColor(tcell.ColorGray)
				newCell.SetTextColor(tcell.ColorGray)
			}
			if !p.Selected {
				sourceCell.SetTextColor(tcell.ColorGray)
			}
		}

		if isHighlighted {
			bgColor := tcell.NewHexColor(0x1a3a5c)
			selCell.SetBackgroundColor(bgColor)
			pkgCell.SetBackgroundColor(bgColor)
			currCell.SetBackgroundColor(bgColor)
			arrowCell.SetBackgroundColor(bgColor)
			newCell.SetBackgroundColor(bgColor)
			sourceCell.SetBackgroundColor(bgColor)
		}

		ui.updateTable.SetCell(row, updateColSelect, selCell)
		ui.updateTable.SetCell(row, updateColPackage, pkgCell)
		ui.updateTable.SetCell(row, updateColCurrent, currCell)
		ui.updateTable.SetCell(row, updateColArrow, arrowCell)
		ui.updateTable.SetCell(row, updateColNew, newCell)
		ui.updateTable.SetCell(row, updateColSource, sourceCell)
	}

	if selectedRow > 0 && selectedRow <= len(ui.updatePackages) {
		ui.updateTable.Select(selectedRow, 0)
	} else if len(ui.updatePackages) > 0 {
		ui.updateTable.Select(1, 0)
	}
}

// togglePackageSelection flips the selection state of a package at the given index and updates the UI
func (ui *UI) togglePackageSelection(index int) {
	if index < 0 || index >= len(ui.updatePackages) {
		return
	}
	if ui.updatePackages[index].NotInAur {
		return
	}
	ui.updatePackages[index].Selected = !ui.updatePackages[index].Selected
	ui.renderUpdateTable()
}

// toggleSelectAll toggles the selection status for all eligible packages in the update list
func (ui *UI) toggleSelectAll() {
	allSelected := true
	for _, p := range ui.updatePackages {
		if p.NotInAur {
			continue
		}
		if !p.Selected {
			allSelected = false
			break
		}
	}
	for i := range ui.updatePackages {
		if ui.updatePackages[i].NotInAur {
			continue
		}
		ui.updatePackages[i].Selected = !allSelected
	}
	ui.renderUpdateTable()
}

// checkForUpdates checks for pending system updates and updates the UI status display
func (ui *UI) checkForUpdates() {
	if ui.updatePackages != nil {
		ui.renderUpdateTable()
		if len(ui.updatePackages) == 0 {
			ui.setStatus("System is up to date.")
			ui.updateDetails.SetText("All packages are up to date!")
		} else {
			up, aur, ood, nia := ui.countUpdatesInfo(ui.updatePackages)
			var statusParts []string
			if up > 0 {
				statusParts = append(statusParts, fmt.Sprintf("%d updates (%d AUR)", up, aur))
			} else {
				statusParts = append(statusParts, "System is up to date")
			}
			if ood > 0 {
				statusParts = append(statusParts, fmt.Sprintf("%d out of date", ood))
			}
			if nia > 0 {
				statusParts = append(statusParts, fmt.Sprintf("%d not in AUR", nia))
			}
			ui.setStatus(strings.Join(statusParts, ", ") + ".")
		}
		return
	}

	ui.backgroundUpdateCheck()
}

// backgroundUpdateCheck executes shell commands asynchronously to detect repo and AUR updates
func (ui *UI) backgroundUpdateCheck() {
	ui.setStatus("Checking for updates...")
	if ui.updateDetails != nil {
		ui.updateDetails.SetText("Updating databases and checking for updates in the background...")
	}

	go func() {
		// 1. Run checkupdates to get pacman updates
		cmdRepo := exec.Command("checkupdates")
		outRepo, _ := cmdRepo.Output()

		// 2. Run yay -Qua to get AUR updates
		cmdAur := exec.Command("yay", "-Qua")
		outAur, _ := cmdAur.Output()

		var pkgs []pkgmgr.UpdatePackage

		// Parse AUR updates
		for line := range strings.SplitSeq(string(outAur), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			pkg, err := pkgmgr.ParseUpdateLine(line)
			if err == nil {
				pkg.Source = "AUR"
				pkgs = append(pkgs, *pkg)
			}
		}

		// Parse Repo updates
		for line := range strings.SplitSeq(string(outRepo), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			pkg, err := pkgmgr.ParseUpdateLine(line)
			if err == nil {
				pkg.Source = ui.getPackageSource(pkg.Name)
				if pkg.Source == "AUR" {
					pkg.Source = "repo"
				}
				pkgs = append(pkgs, *pkg)
			}
		}

		// 3. Get all foreign packages (installed packages not in any sync database)
		var foreignPkgs []string
		ui.alpmMutex.Lock()
		if ui.alpmHandle != nil {
			localDb, err := ui.alpmHandle.LocalDB()
			syncDbs, errSync := ui.alpmHandle.SyncDBs()
			if err == nil && errSync == nil {
				for _, pkg := range localDb.PkgCache().Slice() {
					name := pkg.Name()
					// Ignore debug packages
					if strings.HasSuffix(name, "-debug") {
						continue
					}
					found := false
					for _, db := range syncDbs.Slice() {
						if db.Pkg(name) != nil {
							found = true
							break
						}
					}
					if !found {
						foreignPkgs = append(foreignPkgs, name)
					}
				}
			}
		}
		ui.alpmMutex.Unlock()

		// Query AUR RPC in chunks of 100 to classify NotInAur and OutOfDate
		var aurResults []pkgmgr.InfoRecord
		const chunkSize = 100
		for i := 0; i < len(foreignPkgs); i += chunkSize {
			end := min(i + chunkSize, len(foreignPkgs))
			chunk := foreignPkgs[i:end]
			info := pkgmgr.InfoAur("", 5000, chunk...)
			aurResults = append(aurResults, info.Results...)
		}

		aurFound := make(map[string]pkgmgr.InfoRecord)
		for _, r := range aurResults {
			aurFound[r.Name] = r
		}

		var notInAurPkgs []string
		var outOfDatePkgs []string
		for _, name := range foreignPkgs {
			info, found := aurFound[name]
			if !found {
				notInAurPkgs = append(notInAurPkgs, name)
			} else if info.OutOfDate > 0 {
				outOfDatePkgs = append(outOfDatePkgs, name)
			}
		}

		getLocalVersion := func(name string) string {
			ui.alpmMutex.Lock()
			defer ui.alpmMutex.Unlock()
			if ui.alpmHandle == nil {
				return ""
			}
			localDb, err := ui.alpmHandle.LocalDB()
			if err != nil {
				return ""
			}
			pkg := localDb.Pkg(name)
			if pkg != nil {
				return pkg.Version()
			}
			return ""
		}

		for _, name := range notInAurPkgs {
			exists := false
			for _, p := range pkgs {
				if p.Name == name {
					exists = true
					break
				}
			}
			if !exists {
				pkgs = append(pkgs, pkgmgr.UpdatePackage{
					Name:         name,
					LocalVersion: getLocalVersion(name),
					NewVersion:   "-",
					Source:       "AUR",
					Selected:     false,
					NotInAur:     true,
				})
			}
		}

		for _, name := range outOfDatePkgs {
			foundIdx := -1
			for idx := range pkgs {
				if pkgs[idx].Name == name {
					foundIdx = idx
					break
				}
			}
			if foundIdx != -1 {
				pkgs[foundIdx].OutOfDate = true
			} else {
				pkgs = append(pkgs, pkgmgr.UpdatePackage{
					Name:         name,
					LocalVersion: getLocalVersion(name),
					NewVersion:   "-",
					Source:       "AUR",
					Selected:     false,
					OutOfDate:    true,
				})
			}
		}

		// Sort:
		// 1. OutOfDate at the top
		// 2. Normal updates (repo/AUR)
		// 3. NotInAur at the bottom
		sort.Slice(pkgs, func(i, j int) bool {
			if pkgs[i].OutOfDate != pkgs[j].OutOfDate {
				return pkgs[i].OutOfDate
			}
			if pkgs[i].NotInAur != pkgs[j].NotInAur {
				return !pkgs[i].NotInAur
			}
			aAur := pkgs[i].Source == "AUR"
			bAur := pkgs[j].Source == "AUR"
			if aAur != bAur {
				return aAur
			}
			return pkgs[i].Name < pkgs[j].Name
		})

		ui.app.QueueUpdateDraw(func() {
			ui.updatePackages = pkgs
			if ui.updateTable != nil {
				ui.renderUpdateTable()
			}
			if len(pkgs) == 0 {
				ui.setStatus("System is up to date.")
				if ui.updateDetails != nil {
					ui.updateDetails.SetText("All packages are up to date!")
				}
			} else {
				up, aur, ood, nia := ui.countUpdatesInfo(pkgs)
				var statusParts []string
				if up > 0 {
					statusParts = append(statusParts, fmt.Sprintf("%d updates (%d AUR)", up, aur))
				} else {
					statusParts = append(statusParts, "System is up to date")
				}
				if ood > 0 {
					statusParts = append(statusParts, fmt.Sprintf("%d out of date", ood))
				}
				if nia > 0 {
					statusParts = append(statusParts, fmt.Sprintf("%d not in AUR", nia))
				}
				ui.setStatus(strings.Join(statusParts, ", ") + ".")
				if ui.updateTable != nil && len(pkgs) > 0 {
					ui.updateTable.Select(1, 0)
				}
			}
		})

		// Reset Details Cache
		ui.cacheMutex.Lock()
		ui.updateDetailsCache = make(map[string]string)
		ui.cacheMutex.Unlock()

		// Preload details sequentially in the background, top-to-bottom
		go func() {
			for _, p := range pkgs {
				ui.preloadUpdateDetails(p)
				time.Sleep(300 * time.Millisecond)
			}
		}()
	}()
}

// countUpdatesInfo calculates metrics regarding the total, AUR, out-of-date, and local-only packages
func (ui *UI) countUpdatesInfo(pkgs []pkgmgr.UpdatePackage) (totalUpdates, aurUpdates, outOfDate, notInAur int) {
	for _, p := range pkgs {
		if p.NotInAur {
			notInAur++
		} else {
			totalUpdates++
			if p.Source == "AUR" {
				aurUpdates++
			}
			if p.OutOfDate {
				outOfDate++
			}
		}
	}
	return
}

// getPackageSource determines the database origin of a given package name
func (ui *UI) getPackageSource(pkgName string) string {
	ui.alpmMutex.Lock()
	defer ui.alpmMutex.Unlock()
	if ui.alpmHandle == nil {
		return "AUR"
	}
	dbs, err := ui.alpmHandle.SyncDBs()
	if err != nil {
		return "AUR"
	}
	for _, db := range dbs.Slice() {
		if db.Pkg(pkgName) != nil {
			return db.Name()
		}
	}
	return "AUR"
}

// loadUpdateDetails triggers the loading of package details into the UI, using a cache if available
func (ui *UI) loadUpdateDetails(pkg pkgmgr.UpdatePackage) {
	ui.cacheMutex.RLock()
	cachedText, exists := ui.updateDetailsCache[pkg.Name]
	ui.cacheMutex.RUnlock()

	if exists {
		ui.updateDetails.SetText(cachedText)
		ui.updateDetails.ScrollToBeginning()
		return
	}

	ui.updateDetails.SetText("Fetching details...")
	go ui.preloadUpdateDetails(pkg)
}

// preloadUpdateDetails fetches and formats package details for display, updating the cache and UI
func (ui *UI) preloadUpdateDetails(pkg pkgmgr.UpdatePackage) {
	ui.cacheMutex.RLock()
	_, exists := ui.updateDetailsCache[pkg.Name]
	ui.cacheMutex.RUnlock()
	if exists {
		return
	}

	if pkg.NotInAur {
		var sb strings.Builder
		fmt.Fprintf(&sb, "[blue]Package:[-] %s\n", pkg.Name)
		fmt.Fprintf(&sb, "[blue]Local Version:[-] %s\n", pkg.LocalVersion)
		fmt.Fprintf(&sb, "[blue]Source:[-] Not in AUR\n\n")
		fmt.Fprintf(&sb, "[yellow]Warning:[-] Local-only package (not in AUR).\n")
		fmt.Fprintf(&sb, "This package is installed locally but was not found in the AUR.\n")
		fmt.Fprintf(&sb, "It will not receive updates. You can uninstall it from the Install tab.\n")

		ui.cacheMutex.Lock()
		ui.updateDetailsCache[pkg.Name] = sb.String()
		ui.cacheMutex.Unlock()

		ui.app.QueueUpdateDraw(func() {
			if ui.selectedUpdate != nil && ui.selectedUpdate.Name == pkg.Name {
				ui.updateDetails.SetText(sb.String())
				ui.updateDetails.ScrollToBeginning()
			}
		})
		return
	}

	details := ui.FetchAndBuildDetails(pkg.Name, pkg.Source)

	ui.cacheMutex.Lock()
	ui.updateDetailsCache[pkg.Name] = details
	ui.cacheMutex.Unlock()

	ui.app.QueueUpdateDraw(func() {
		if ui.selectedUpdate != nil && ui.selectedUpdate.Name == pkg.Name {
			ui.updateDetails.SetText(details)
			ui.updateDetails.ScrollToBeginning()
		}
	})
}

// runUpgradeProcess suspends the UI to perform the system upgrade via the configured command line interface
func (ui *UI) runUpgradeProcess() {
	var selectedCount int
	var ignoreList []string
	for _, p := range ui.updatePackages {
		if p.Selected {
			selectedCount++
		} else {
			ignoreList = append(ignoreList, p.Name)
		}
	}

	if len(ui.updatePackages) > 0 && selectedCount == 0 {
		ui.setStatus("[red]Error: No packages selected for upgrade.")
		return
	}

	ui.app.Suspend(func() {
		fmt.Print("\033[H\033[2J")
		fmt.Printf(" ➤ \033[1;34mStarting system package update...\033[0m\n")

		cmdStr := ui.conf.SysUpgradeCmd
		if cmdStr == "" {
			cmdStr = "yay"
		}

		args := []string{}
		parts := strings.Fields(cmdStr)
		binary := parts[0]

		switch binary {
		case "yay":
			if len(parts) == 1 {
				args = append(args, "-Syu", "--noconfirm")
			} else {
				args = append(args, parts[1:]...)
				if !hasFlag(args, "--noconfirm") {
					args = append(args, "--noconfirm")
				}
			}
		case "pacman":
			args = append(args, parts[1:]...)
			if !hasFlag(args, "--noconfirm") {
				args = append(args, "--noconfirm")
			}
		default:
			if len(parts) > 1 {
				args = append(args, parts[1:]...)
			}
		}

		if len(ignoreList) > 0 {
			args = append(args, "--ignore", strings.Join(ignoreList, ","))
		}

		// Ensure we run system upgrade command. If pacman, run via sudo
		var upgradeCmd *exec.Cmd
		if binary == "pacman" {
			upgradeCmd = exec.Command("sudo", append([]string{binary}, args...)...)
			fmt.Printf("Running: sudo %s %s\n\n", binary, strings.Join(args, " "))
		} else {
			upgradeCmd = exec.Command(binary, args...)
			fmt.Printf("Running: %s %s\n\n", binary, strings.Join(args, " "))
		}

		upgradeCmd.Stdin = os.Stdin
		upgradeCmd.Stdout = os.Stdout
		upgradeCmd.Stderr = os.Stderr

		err := upgradeCmd.Run()
		if err != nil {
			util.PrintError("\nSystem upgrade failed: %v\n", err)
		} else {
			util.PrintSuccess("\nSystem upgrade completed successfully.\n")
		}

		ui.runUpdateHooks()

		fmt.Println("\nPress ENTER to return to drxpkg...")
		_, _ = os.Stdin.Read(make([]byte, 1))
	})

	_ = ui.reinitPacmanDbs()
	ui.updatePackages = nil
	ui.checkForUpdates()
}

// runUpdateHooks executes user-defined post-update scripts found in the configuration directory
func (ui *UI) runUpdateHooks() {
	if !ui.conf.RunUpdateHooks {
		fmt.Printf("\nUpdate hooks are disabled in settings.\n")
		return
	}

	configDir, err := config.GetConfigDir()
	if err != nil {
		util.PrintError("\nFailed to get config directory: %v\n", err)
		return
	}

	hooksDir := filepath.Join(configDir, "update_hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		util.PrintError("\nFailed to create update hooks directory: %v\n", err)
		return
	}

	files, err := os.ReadDir(hooksDir)
	if err != nil {
		util.PrintError("\nFailed to read update hooks directory: %v\n", err)
		return
	}

	var hookFiles []string
	for _, file := range files {
		if !file.IsDir() {
			hookFiles = append(hookFiles, file.Name())
		}
	}

	if len(hookFiles) == 0 {
		fmt.Printf("\nNo update hooks found in %s\n", hooksDir)
		return
	}

	// Sort alphabetically to run sequentially
	sort.Strings(hookFiles)

	fmt.Printf("\n ➤ \033[1;34mRunning post-update hooks...\033[0m\n")
	for _, filename := range hookFiles {
		scriptPath := filepath.Join(hooksDir, filename)
		fmt.Printf("\n ➤ \033[1;34mExecuting hook: %s...\033[0m\n", filename)

		cmd := exec.Command("bash", scriptPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			util.PrintError("Hook '%s' failed: %v\n", filename, err)
		} else {
			util.PrintSuccess("Hook '%s' completed.\n", filename)
		}
	}
}

// hasFlag checks for the presence of a specific flag within a slice of command arguments
func hasFlag(args []string, flag string) bool {
	return slices.Contains(args, flag)
}
