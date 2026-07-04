// Package tui manages the Terminal User Interface, layouts, user input, and screen rendering.
package tui

import (
	"fmt"
	"io"
	"net/http"
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

func (ui *UI) setupUpdatePage() {
	ui.updateTable = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	ui.updateTable.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite).Attributes(tcell.AttrBold))
	ui.updateTable.SetBorder(true).SetBorderColor(tcell.ColorDefault).SetTitle(" Updates ")

	ui.updateDetails = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true)
	ui.updateDetails.SetBorder(true).SetBorderColor(tcell.ColorDefault).SetTitle(" Details / PKGBUILD Changes ")

	ui.updateTable.SetFocusFunc(func() {
		ui.updateTable.SetBorderColor(tcell.ColorBlue)
	})
	ui.updateTable.SetBlurFunc(func() {
		ui.updateTable.SetBorderColor(tcell.ColorDefault)
	})

	ui.updateDetails.SetFocusFunc(func() {
		ui.updateDetails.SetBorderColor(tcell.ColorBlue)
	})
	ui.updateDetails.SetBlurFunc(func() {
		ui.updateDetails.SetBorderColor(tcell.ColorDefault)
	})

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

		if event.Key() == tcell.KeyRune {
			r := event.Rune()
			if r == 'g' {
				if ui.lastKeyWasG {
					if len(ui.updatePackages) > 0 {
						ui.updateTable.ScrollToBeginning()
						ui.updateTable.Select(1, 0)
					}
					ui.lastKeyWasG = false
				} else {
					ui.lastKeyWasG = true
				}
				return nil
			} else if r == 'G' {
				if len(ui.updatePackages) > 0 {
					ui.updateTable.ScrollToEnd()
					ui.updateTable.Select(len(ui.updatePackages), 0)
				}
				ui.lastKeyWasG = false
				return nil
			}
		}
		ui.lastKeyWasG = false

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
		if event.Key() == tcell.KeyRune && (event.Rune() == 'v' || event.Rune() == 'V') {
			ui.inVisualMode = !ui.inVisualMode
			if ui.inVisualMode {
				row, _ := ui.updateTable.GetSelection()
				ui.visualStartRow = row
				ui.visualEndRow = row
			}
			ui.updateStatusDisplay()
			ui.renderUpdateTable()
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

func (ui *UI) renderUpdateTable() {
	ui.isRendering = true
	defer func() { ui.isRendering = false }()

	selectedRow, _ := ui.updateTable.GetSelection()

	ui.updateTable.Clear()

	// Header row
	ui.updateTable.SetCell(0, 0, tview.NewTableCell("").SetSelectable(false).SetMaxWidth(8))
	ui.updateTable.SetCell(0, 1, tview.NewTableCell("Package").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetExpansion(1))
	ui.updateTable.SetCell(0, 2, tview.NewTableCell("Current").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetMaxWidth(20))
	ui.updateTable.SetCell(0, 3, tview.NewTableCell("").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetMaxWidth(4))
	ui.updateTable.SetCell(0, 4, tview.NewTableCell("New").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetMaxWidth(20))
	ui.updateTable.SetCell(0, 5, tview.NewTableCell("Source").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetMaxWidth(12))

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

		ui.updateTable.SetCell(row, 0, selCell)
		ui.updateTable.SetCell(row, 1, pkgCell)
		ui.updateTable.SetCell(row, 2, currCell)
		ui.updateTable.SetCell(row, 3, arrowCell)
		ui.updateTable.SetCell(row, 4, newCell)
		ui.updateTable.SetCell(row, 5, sourceCell)
	}

	if selectedRow > 0 && selectedRow <= len(ui.updatePackages) {
		ui.updateTable.Select(selectedRow, 0)
	} else if len(ui.updatePackages) > 0 {
		ui.updateTable.Select(1, 0)
	}
}

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
		linesAur := strings.Split(string(outAur), "\n")
		for _, line := range linesAur {
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
		linesRepo := strings.Split(string(outRepo), "\n")
		for _, line := range linesRepo {
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
			end := i + chunkSize
			if end > len(foreignPkgs) {
				end = len(foreignPkgs)
			}
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

func countAur(pkgs []pkgmgr.UpdatePackage) int {
	count := 0
	for _, p := range pkgs {
		if p.Source == "AUR" {
			count++
		}
	}
	return count
}

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

	var info pkgmgr.SearchResults
	if pkg.Source == "AUR" {
		info = pkgmgr.InfoAur("", 5000, pkg.Name)
	} else {
		ui.alpmMutex.Lock()
		info = pkgmgr.InfoPacman(ui.alpmHandle, pkg.Name)
		ui.alpmMutex.Unlock()
	}

	var pkgBase string
	if len(info.Results) > 0 {
		pkgBase = info.Results[0].PackageBase
	}
	if pkgBase == "" {
		pkgBase = pkg.Name
	}

	var localPKGBUILD string
	var localPath string
	var remotePKGBUILD string

	if pkg.Source == "AUR" {
		home, err := os.UserHomeDir()
		if err == nil {
			localPath = filepath.Join(home, ".cache/yay", pkgBase, "PKGBUILD")
			data, err := os.ReadFile(localPath)
			if err == nil {
				localPKGBUILD = string(data)
			}
		}
	}

	// Fetch remote PKGBUILD
	remoteURL := pkgmgr.GetPkgbuildURL(pkg.Source, pkgBase)
	if remoteURL != "" {
		remotePKGBUILD, _ = getPkgbuildContentWithTimeout(remoteURL, 5*time.Second)
	}

	var sb strings.Builder
	if pkg.OutOfDate {
		fmt.Fprintf(&sb, "[red]------------------------------------------------------------------[-]\n")
		fmt.Fprintf(&sb, "[red]WARNING:[-] Flagged OUT OF DATE in the AUR.\n")
		fmt.Fprintf(&sb, "It is recommended to avoid updating this package or uninstall it.\n")
		fmt.Fprintf(&sb, "[red]------------------------------------------------------------------[-]\n\n")
	}
	if len(info.Results) > 0 {
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
	} else {
		fmt.Fprintf(&sb, "[blue]Package:[-] %s\n", pkg.Name)
		fmt.Fprintf(&sb, "[blue]Current Version:[-] %s\n", pkg.LocalVersion)
		fmt.Fprintf(&sb, "[blue]New Version:[-] %s\n", pkg.NewVersion)
		fmt.Fprintf(&sb, "[blue]Source:[-] %s\n\n", pkg.Source)
	}

	if pkg.Source == "AUR" {
		fmt.Fprintf(&sb, "\n[yellow]----------------- PKGBUILD Diff -----------------[-]\n")
		if remotePKGBUILD == "" {
			fmt.Fprintf(&sb, "[red]Failed to fetch remote PKGBUILD from AUR cgit.[-]\n")
		} else if localPKGBUILD == "" {
			fmt.Fprintf(&sb, "%s", formatDiff(remotePKGBUILD))
		} else {
			diffOut, err := runDiff(localPKGBUILD, remotePKGBUILD)
			if err != nil || diffOut == "" {
				fmt.Fprintf(&sb, "%s", formatDiff(remotePKGBUILD))
			} else {
				fmt.Fprintf(&sb, "%s", formatDiff(diffOut))
			}
		}
	} else {
		if remotePKGBUILD != "" {
			fmt.Fprintf(&sb, "\n[yellow]----------------- Remote PKGBUILD -----------------[-]\n")
			fmt.Fprintf(&sb, "%s", formatDiff(remotePKGBUILD))
		} else {
			fmt.Fprintf(&sb, "\n[gray]No PKGBUILD diff/content available for repository packages.[-]\n")
		}
	}

	ui.cacheMutex.Lock()
	ui.updateDetailsCache[pkg.Name] = sb.String()
	ui.cacheMutex.Unlock()

	ui.app.QueueUpdateDraw(func() {
		if ui.selectedUpdate != nil && ui.selectedUpdate.Name == pkg.Name {
			ui.updateDetails.SetText(sb.String())
			ui.updateDetails.ScrollToBeginning()
		}
	})
}

func getPkgbuildContentWithTimeout(url string, timeout time.Duration) (string, error) {
	client := http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
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
	defer os.Remove(tmpLocal.Name())
	defer tmpLocal.Close()

	if _, err := tmpLocal.WriteString(localContent); err != nil {
		return "", err
	}

	tmpRemote, err := os.CreateTemp("", "drxpkg-remote-")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpRemote.Name())
	defer tmpRemote.Close()

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

func hasFlag(args []string, flag string) bool {
	return slices.Contains(args, flag)
}
