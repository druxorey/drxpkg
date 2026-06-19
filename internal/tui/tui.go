// Package tui does something
package tui

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Jguer/go-alpm/v2"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/druxorey/drxpkg/internal/config"
	"github.com/druxorey/drxpkg/internal/pkglist"
)

type cachedPkg struct {
	Package
	Description      string
	NameLower        string
	DescriptionLower string
}

type UI struct {
	conf       *config.Settings
	app        *tview.Application
	alpmHandle *alpm.Handle
	alpmMutex  sync.Mutex

	// Main Layout
	grid                 *tview.Grid
	tabBar               *tview.TextView
	pages                *tview.Pages
	settingsGrid         *tview.Grid
	settingInputs        []*tview.InputField
	settingAurCb         *tview.Checkbox
	btnSave              *tview.TextView
	btnDefaults          *tview.TextView
	settingsFocusedIndex int
	settingsEditMode     bool

	// Tab 1: Install Views
	searchField *tview.InputField
	pkgTable    *tview.Table
	detailsView *tview.TextView
	statusText  *tview.TextView

	// Tab 2: Update Views
	updatePageFlex    *tview.Flex
	updateTable       *tview.Table
	updateDetails     *tview.TextView
	updatePackages    []UpdatePackage
	selectedUpdate    *UpdatePackage
	updateDetailsCache map[string]string
	cacheMutex         sync.RWMutex

	// State
	activeTab      int
	lastSearchTerm string
	shownPackages  []Package
	selectedPkg    *Package
	// searching      bool

	// Fast Search Cache & Debouncing
	searchMutex  sync.Mutex
	searchCancel context.CancelFunc
	searchTimer  *time.Timer
	pkgsCache    []cachedPkg
}

func New(conf *config.Settings) (*UI, error) {
	// Configure global tview styles to inherit terminal transparency
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDefault
	tview.Styles.ContrastBackgroundColor = tcell.ColorDefault
	tview.Styles.MoreContrastBackgroundColor = tcell.ColorDefault
	tview.Styles.BorderColor = tcell.ColorBlue
	tview.Styles.TitleColor = tcell.ColorBlue
	tview.Styles.GraphicsColor = tcell.ColorBlue
	tview.Styles.PrimaryTextColor = tcell.ColorDefault
	tview.Styles.SecondaryTextColor = tcell.ColorDefault
	tview.Styles.TertiaryTextColor = tcell.ColorDefault

	ui := &UI{
		conf:               conf,
		app:                tview.NewApplication(),
		activeTab:          0,
		updateDetailsCache: make(map[string]string),
	}

	var err error
	ui.alpmHandle, err = InitPacmanDbs(conf.PacmanDBPath, conf.PacmanConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to init pacman db: %w", err)
	}

	ui.setupWidgets()
	ui.setupLayout()
	ui.setupKeyboard()

	ui.rebuildCache()
	ui.backgroundUpdateCheck()

	return ui, nil
}

func (ui *UI) Start() error {
	ui.drawTabBar()
	ui.app.SetRoot(ui.grid, true).EnableMouse(true)
	return ui.app.Run()
}

func (ui *UI) reinitPacmanDbs() error {
	ui.alpmMutex.Lock()
	if ui.alpmHandle != nil {
		_ = ui.alpmHandle.Release()
	}
	var err error
	ui.alpmHandle, err = InitPacmanDbs(ui.conf.PacmanDBPath, ui.conf.PacmanConfigPath)
	ui.alpmMutex.Unlock()

	if err == nil {
		ui.rebuildCache()
	}
	return err
}

func (ui *UI) setupWidgets() {
	// Tab bar
	ui.tabBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	// Tab 1 Widgets
	ui.searchField = tview.NewInputField().
		SetLabel("Search: ").
		SetLabelColor(tcell.ColorBlue).
		SetFieldTextColor(tcell.ColorDefault).
		SetFieldBackgroundColor(tcell.ColorDefault)
	ui.searchField.SetBorder(true).SetBorderColor(tcell.ColorDefault)
	ui.searchField.SetChangedFunc(func(text string) {
		ui.handleSearchChange(text)
	})
	ui.searchField.SetFocusFunc(func() {
		ui.searchField.SetBorderColor(tcell.ColorBlue)
	})
	ui.searchField.SetBlurFunc(func() {
		ui.searchField.SetBorderColor(tcell.ColorDefault)
	})

	ui.pkgTable = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	ui.pkgTable.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite).Attributes(tcell.AttrBold))
	ui.pkgTable.SetBorder(true).SetBorderColor(tcell.ColorDefault)
	ui.pkgTable.SetFocusFunc(func() {
		ui.pkgTable.SetBorderColor(tcell.ColorBlue)
	})
	ui.pkgTable.SetBlurFunc(func() {
		ui.pkgTable.SetBorderColor(tcell.ColorDefault)
	})

	ui.detailsView = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true)
	ui.detailsView.SetBorder(true).SetTitle(" Details ")

	ui.statusText = tview.NewTextView().
		SetDynamicColors(true)

	// Tab 1 Packages Selection Change
	ui.pkgTable.SetSelectionChangedFunc(func(row, column int) {
		if row <= 0 || row > len(ui.shownPackages) {
			ui.selectedPkg = nil
			ui.detailsView.Clear()
			return
		}
		pkg := ui.shownPackages[row-1]
		ui.selectedPkg = &pkg
		ui.loadPackageDetails(pkg)
	})
	ui.setupUpdatePage()
	ui.setupSettingsPanel()
}

func (ui *UI) setupLayout() {
	ui.pages = tview.NewPages()

	// Tab 1: Installation Page
	installFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.searchField, 3, 0, true).
		AddItem(ui.pkgTable, 0, 1, false)

	installPage := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(installFlex, 0, 1, true).
		AddItem(ui.detailsView, 0, 1, false)



	// Tab 3: Package Management Placeholder
	managePage := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("\n\n[blue]Package Management[-]\n\nThis page is currently a placeholder.")

	// Tab 4: Settings Page
	ui.pages.AddPage("install", installPage, true, true)
	ui.pages.AddPage("update", ui.updatePageFlex, true, false)
	ui.pages.AddPage("manage", managePage, true, false)
	ui.pages.AddPage("settings", ui.settingsGrid, true, false)

	// Main Grid Layout
	ui.grid = tview.NewGrid().
		SetRows(1, 0, 1).
		SetColumns(0).
		AddItem(ui.tabBar, 0, 0, 1, 1, 0, 0, false).
		AddItem(ui.pages, 1, 0, 1, 1, 0, 0, true).
		AddItem(ui.statusText, 2, 0, 1, 1, 0, 0, false)
}

func (ui *UI) drawTabBar() {
	tabs := []string{"[1] Install", "[2] Update", "[3] Manage", "[4] Settings"}
	var styledTabs []string
	for i, tab := range tabs {
		if i == ui.activeTab {
			styledTabs = append(styledTabs, fmt.Sprintf("[blue::b]%s[-:-:-]", tab))
		} else {
			styledTabs = append(styledTabs, tab)
		}
	}
	ui.tabBar.SetText(strings.Join(styledTabs, "   "))
}

func (ui *UI) switchTab(tabIndex int) {
	if tabIndex < 0 || tabIndex > 3 {
		return
	}
	ui.activeTab = tabIndex
	ui.drawTabBar()

	switch tabIndex {
	case 0:
		ui.pages.SwitchToPage("install")
		ui.app.SetFocus(ui.searchField)
	case 1:
		ui.pages.SwitchToPage("update")
		ui.app.SetFocus(ui.updateTable)
		ui.checkForUpdates()
	case 2:
		ui.pages.SwitchToPage("manage")
	case 3:
		ui.pages.SwitchToPage("settings")
		ui.app.SetFocus(ui.settingsGrid)
	}
}

func (ui *UI) setupKeyboard() {
	// Global captures
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		focused := ui.app.GetFocus()
		_, isInputField := focused.(*tview.InputField)

		if !isInputField {
			if event.Key() == tcell.KeyEscape || event.Rune() == 'q' || event.Rune() == 'Q' {
				ui.app.Stop()
				return nil
			}
		} else {
			if event.Key() == tcell.KeyCtrlQ {
				ui.app.Stop()
				return nil
			}
		}
		// F-keys or Alt/Ctrl numbers to switch tabs
		if event.Key() == tcell.KeyF1 {
			ui.switchTab(0)
			return nil
		}
		if event.Key() == tcell.KeyF2 {
			ui.switchTab(1)
			return nil
		}
		if event.Key() == tcell.KeyF3 {
			ui.switchTab(2)
			return nil
		}
		if event.Key() == tcell.KeyF4 {
			ui.switchTab(3)
			return nil
		}
		if event.Rune() == '[' {
			ui.switchTab((ui.activeTab + 3) % 4)
			return nil
		}
		if event.Rune() == ']' {
			ui.switchTab((ui.activeTab + 1) % 4)
			return nil
		}

		return event
	})

	// Search input captured
	ui.searchField.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			term := strings.TrimSpace(ui.searchField.GetText())
			ui.forceSearch(term)
		case tcell.KeyTAB, tcell.KeyDown:
			ui.app.SetFocus(ui.pkgTable)
		}
	})

	// Table list captured
	ui.pkgTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB {
			ui.app.SetFocus(ui.searchField)
			return nil
		}
		if event.Key() == tcell.KeyEnter {
			if ui.selectedPkg != nil {
				ui.installOrUninstallPackage(*ui.selectedPkg)
			}
			return nil
		}
		return event
	})
}

func (ui *UI) rebuildCache() {
	ui.alpmMutex.Lock()
	defer ui.alpmMutex.Unlock()

	if ui.alpmHandle == nil {
		return
	}

	dbs, err := ui.alpmHandle.SyncDBs()
	if err != nil {
		return
	}

	local, err := ui.alpmHandle.LocalDB()
	if err != nil {
		return
	}

	var cache []cachedPkg

	// Sync DBs
	for _, db := range dbs.Slice() {
		for _, pkg := range db.PkgCache().Slice() {
			cache = append(cache, cachedPkg{
				Package: Package{
					Name:         pkg.Name(),
					Source:       db.Name(),
					IsInstalled:  local.Pkg(pkg.Name()) != nil,
					LastModified: int(pkg.BuildDate().Unix()),
					Popularity:   math.MaxFloat64,
				},
				Description:      pkg.Description(),
				NameLower:        strings.ToLower(pkg.Name()),
				DescriptionLower: strings.ToLower(pkg.Description()),
			})
		}
	}

	// Local DB (only those not already in sync or representing local modifications)
	for _, pkg := range local.PkgCache().Slice() {
		cache = append(cache, cachedPkg{
			Package: Package{
				Name:         pkg.Name(),
				Source:       local.Name(),
				IsInstalled:  true,
				LastModified: int(pkg.BuildDate().Unix()),
				Popularity:   math.MaxFloat64,
			},
			Description:      pkg.Description(),
			NameLower:        strings.ToLower(pkg.Name()),
			DescriptionLower: strings.ToLower(pkg.Description()),
		})
	}

	ui.pkgsCache = cache
}

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
	var reposPkgs []Package
	var localPkgs []Package

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

	uniqueMap := make(map[string]Package)
	for _, p := range allPkgs {
		existing, exists := uniqueMap[p.Name]
		if !exists || (!existing.IsInstalled && p.IsInstalled) {
			uniqueMap[p.Name] = p
		}
	}

	var resultList []Package
	for _, p := range uniqueMap {
		resultList = append(resultList, p)
	}

	sort.Slice(resultList, func(i, j int) bool {
		a, b := resultList[i], resultList[j]
		if a.IsInstalled != b.IsInstalled {
			return a.IsInstalled
		}
		aScore := getUnifiedScore(a, term)
		bScore := getUnifiedScore(b, term)
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

	aurPkgs, err := SearchAur(ctx, "", term, 5000, 2000)
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
		var reposPkgs []Package
		var localPkgs []Package

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

		uniqueMap := make(map[string]Package)
		for _, p := range allPkgs {
			existing, exists := uniqueMap[p.Name]
			if !exists || (!existing.IsInstalled && p.IsInstalled) {
				uniqueMap[p.Name] = p
			}
		}

		var resultList []Package
		for _, p := range uniqueMap {
			resultList = append(resultList, p)
		}

		sort.Slice(resultList, func(i, j int) bool {
			a, b := resultList[i], resultList[j]
			if a.IsInstalled != b.IsInstalled {
				return a.IsInstalled
			}
			aScore := getUnifiedScore(a, term)
			bScore := getUnifiedScore(b, term)
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
	ui.pkgTable.Clear()

	// Header row
	ui.pkgTable.SetCell(0, 0, tview.NewTableCell("Package").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetExpansion(1))
	ui.pkgTable.SetCell(0, 1, tview.NewTableCell("Source").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetMaxWidth(12))
	ui.pkgTable.SetCell(0, 2, tview.NewTableCell("Installed").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetMaxWidth(10))
	ui.pkgTable.SetCell(0, 3, tview.NewTableCell("Reputation").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetMaxWidth(12))

	for idx, p := range ui.shownPackages {
		installedStr := "No"
		installedCell := tview.NewTableCell(installedStr).SetMaxWidth(10)
		if p.IsInstalled {
			installedStr = "Yes"
			installedCell.SetText(installedStr).
				SetStyle(tcell.StyleDefault.Foreground(tcell.ColorGreen).Attributes(tcell.AttrBold))
		} else {
			installedCell.SetTextColor(tcell.ColorGray)
		}

		sourceColor := getSourceColor(p.Source)
		sourceCell := tview.NewTableCell(p.Source).SetTextColor(sourceColor).SetMaxWidth(12)

		pkgCell := tview.NewTableCell(p.Name).SetTextColor(tcell.ColorDefault).SetExpansion(1)

		reputationStr := ""
		if p.Source == "AUR" {
			reputationStr = strconv.Itoa(p.Votes)
		}
		reputationCell := tview.NewTableCell(reputationStr).SetTextColor(tcell.ColorDefault).SetMaxWidth(12)

		ui.pkgTable.SetCell(idx+1, 0, pkgCell)
		ui.pkgTable.SetCell(idx+1, 1, sourceCell)
		ui.pkgTable.SetCell(idx+1, 2, installedCell)
		ui.pkgTable.SetCell(idx+1, 3, reputationCell)
	}
	ui.pkgTable.ScrollToBeginning()
	ui.pkgTable.Select(1, 0)
}

func getSourceColor(source string) tcell.Color {
	switch strings.ToLower(source) {
	case "core":
		return tcell.ColorBlue
	case "extra":
		return tcell.ColorGreen
	case "multilib":
		return tcell.NewHexColor(0xff00ff)
	case "aur":
		return tcell.NewHexColor(0x00ffff)
	default:
		return tcell.ColorDefault
	}
}

func (ui *UI) loadPackageDetails(pkg Package) {
	ui.detailsView.Clear()
	ui.detailsView.SetTitle(fmt.Sprintf(" Details: %s ", pkg.Name))

	go func() {
		var info SearchResults
		if pkg.Source == "AUR" {
			info = InfoAur("", 5000, pkg.Name)
		} else {
			ui.alpmMutex.Lock()
			info = InfoPacman(ui.alpmHandle, pkg.Name)
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

func getUnifiedScore(p Package, term string) float64 {
	nameLower := strings.ToLower(p.Name)
	termLower := strings.ToLower(term)

	var matchScore float64
	if nameLower == termLower {
		matchScore = 1000000.0
	} else if strings.HasPrefix(nameLower, termLower) {
		matchScore = 30000.0
	} else if strings.Contains(nameLower, termLower) {
		matchScore = 10000.0
	} else {
		matchScore = 1000.0 // Description match
	}

	// Source trust bonus (+5,000 for official repositories)
	var sourceBonus float64
	if p.Source != "AUR" && p.Source != "local" {
		sourceBonus = 5000.0
	}

	// Reputation (AUR votes) heavily weighted
	reputation := float64(p.Votes) * 30.0

	// Name length tie-breaker (shorter names get a small bonus)
	nameLen := len(p.Name)
	if nameLen == 0 {
		nameLen = 1
	}
	lengthBonus := 100.0 / float64(nameLen)

	return matchScore + sourceBonus + reputation + lengthBonus
}

func (ui *UI) installOrUninstallPackage(pkg Package) {
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
			// Success! Auto-track in pkglist
			if isInstall {
				_ = pkglist.AddPackage(ui.conf.PackagesPath, pkg.Name)
				fmt.Printf("\033[1;32m[SUCCESS]\033[0m Package '%s' installed and added to drxboot.packages.\n", pkg.Name)
			} else {
				_ = pkglist.RemovePackage(ui.conf.PackagesPath, pkg.Name)
				fmt.Printf("\033[1;32m[SUCCESS]\033[0m Package '%s' uninstalled and removed from drxboot.packages.\n", pkg.Name)
			}
			fmt.Println("\nPress ENTER to return to drxpkg...")
			_, _ = os.Stdin.Read(make([]byte, 1))
		} else {
			fmt.Printf("\033[1;31m[ERROR]\033[0m Command failed: %v\nPress ENTER to return to drxpkg...", err)
			_, _ = os.Stdin.Read(make([]byte, 1))
		}
	})

	_ = ui.reinitPacmanDbs()
	if ui.lastSearchTerm != "" {
		ui.forceSearch(ui.lastSearchTerm)
	}
}

func (ui *UI) setStatus(msg string) {
	ui.statusText.SetText(msg)
}

func (ui *UI) setupSettingsPanel() {
	// Initialize inputs
	ui.settingInputs = make([]*tview.InputField, 7)
	ui.settingInputs[0] = tview.NewInputField().SetText(ui.conf.PackagesPath)
	ui.settingInputs[1] = tview.NewInputField().SetText(ui.conf.PacmanDBPath)
	ui.settingInputs[2] = tview.NewInputField().SetText(ui.conf.PacmanConfigPath)
	ui.settingInputs[3] = tview.NewInputField().SetText(ui.conf.InstallCommand)
	ui.settingInputs[4] = tview.NewInputField().SetText(ui.conf.UninstallCommand)
	ui.settingInputs[5] = tview.NewInputField().SetText(ui.conf.SysUpgradeCmd)
	ui.settingInputs[6] = tview.NewInputField().SetText(strconv.Itoa(ui.conf.MaxResults))

	for i, input := range ui.settingInputs {
		idx := i
		input.SetBorder(true).SetBorderColor(tcell.ColorGray)
		input.SetFieldBackgroundColor(tcell.ColorDefault)
		input.SetFieldTextColor(tcell.ColorDefault)

		input.SetFocusFunc(func() {
			ui.settingsFocusedIndex = idx
			ui.settingsEditMode = true
			ui.updateSettingsDisplay()
		})

		input.SetBlurFunc(func() {
			ui.settingsEditMode = false
			ui.updateSettingsDisplay()
		})

		input.SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter || key == tcell.KeyEscape {
				ui.settingsEditMode = false
				ui.app.SetFocus(ui.settingsGrid)
				ui.updateSettingsDisplay()
			}
		})
	}

	// Initialize checkbox
	ui.settingAurCb = tview.NewCheckbox().SetLabel("").SetChecked(ui.conf.DisableAur)
	ui.settingAurCb.SetFocusFunc(func() {
		ui.settingsFocusedIndex = 7
		ui.updateSettingsDisplay()
	})

	// Initialize buttons
	ui.btnSave = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	ui.btnDefaults = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)

	// Settings Box layout (centered inside grid)
	settingsBox := tview.NewFlex().SetDirection(tview.FlexRow)
	settingsBox.SetBorder(true).
		SetTitle(" Settings ").
		SetBorderColor(tcell.ColorBlue).
		SetTitleColor(tcell.ColorBlue)

	// Fields Grid: 7 inputs (height 3 each), 1 checkbox (height 1)
	fieldsGrid := tview.NewGrid().
		SetRows(3, 3, 3, 3, 3, 3, 3, 3).
		SetColumns(25, 0)

	labels := []string{
		"Packages Save Path",
		"Pacman DB Path",
		"Pacman Config Path",
		"Install Command",
		"Uninstall Command",
		"Upgrade Command",
		"Max Results",
		"Disable AUR",
	}

	for i, name := range labels {
		lblText := "  " + name
		if i < 7 {
			lblText = "\n" + lblText
		}
		lbl := tview.NewTextView().SetDynamicColors(true).SetText(lblText)
		lbl.SetTextColor(tcell.ColorDefault)

		if i < 7 {
			fieldsGrid.AddItem(lbl, i, 0, 1, 1, 0, 0, false)
			fieldsGrid.AddItem(ui.settingInputs[i], i, 1, 1, 1, 0, 0, false)
		} else {
			fieldsGrid.AddItem(lbl, i, 0, 1, 1, 0, 0, false)
			fieldsGrid.AddItem(ui.settingAurCb, i, 1, 1, 1, 0, 0, false)
		}
	}

	// Buttons Flex Row
	buttonsFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).
		AddItem(ui.btnSave, 18, 0, false).
		AddItem(nil, 4, 0, false).
		AddItem(ui.btnDefaults, 14, 0, false).
		AddItem(nil, 0, 1, false)

	settingsBox.
		AddItem(fieldsGrid, 0, 1, false).
		AddItem(nil, 1, 0, false). // spacer
		AddItem(buttonsFlex, 2, 0, false).
		AddItem(nil, 1, 0, false) // spacer

	// Center the settingsBox using a grid
	ui.settingsGrid = tview.NewGrid().
		SetRows(0, 28, 0).
		SetColumns(0, 75, 0).
		AddItem(settingsBox, 1, 1, 1, 1, 0, 0, true)

	// Set input capture on settingsGrid for navigation
	ui.settingsGrid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if ui.settingsEditMode {
			return event
		}

		switch event.Key() {
		case tcell.KeyUp:
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 9) % 10
			ui.updateSettingsDisplay()
			return nil
		case tcell.KeyDown:
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 1) % 10
			ui.updateSettingsDisplay()
			return nil
		case tcell.KeyLeft:
			if ui.settingsFocusedIndex == 9 {
				ui.settingsFocusedIndex = 8
				ui.updateSettingsDisplay()
				return nil
			}
		case tcell.KeyRight:
			if ui.settingsFocusedIndex == 8 {
				ui.settingsFocusedIndex = 9
				ui.updateSettingsDisplay()
				return nil
			}
		case tcell.KeyTAB:
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 1) % 10
			ui.updateSettingsDisplay()
			return nil
		case tcell.KeyBacktab:
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 9) % 10
			ui.updateSettingsDisplay()
			return nil
		case tcell.KeyEnter:
			ui.handleSettingsSelect()
			return nil
		}

		switch event.Rune() {
		case 'j', 'J':
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 1) % 10
			ui.updateSettingsDisplay()
			return nil
		case 'k', 'K':
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 9) % 10
			ui.updateSettingsDisplay()
			return nil
		case 'h', 'H':
			if ui.settingsFocusedIndex == 9 {
				ui.settingsFocusedIndex = 8
				ui.updateSettingsDisplay()
				return nil
			}
		case 'l', 'L':
			if ui.settingsFocusedIndex == 8 {
				ui.settingsFocusedIndex = 9
				ui.updateSettingsDisplay()
				return nil
			}
		case 'i', 'I':
			ui.handleSettingsSelect()
			return nil
		}

		return event
	})

	ui.updateSettingsDisplay()
}

func (ui *UI) updateSettingsDisplay() {
	// Reset all inputs border color
	for i, input := range ui.settingInputs {
		if i == ui.settingsFocusedIndex {
			if ui.settingsEditMode {
				input.SetBorderColor(tcell.ColorGreen)
			} else {
				input.SetBorderColor(tcell.ColorBlue)
			}
		} else {
			input.SetBorderColor(tcell.ColorGray)
		}
	}

	// Checkbox styling
	if ui.settingsFocusedIndex == 7 {
		ui.settingAurCb.SetFieldBackgroundColor(tcell.ColorYellow)
		ui.settingAurCb.SetFieldTextColor(tcell.ColorBlack)
	} else {
		ui.settingAurCb.SetFieldBackgroundColor(tcell.ColorDefault)
		ui.settingAurCb.SetFieldTextColor(tcell.ColorWhite)
	}

	// Save button styling
	if ui.settingsFocusedIndex == 8 {
		ui.btnSave.SetTextColor(tcell.ColorDefault)
		ui.btnSave.SetBackgroundColor(tcell.ColorBlue)
		ui.btnSave.SetText("Apply & Save")
	} else {
		ui.btnSave.SetTextColor(tcell.ColorWhite)
		ui.btnSave.SetBackgroundColor(tcell.ColorGray)
		ui.btnSave.SetText("Apply & Save")
	}

	// Defaults button styling
	if ui.settingsFocusedIndex == 9 {
		ui.btnDefaults.SetTextColor(tcell.ColorDefault)
		ui.btnDefaults.SetBackgroundColor(tcell.ColorBlue)
		ui.btnDefaults.SetText("Defaults")
	} else {
		ui.btnDefaults.SetTextColor(tcell.ColorWhite)
		ui.btnDefaults.SetBackgroundColor(tcell.ColorGray)
		ui.btnDefaults.SetText("Defaults")
	}
}

func (ui *UI) handleSettingsSelect() {
	if ui.settingsFocusedIndex >= 0 && ui.settingsFocusedIndex < 7 {
		ui.settingsEditMode = true
		ui.updateSettingsDisplay()
		ui.app.SetFocus(ui.settingInputs[ui.settingsFocusedIndex])
	} else if ui.settingsFocusedIndex == 7 {
		ui.settingAurCb.SetChecked(!ui.settingAurCb.IsChecked())
		ui.updateSettingsDisplay()
	} else if ui.settingsFocusedIndex == 8 {
		ui.saveSettingsAction()
	} else if ui.settingsFocusedIndex == 9 {
		ui.loadSettingsDefaults()
	}
}

func (ui *UI) saveSettingsAction() {
	ui.conf.PackagesPath = ui.settingInputs[0].GetText()
	ui.conf.PacmanDBPath = ui.settingInputs[1].GetText()
	ui.conf.PacmanConfigPath = ui.settingInputs[2].GetText()
	ui.conf.InstallCommand = ui.settingInputs[3].GetText()
	ui.conf.UninstallCommand = ui.settingInputs[4].GetText()
	ui.conf.SysUpgradeCmd = ui.settingInputs[5].GetText()

	maxRes, err := strconv.Atoi(ui.settingInputs[6].GetText())
	if err == nil {
		ui.conf.MaxResults = maxRes
	}

	ui.conf.DisableAur = ui.settingAurCb.IsChecked()

	if err := ui.conf.Save(); err != nil {
		ui.setStatus("Error saving settings: " + err.Error())
	} else {
		ui.setStatus("Settings saved successfully!")
	}
	_ = ui.reinitPacmanDbs()
}

func (ui *UI) loadSettingsDefaults() {
	ui.conf = config.Defaults()
	ui.settingInputs[0].SetText(ui.conf.PackagesPath)
	ui.settingInputs[1].SetText(ui.conf.PacmanDBPath)
	ui.settingInputs[2].SetText(ui.conf.PacmanConfigPath)
	ui.settingInputs[3].SetText(ui.conf.InstallCommand)
	ui.settingInputs[4].SetText(ui.conf.UninstallCommand)
	ui.settingInputs[5].SetText(ui.conf.SysUpgradeCmd)
	ui.settingInputs[6].SetText(strconv.Itoa(ui.conf.MaxResults))
	ui.settingAurCb.SetChecked(ui.conf.DisableAur)
	ui.setStatus("Settings reset to defaults!")
}
