// Package tui manages the Terminal User Interface, layouts, user input, and screen rendering.
package tui

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
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
	settingHooksCb       *tview.Checkbox
	btnSave              *tview.TextView
	btnDefaults          *tview.TextView
	settingsFocusedIndex int
	settingsEditMode     bool
	helpGrid             *tview.Grid
	settingsPopupOpen    bool
	helpPopupOpen        bool

	// Tab 1: Install Views
	searchField *tview.InputField
	pkgTable    *tview.Table
	detailsView *tview.TextView
	statusText  *tview.TextView

	// Tab 2: Update Views
	updatePageFlex    *tview.Flex
	updateTable       *tview.Table
	updateDetails     *tview.TextView
	updatePackages    []pkgmgr.UpdatePackage
	selectedUpdate    *pkgmgr.UpdatePackage
	updateDetailsCache map[string]string
	cacheMutex         sync.RWMutex

	// State
	activeTab      int
	lastSearchTerm string
	shownPackages  []pkgmgr.Package
	selectedPkg    *pkgmgr.Package

	// Fast Search Cache & Debouncing
	searchMutex  sync.Mutex
	searchCancel context.CancelFunc
	searchTimer  *time.Timer
	pkgsCache    []cachedPkg

	// Visual Mode State
	inVisualMode    bool
	visualStartRow  int
	visualEndRow    int
	selectedInstall map[string]bool
	isRendering     bool
	statusMessage            string
	confirmationFocusedIndex int
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
		selectedInstall:    make(map[string]bool),
	}

	var err error
	ui.alpmHandle, err = pkgmgr.InitPacmanDbs(conf.PacmanDBPath, conf.PacmanConfigPath)
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
	ui.alpmHandle, err = pkgmgr.InitPacmanDbs(ui.conf.PacmanDBPath, ui.conf.PacmanConfigPath)
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
		if ui.isRendering {
			return
		}
		if ui.inVisualMode {
			ui.visualEndRow = row
			ui.renderPackageTable()
		}
		if row <= 0 || row > len(ui.shownPackages) {
			ui.selectedPkg = nil
			if ui.detailsView != nil {
				ui.detailsView.Clear()
			}
			return
		}
		pkg := ui.shownPackages[row-1]
		ui.selectedPkg = &pkg
		ui.loadPackageDetails(pkg)
	})
	ui.setupUpdatePage()
	ui.setupSettingsPanel()
	ui.setupHelpPopup()
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

	// Tab 3: Package Management Page
	managePage := ui.setupManagePage()

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

func (ui *UI) drawTabBar() {
	tabs := []string{"[1] Install", "[2] Update", "[3] Manage"}
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
	if tabIndex < 0 || tabIndex > 2 {
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
	}
}

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
			if event.Rune() == '/' {
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
			if !ui.settingsPopupOpen && !ui.helpPopupOpen {
				ui.showSettingsPopup()
			}
			return nil
		}
		if event.Rune() == '[' {
			ui.switchTab((ui.activeTab + 2) % 3)
			return nil
		}
		if event.Rune() == ']' {
			ui.switchTab((ui.activeTab + 1) % 3)
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
		if event.Key() == tcell.KeyEscape {
			if ui.inVisualMode {
				ui.inVisualMode = false
				ui.updateStatusDisplay()
				ui.renderPackageTable()
				return nil
			}
		}
		if event.Key() == tcell.KeyRune && (event.Rune() == 'v' || event.Rune() == 'V') {
			ui.inVisualMode = !ui.inVisualMode
			if ui.inVisualMode {
				row, _ := ui.pkgTable.GetSelection()
				ui.visualStartRow = row
				ui.visualEndRow = row
			}
			ui.updateStatusDisplay()
			ui.renderPackageTable()
			return nil
		}
		if event.Key() == tcell.KeyRune && event.Rune() == ' ' {
			row, _ := ui.pkgTable.GetSelection()
			if row > 0 && row <= len(ui.shownPackages) {
				pkg := ui.shownPackages[row-1]
				ui.selectedInstall[pkg.Name] = !ui.selectedInstall[pkg.Name]
				ui.renderPackageTable()
				ui.pkgTable.Select(row, 0)
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

func (ui *UI) setStatus(msg string) {
	ui.statusMessage = msg
	ui.updateStatusDisplay()
}

func (ui *UI) updateStatusDisplay() {
	prefix := ""
	if ui.inVisualMode {
		prefix = "[yellow::b]SELECT MODE[-] "
	}
	ui.statusText.SetText(prefix + ui.statusMessage)
}

func getSourceColor(source string) tcell.Color {
	switch strings.ToLower(source) {
	case "core":
		return tcell.ColorRed
	case "extra":
		return tcell.ColorGreen
	case "multilib":
		return tcell.ColorYellow
	case "aur":
		return tcell.ColorBlue
	default:
		return tcell.ColorDefault
	}
}

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

func (ui *UI) getSelectedUpdatePackages() []string {
	var pkgs []string
	for _, p := range ui.updatePackages {
		if p.Selected && !p.NotInAur {
			pkgs = append(pkgs, p.Name)
		}
	}
	return pkgs
}



func (ui *UI) showSettingsPopup() {
	ui.settingsPopupOpen = true
	ui.settingInputs[0].SetText(ui.conf.PackagesPath)
	ui.settingInputs[1].SetText(ui.conf.PackagesFile)
	ui.settingInputs[2].SetText(ui.conf.PacmanDBPath)
	ui.settingInputs[3].SetText(ui.conf.PacmanConfigPath)
	ui.settingInputs[4].SetText(ui.conf.InstallCommand)
	ui.settingInputs[5].SetText(ui.conf.UninstallCommand)
	ui.settingInputs[6].SetText(ui.conf.SysUpgradeCmd)
	ui.settingInputs[7].SetText(strconv.Itoa(ui.conf.MaxResults))
	ui.settingAurCb.SetChecked(ui.conf.DisableAur)
	ui.settingHooksCb.SetChecked(ui.conf.RunUpdateHooks)
	ui.updateSettingsDisplay()

	ui.pages.ShowPage("settings")
	ui.app.SetFocus(ui.settingsGrid)
}

func (ui *UI) closeSettingsPopup() {
	ui.settingsPopupOpen = false
	ui.pages.HidePage("settings")
	ui.restoreFocusToActiveTab()
}

func (ui *UI) restoreFocusToActiveTab() {
	switch ui.activeTab {
	case 0:
		ui.app.SetFocus(ui.searchField)
	case 1:
		ui.app.SetFocus(ui.updateTable)
	case 2:
		// manage tab has no interactive input fields to focus.
	}
}
