// Package tui manages the Terminal User Interface, layouts, user input, and screen rendering.
package tui

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Jguer/go-alpm/v2"
	"github.com/druxorey/drxpkg/internal/config"
	"github.com/druxorey/drxpkg/internal/pkgmgr"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type cachedPkg struct {
	pkgmgr.Package
	Description      string
	NameLower        string
	DescriptionLower string
}

// Theme defines the visual color palette and styling configuration for the TUI components.
type Theme struct {
	PrimaryColor           tcell.Color // Main accent color used for focused borders, highlights, and headers
	UnfocusedBorderColor   tcell.Color // Color applied to primitive borders when they do not have keyboard focus
	FocusedBorderColor     tcell.Color // Color applied to primitive borders when the widget is actively focused
	EditingBorderColor     tcell.Color // Color used for active input fields when the user is in text editing mode
	NeutralGrayColor       tcell.Color // Used for secondary labels, inactive buttons, or disabled indicators
	SelectedTextColor      tcell.Color // Foreground color used for highlighted/selected text in lists and tables
	SettingsFieldFocusedBg tcell.Color // Background color of checkboxes/fields when focused in the settings menu
	SettingsFieldFocusedFg tcell.Color // Text color of checkboxes/fields when focused in the settings menu
}

// DefaultTheme represents the standard blue-accented dark theme of the application.
var DefaultTheme = Theme{
	PrimaryColor:           tcell.ColorBlue,
	UnfocusedBorderColor:   tcell.ColorDefault,
	FocusedBorderColor:     tcell.ColorBlue,
	EditingBorderColor:     tcell.ColorGreen,
	NeutralGrayColor:       tcell.ColorGray,
	SelectedTextColor:      tcell.ColorWhite,
	SettingsFieldFocusedBg: tcell.ColorYellow,
	SettingsFieldFocusedFg: tcell.ColorBlack,
}

// FocusBorderable defines an interface for primitives that change border color on focus/blur.
type FocusBorderable interface {
	SetFocusFunc(func()) *tview.Box
	SetBlurFunc(func()) *tview.Box
	SetBorderColor(tcell.Color) *tview.Box
}

type UI struct {
	theme                    Theme
	conf                     *config.Settings
	app                      *tview.Application
	alpmHandle               *alpm.Handle
	alpmMutex                sync.Mutex

	// Main Layout
	grid                     *tview.Grid
	tabBar                   *tview.TextView
	pages                    *tview.Pages
	settingsGrid             *tview.Grid
	settingInputs            []*tview.InputField
	settingAurCb             *tview.Checkbox
	settingHooksCb           *tview.Checkbox
	btnSave                  *tview.TextView
	btnDefaults              *tview.TextView
	settingsFocusedIndex     int
	settingsEditMode         bool
	helpGrid                 *tview.Grid
	settingsPopupOpen        bool
	helpPopupOpen            bool

	// Tab 1: Install Views
	searchField              *tview.InputField
	pkgTable                 *tview.Table
	detailsView              *tview.TextView
	statusText               *tview.TextView
	installLeftFlex          *tview.Flex
	installRightFlex         *tview.Flex
	selectedTable            *tview.Table
	installDetailsCache      map[string]string
	ignoreSearchChange       bool

	// Tab 2: Update Views
	updatePageFlex           *tview.Flex
	updateTable              *tview.Table
	updateDetails            *tview.TextView
	updatePackages           []pkgmgr.UpdatePackage
	selectedUpdate           *pkgmgr.UpdatePackage
	updateDetailsCache       map[string]string
	cacheMutex               sync.RWMutex

	// Tab 3: Maintenance Views
	manageTable              *tview.Table
	manageDetails            *tview.TextView
	manageFlex               *tview.Flex
	managePages              *tview.Pages
	trashTable               *tview.Table
	trashFiles               []trashFile
	cacheTable               *tview.Table
	cacheOptions             []cacheOption
	logsTable                *tview.Table
	logOptions               []logOption
	maintenanceItems         []maintenanceMenuItem

	// System detection
	isCachyOS                bool
	aurHelper                string

	// State
	activeTab                int
	lastSearchTerm           string
	shownPackages            []pkgmgr.Package
	selectedPkg              *pkgmgr.Package

	// Fast Search Cache & Debouncing
	searchMutex              sync.Mutex
	searchCancel             context.CancelFunc
	searchTimer              *time.Timer
	pkgsCache                []cachedPkg
	aurPkgsMutex             sync.RWMutex
	aurPkgsCache             []string
	aurCacheLoading          bool
	aurCacheLoaded           bool

	// Visual Mode State
	inVisualMode             bool
	visualStartRow           int
	visualEndRow             int
	selectedInstall          map[string]bool
	isRendering              bool
	statusMessage            string
	confirmationFocusedIndex int
	lastKeyWasG              bool
}

// New initializes the UI with the given configuration and sets up the layout and event handlers
func New(conf *config.Settings) (*UI, error) {
	ui := &UI{
		theme:               DefaultTheme,
		conf:                conf,
		app:                 tview.NewApplication(),
		activeTab:           tabInstall,
		updateDetailsCache:  make(map[string]string),
		installDetailsCache: make(map[string]string),
		selectedInstall:     make(map[string]bool),
	}

	ui.detectSystemConfig()

	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDefault
	tview.Styles.ContrastBackgroundColor = tcell.ColorDefault
	tview.Styles.MoreContrastBackgroundColor = tcell.ColorDefault
	tview.Styles.BorderColor = ui.theme.PrimaryColor
	tview.Styles.TitleColor = ui.theme.PrimaryColor
	tview.Styles.GraphicsColor = ui.theme.PrimaryColor
	tview.Styles.PrimaryTextColor = tcell.ColorDefault
	tview.Styles.SecondaryTextColor = tcell.ColorDefault
	tview.Styles.TertiaryTextColor = tcell.ColorDefault

	var err error
	ui.alpmHandle, err = pkgmgr.InitPacmanDbs()
	if err != nil {
		return nil, fmt.Errorf("failed to init pacman db: %w", err)
	}

	ui.setupWidgets()
	ui.setupLayout()
	ui.setupKeyboard()

	ui.rebuildCache()
	ui.backgroundUpdateCheck()
	ui.loadAurPackagesCache()

	return ui, nil
}

// detectSystemConfig probes the filesystem for OS release info and available AUR helpers
func (ui *UI) detectSystemConfig() {
	ui.isCachyOS = false
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for line := range strings.Lines(string(data)) {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "ID=") {
				id := strings.Trim(line[3:], `"'`)
				if id == "cachyos" {
					ui.isCachyOS = true
					break
				}
			}
			if strings.HasPrefix(line, "ID_LIKE=") {
				like := strings.Trim(line[8:], `"'`)
				if strings.Contains(like, "cachyos") {
					ui.isCachyOS = true
					break
				}
			}
		}
	}

	ui.aurHelper = ""
	configCmds := strings.ToLower(ui.conf.InstallCommand + " " + ui.conf.UninstallCommand + " " + ui.conf.SysUpgradeCmd)
	if strings.Contains(configCmds, "paru") {
		ui.aurHelper = "paru"
	} else if strings.Contains(configCmds, "yay") {
		ui.aurHelper = "yay"
	} else {
		if _, err := exec.LookPath("paru"); err == nil {
			ui.aurHelper = "paru"
		} else if _, err := exec.LookPath("yay"); err == nil {
			ui.aurHelper = "yay"
		}
	}
}

// Start initializes the TUI layout and enters the application main loop
func (ui *UI) Start() error {
	ui.drawTabBar()
	ui.app.SetRoot(ui.grid, true).EnableMouse(true)
	return ui.app.Run()
}

// reinitPacmanDbs releases the current alpm handle and reloads the pacman databases
func (ui *UI) reinitPacmanDbs() error {
	ui.alpmMutex.Lock()
	if ui.alpmHandle != nil {
		_ = ui.alpmHandle.Release()
	}
	var err error
	ui.alpmHandle, err = pkgmgr.InitPacmanDbs()
	ui.alpmMutex.Unlock()

	if err == nil {
		ui.rebuildCache()
	}
	return err
}

// rebuildCache refreshes the local and sync package databases and updates the search cache
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
				Package: pkgmgr.Package{
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
			Package: pkgmgr.Package{
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

// setupWidgets initializes all TUI components for the interface
func (ui *UI) setupWidgets() {
	// Tab bar
	ui.tabBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	// Tab 1 Widgets
	ui.searchField = tview.NewInputField().
		SetLabel(" > ").
		SetLabelColor(ui.theme.PrimaryColor).
		SetFieldTextColor(tcell.ColorDefault).
		SetFieldBackgroundColor(tcell.ColorDefault)
	ui.searchField.SetBorder(false)
	ui.searchField.SetChangedFunc(func(text string) {
		ui.handleSearchChange(text)
	})
	ui.searchField.SetFocusFunc(func() {
		if ui.installLeftFlex != nil {
			ui.installLeftFlex.SetBorderColor(ui.theme.FocusedBorderColor)
		}
	})
	ui.searchField.SetBlurFunc(func() {
		if ui.installLeftFlex != nil {
			ui.installLeftFlex.SetBorderColor(ui.theme.UnfocusedBorderColor)
		}
	})

	ui.pkgTable = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	ui.pkgTable.SetSelectedStyle(tcell.StyleDefault.Background(ui.theme.PrimaryColor).Foreground(ui.theme.SelectedTextColor).Attributes(tcell.AttrBold))
	ui.pkgTable.SetBorder(false)
	ui.pkgTable.SetFocusFunc(func() {
		ui.app.SetFocus(ui.searchField)
	})

	ui.detailsView = ui.createStandardTextView(" Details ", true)

	ui.selectedTable = ui.createStandardTable(" Selected Packages (0) ", 1, 0)

	ui.statusText = tview.NewTextView().
		SetDynamicColors(true)

	// Tab 1 Packages Selection Change
	ui.pkgTable.SetSelectionChangedFunc(func(row, column int) {
		if ui.isRendering {
			return
		}
		if row <= 0 || row > len(ui.shownPackages) {
			ui.selectedPkg = nil
			if ui.detailsView != nil {
				ui.detailsView.Clear()
			}
			return
		}
		ui.updateTableFuzzyHighlights(row)
		pkg := ui.shownPackages[row-1]
		ui.selectedPkg = &pkg
		ui.loadPackageDetails(pkg)
	})
	ui.setupUpdatePage()
	ui.setupSettingsPopup()
	ui.setupHelpPopup()
}

// setupLayout arranges the main grid and organizes pages for the TUI application
func (ui *UI) setupLayout() {
	ui.pages = tview.NewPages()

	// Tab 1: Installation Page
	ui.installLeftFlex = tview.NewFlex().SetDirection(tview.FlexRow)
	ui.installLeftFlex.SetBorder(true).
		SetTitle(" Search ").
		SetBorderColor(ui.theme.UnfocusedBorderColor)
	ui.installLeftFlex.SetBorderPadding(1, 1, 2, 2)

	separator := tview.NewBox().SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		color := ui.theme.NeutralGrayColor
		style := tcell.StyleDefault.Foreground(color)
		for i := x + 2; i < x+width-2; i++ {
			screen.SetContent(i, y, '─', nil, style)
		}
		return x, y, width, height
	})

	ui.installLeftFlex.
		AddItem(ui.searchField, 1, 0, true).
		AddItem(separator, 1, 0, false).
		AddItem(ui.pkgTable, 0, 1, false)

	ui.installRightFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.detailsView, 0, 1, false)

	installPage := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ui.installLeftFlex, 0, 1, true).
		AddItem(ui.installRightFlex, 0, 1, false)

	// Tab 3: Package Maintenance Section
	managePage := ui.setupMaintenanceSection()

	// Tab Pages
	ui.pages.AddPage("install", installPage, true, true)
	ui.pages.AddPage("update", ui.updatePageFlex, true, false)
	ui.pages.AddPage("manage", managePage, true, false)
	ui.pages.AddPage("settings", ui.settingsGrid, true, false)
	ui.pages.AddPage("help", ui.helpGrid, true, false)

	// Main Grid Layout
	ui.grid = tview.NewGrid().
		SetRows(1, 0, 1).
		SetColumns(0).
		AddItem(ui.tabBar, 0, 0, 1, 1, 0, 0, false).
		AddItem(ui.pages, 1, 0, 1, 1, 0, 0, true).
		AddItem(ui.statusText, 2, 0, 1, 1, 0, 0, false)
}

// drawTabBar renders the tab navigation bar with the current active tab highlighted
func (ui *UI) drawTabBar() {
	tabs := []string{"[1] Install", "[2] Update", "[3] Maintenance"}
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

// switchTab transitions between the main application modules and updates the focus
func (ui *UI) switchTab(tabIndex int) {
	if tabIndex < 0 || tabIndex >= tabCount {
		return
	}
	ui.activeTab = tabIndex
	ui.drawTabBar()

	switch tabIndex {
	case tabInstall:
		ui.pages.SwitchToPage("install")
		ui.app.SetFocus(ui.searchField)
	case tabUpdate:
		ui.pages.SwitchToPage("update")
		ui.app.SetFocus(ui.updateTable)
		ui.checkForUpdates()
	case tabManage:
		ui.pages.SwitchToPage("manage")
		ui.app.SetFocus(ui.manageTable)
	}
}

// restoreFocusToActiveTab returns the input focus to the active tab's primary widget
func (ui *UI) restoreFocusToActiveTab() {
	switch ui.activeTab {
	case tabInstall:
		ui.app.SetFocus(ui.searchField)
	case tabUpdate:
		ui.app.SetFocus(ui.updateTable)
	case tabManage:
		ui.app.SetFocus(ui.manageTable)
	}
}

// setupKeyboard configures global and component-specific keyboard input handlers
func (ui *UI) setupKeyboard() {
	// Global captures
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if name, item := ui.pages.GetFrontPage(); name == "confirmation" {
			if event.Key() == tcell.KeyEscape {
				return event
			}
			if modal, ok := item.(*tview.Modal); ok {
				switch event.Key() {
				case tcell.KeyLeft, tcell.KeyBacktab:
					if ui.confirmationFocusedIndex == 0 {
						ui.confirmationFocusedIndex = 1
					} else {
						ui.confirmationFocusedIndex = 0
					}
					modal.SetFocus(ui.confirmationFocusedIndex)
					ui.app.SetFocus(modal)
					return nil
				case tcell.KeyRight, tcell.KeyTab:
					if ui.confirmationFocusedIndex == 1 {
						ui.confirmationFocusedIndex = 0
					} else {
						ui.confirmationFocusedIndex = 1
					}
					modal.SetFocus(ui.confirmationFocusedIndex)
					ui.app.SetFocus(modal)
					return nil
				}

				switch event.Rune() {
				case 'h', 'H', 'k', 'K':
					if ui.confirmationFocusedIndex == 0 {
						ui.confirmationFocusedIndex = 1
					} else {
						ui.confirmationFocusedIndex = 0
					}
					modal.SetFocus(ui.confirmationFocusedIndex)
					ui.app.SetFocus(modal)
					return nil
				case 'l', 'L', 'j', 'J':
					if ui.confirmationFocusedIndex == 1 {
						ui.confirmationFocusedIndex = 0
					} else {
						ui.confirmationFocusedIndex = 1
					}
					modal.SetFocus(ui.confirmationFocusedIndex)
					ui.app.SetFocus(modal)
					return nil
				}
			}
		}

		if event.Key() == tcell.KeyEscape {
			if name, _ := ui.pages.GetFrontPage(); name == "confirmation" {
				return event
			}
			if ui.helpPopupOpen {
				ui.closeHelpPopup()
				return nil
			}
			if ui.settingsPopupOpen {
				if ui.settingsEditMode {
					return event
				}
				ui.closeSettingsPopup()
				return nil
			}
			ui.showConfirmation("Are you sure you want to exit?", func() {
				ui.app.Stop()
			})
			return nil
		}

		hasModal := false
		if name, _ := ui.pages.GetFrontPage(); name == "confirmation" {
			hasModal = true
		}

		if !ui.settingsPopupOpen && !ui.helpPopupOpen && !hasModal {
			if event.Rune() == '.' {
				ui.showSettingsPopup()
				return nil
			}
			if event.Rune() == '?' {
				ui.showHelpPopup()
				return nil
			}
		}

		// F-keys or Alt/Ctrl numbers to switch tabs
		if event.Key() == tcell.KeyF1 {
			ui.switchTab(tabInstall)
			return nil
		}
		if event.Key() == tcell.KeyF2 {
			ui.switchTab(tabUpdate)
			return nil
		}
		if event.Key() == tcell.KeyF3 {
			ui.switchTab(tabManage)
			return nil
		}
		if event.Key() == tcell.KeyF4 {
			if !ui.settingsPopupOpen && !ui.helpPopupOpen {
				ui.showSettingsPopup()
			}
			return nil
		}
		if event.Rune() == '[' {
			ui.switchTab((ui.activeTab + tabCount - 1) % tabCount)
			return nil
		}
		if event.Rune() == ']' {
			ui.switchTab((ui.activeTab + 1) % tabCount)
			return nil
		}

		return event
	})

	// Search input captured
	ui.searchField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			ui.app.SetFocus(ui.detailsView)
			return nil
		case tcell.KeyBacktab:
			ui.app.SetFocus(ui.detailsView)
			return nil
		case tcell.KeyDown:
			ui.moveSelectionDown()
			return nil
		case tcell.KeyUp:
			ui.moveSelectionUp()
			return nil
		case tcell.KeyCtrlN:
			ui.moveSelectionDown()
			return nil
		case tcell.KeyCtrlP:
			ui.moveSelectionUp()
			return nil
		case tcell.KeyEnter:
			term := strings.TrimSpace(ui.searchField.GetText())
			if term != "" {
				ui.attemptInstallExact(term)
			}
			return nil
		}
		return event
	})

	// Table list captured
	ui.pkgTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if ui.handleTableVimNavigation(event, ui.pkgTable, len(ui.shownPackages)) {
			return nil
		}

		if event.Key() == tcell.KeyTAB {
			ui.app.SetFocus(ui.detailsView)
			return nil
		}
		if event.Key() == tcell.KeyBacktab {
			ui.app.SetFocus(ui.searchField)
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			if ui.inVisualMode {
				ui.inVisualMode = false
				ui.updateStatusDisplay()
				ui.renderPackageTable()
				return nil
			}
		}

		if ui.handleVisualModeToggle(event, ui.pkgTable, ui.renderPackageTable) {
			return nil
		}

		if event.Key() == tcell.KeyRune && event.Rune() == ' ' {
			if ui.inVisualMode {
				minRow := min(ui.visualStartRow, ui.visualEndRow)
				maxRow := max(ui.visualStartRow, ui.visualEndRow)
				for r := minRow; r <= maxRow; r++ {
					if r > 0 && r <= len(ui.shownPackages) {
						name := ui.shownPackages[r-1].Name
						ui.selectedInstall[name] = !ui.selectedInstall[name]
					}
				}
				ui.inVisualMode = false
				ui.updateStatusDisplay()
				ui.renderPackageTable()
			} else {
				row, _ := ui.pkgTable.GetSelection()
				if row > 0 && row <= len(ui.shownPackages) {
					pkg := ui.shownPackages[row-1]
					ui.selectedInstall[pkg.Name] = !ui.selectedInstall[pkg.Name]
					ui.renderPackageTable()
					ui.pkgTable.Select(row, 0)
				}
			}
			return nil
		}
		if event.Key() == tcell.KeyEnter {
			if ui.inVisualMode {
				minRow := min(ui.visualStartRow, ui.visualEndRow)
				maxRow := max(ui.visualStartRow, ui.visualEndRow)
				for r := minRow; r <= maxRow; r++ {
					if r > 0 && r <= len(ui.shownPackages) {
						name := ui.shownPackages[r-1].Name
						ui.selectedInstall[name] = !ui.selectedInstall[name]
					}
				}
				ui.inVisualMode = false
				ui.updateStatusDisplay()
				ui.renderPackageTable()
			}
			return nil
		}
		if event.Key() == tcell.KeyRune && (event.Rune() == 'u' || event.Rune() == 'U') {
			selectedPkgs := ui.getSelectedInstallPackages()
			if len(selectedPkgs) > 0 {
				ui.promptUninstall(strings.Join(selectedPkgs, " "))
			} else if ui.selectedPkg != nil {
				ui.promptUninstall(ui.selectedPkg.Name)
			}
			return nil
		}
		if event.Key() == tcell.KeyRune && (event.Rune() == 'i' || event.Rune() == 'I') {
			selectedPkgs := ui.getSelectedInstallPackages()
			if len(selectedPkgs) > 0 {
				ui.promptInstall(strings.Join(selectedPkgs, " "))
			} else if ui.selectedPkg != nil {
				ui.promptInstall(ui.selectedPkg.Name)
			}
			return nil
		}
		return event
	})

	ui.selectedTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB {
			ui.app.SetFocus(ui.searchField)
			return nil
		}
		if event.Key() == tcell.KeyBacktab {
			ui.app.SetFocus(ui.detailsView)
			return nil
		}
		if (event.Key() == tcell.KeyRune && event.Rune() == ' ') || event.Key() == tcell.KeyEnter {
			row, _ := ui.selectedTable.GetSelection()
			selectedPkgs := ui.getSelectedInstallPackages()
			if row >= 0 && row < len(selectedPkgs) {
				pkgName := selectedPkgs[row]
				ui.selectedInstall[pkgName] = false

				// Re-render package table to reflect the deselection
				ui.renderPackageTable()

				// If we still have packages left, update rendering and keep focus
				selectedAfter := ui.getSelectedInstallPackages()
				if len(selectedAfter) > 0 {
					ui.renderSelectedTable(selectedAfter)
					if row >= len(selectedAfter) {
						ui.selectedTable.Select(len(selectedAfter)-1, 0)
					} else {
						ui.selectedTable.Select(row, 0)
					}
				}
			}
			return nil
		}
		return event
	})

	ui.detailsView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB {
			if ui.installRightFlex != nil && ui.installRightFlex.GetItemCount() == 2 {
				ui.app.SetFocus(ui.selectedTable)
			} else {
				ui.app.SetFocus(ui.searchField)
			}
			return nil
		}
		if event.Key() == tcell.KeyBacktab {
			ui.app.SetFocus(ui.pkgTable)
			return nil
		}
		return event
	})
}

// setStatus updates the current status message and refreshes the status bar display
func (ui *UI) setStatus(msg string) {
	ui.statusMessage = msg
	ui.updateStatusDisplay()
}

// updateStatusDisplay refreshes the status bar text based on current mode and messages
func (ui *UI) updateStatusDisplay() {
	prefix := ""
	if ui.inVisualMode {
		prefix = "[yellow::b]SELECT MODE[-] "
	}
	ui.statusText.SetText(prefix + ui.statusMessage)
}

// getSourceColor maps package repository names to corresponding display colors
func getSourceColor(source string) tcell.Color {
	switch strings.ToLower(source) {
	case "core":
		return tcell.ColorRed
	case "cachyos-core-v3":
		return tcell.ColorRed
	case "cachyos-core-v4":
		return tcell.ColorRed
	case "extra":
		return tcell.ColorLime
	case "cachyos-extra-v3":
		return tcell.ColorLime
	case "cachyos-extra-v4":
		return tcell.ColorLime
	case "multilib":
		return tcell.ColorYellow
	case "cachyos":
		return tcell.ColorPurple
	case "cachyos-v3":
		return tcell.ColorPurple
	case "cachyos-v4":
		return tcell.ColorPurple
	case "aur":
		return tcell.ColorTeal
	default:
		return tcell.ColorDefault
	}
}

// getSelectedInstallPackages retrieves a sorted list of names for all currently selected packages
func (ui *UI) getSelectedInstallPackages() []string {
	var pkgs []string
	for name, selected := range ui.selectedInstall {
		if selected {
			pkgs = append(pkgs, name)
		}
	}
	sort.Strings(pkgs)
	return pkgs
}

// handleTableVimNavigation processes Vim-like 'gg' top and 'G' bottom scrolling for tables
func (ui *UI) handleTableVimNavigation(event *tcell.EventKey, table *tview.Table, numItems int) bool {
	if event.Key() == tcell.KeyRune {
		r := event.Rune()
		switch r {
		case 'g':
			if ui.lastKeyWasG {
				if numItems > 0 {
					table.ScrollToBeginning()
					table.Select(1, 0)
				}
				ui.lastKeyWasG = false
			} else {
				ui.lastKeyWasG = true
			}
			return true
		case 'G':
			if numItems > 0 {
				table.ScrollToEnd()
				table.Select(numItems, 0)
			}
			ui.lastKeyWasG = false
			return true
		}
	}
	ui.lastKeyWasG = false
	return false
}

// handleVisualModeToggle toggles visual range selection mode and initializes the starting row
func (ui *UI) handleVisualModeToggle(event *tcell.EventKey, table *tview.Table, renderFunc func()) bool {
	if event.Key() == tcell.KeyRune && (event.Rune() == 'v' || event.Rune() == 'V') {
		ui.inVisualMode = !ui.inVisualMode
		if ui.inVisualMode {
			row, _ := table.GetSelection()
			ui.visualStartRow = row
			ui.visualEndRow = row
		}
		ui.updateStatusDisplay()
		renderFunc()
		return true
	}
	return false
}

// loadAurPackagesCache runs yay -Slqa (or helper -Slqa) in the background to load AUR packages cache.
func (ui *UI) loadAurPackagesCache() {
	if ui.conf.DisableAur {
		return
	}

	ui.aurPkgsMutex.Lock()
	ui.aurCacheLoading = true
	ui.aurPkgsMutex.Unlock()

	ui.setStatus("Loading AUR cache...")

	go func() {
		helper := ui.aurHelper
		if helper == "" {
			helper = "yay"
		}

		cmd := exec.Command(helper, "-Slqa")
		out, err := cmd.Output()
		if err != nil {
			ui.aurPkgsMutex.Lock()
			ui.aurCacheLoading = false
			ui.aurPkgsMutex.Unlock()
			ui.app.QueueUpdateDraw(func() {
				ui.setStatus(fmt.Sprintf("Failed to load AUR cache: %v", err))
			})
			return
		}

		var aurPkgs []string
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) == 0 {
				continue
			}
			var pkgName string
			if len(parts) >= 2 {
				repo := strings.ToLower(parts[0])
				if repo == "aur" {
					pkgName = parts[1]
				} else {
					continue
				}
			} else {
				pkgName = parts[0]
			}
			aurPkgs = append(aurPkgs, pkgName)
		}

		ui.aurPkgsMutex.Lock()
		ui.aurPkgsCache = aurPkgs
		ui.aurCacheLoaded = true
		ui.aurCacheLoading = false
		ui.aurPkgsMutex.Unlock()

		ui.app.QueueUpdateDraw(func() {
			ui.setStatus("AUR cache loaded successfully.")
			if ui.lastSearchTerm != "" {
				ui.performLocalSearch(ui.lastSearchTerm)
			}
		})
	}()
}

