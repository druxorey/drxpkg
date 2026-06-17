package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Jguer/go-alpm/v2"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/druxorey/drxpkg/internal/config"
	"github.com/druxorey/drxpkg/internal/pkglist"
)

type UI struct {
	conf           *config.Settings
	app            *tview.Application
	alpmHandle     *alpm.Handle
	alpmMutex      sync.Mutex

	// Main Layout
	grid           *tview.Grid
	tabBar         *tview.TextView
	pages          *tview.Pages
	formSettings   *tview.Form

	// Tab 1: Install Views
	searchField    *tview.InputField
	pkgTable       *tview.Table
	detailsView    *tview.TextView
	statusText     *tview.TextView

	// State
	activeTab      int
	lastSearchTerm string
	shownPackages  []Package
	selectedPkg    *Package
	searching      bool
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
		conf:      conf,
		app:       tview.NewApplication(),
		activeTab: 0,
	}

	var err error
	ui.alpmHandle, err = InitPacmanDbs(conf.PacmanDbPath, conf.PacmanConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to init pacman db: %w", err)
	}

	ui.setupWidgets()
	ui.setupLayout()
	ui.setupKeyboard()

	return ui, nil
}

func (ui *UI) Start() error {
	ui.drawTabBar()
	ui.app.SetRoot(ui.grid, true).EnableMouse(true)
	return ui.app.Run()
}

func (ui *UI) reinitPacmanDbs() error {
	ui.alpmMutex.Lock()
	defer ui.alpmMutex.Unlock()
	if ui.alpmHandle != nil {
		_ = ui.alpmHandle.Release()
	}
	var err error
	ui.alpmHandle, err = InitPacmanDbs(ui.conf.PacmanDbPath, ui.conf.PacmanConfigPath)
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

	// Tab 2: System Update Placeholder
	updatePage := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("\n\n[blue]System Update[-]\n\nThis page is currently a placeholder.")

	// Tab 3: Package Management Placeholder
	managePage := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("\n\n[blue]Package Management[-]\n\nThis page is currently a placeholder.")

	// Tab 4: Settings Page
	ui.formSettings = tview.NewForm()
	ui.formSettings.SetBorder(true).SetTitle(" Settings ")
	ui.formSettings.SetLabelColor(tcell.ColorBlue).
		SetFieldTextColor(tcell.ColorWhite).
		SetFieldBackgroundColor(tcell.ColorDefault)

	ui.formSettings.AddInputField("Packages Save Path", ui.conf.PackagesPath, 50, nil, nil)
	ui.formSettings.AddInputField("Pacman DB Path", ui.conf.PacmanDbPath, 50, nil, nil)
	ui.formSettings.AddInputField("Pacman Config Path", ui.conf.PacmanConfigPath, 50, nil, nil)
	ui.formSettings.AddInputField("Install Command", ui.conf.InstallCommand, 50, nil, nil)
	ui.formSettings.AddInputField("Uninstall Command", ui.conf.UninstallCommand, 50, nil, nil)
	ui.formSettings.AddInputField("Upgrade Command", ui.conf.SysUpgradeCmd, 50, nil, nil)
	ui.formSettings.AddInputField("Max Results", strconv.Itoa(ui.conf.MaxResults), 10, nil, nil)
	ui.formSettings.AddCheckbox("Disable AUR", ui.conf.DisableAur, nil)

	ui.formSettings.AddButton("Apply & Save", func() {
		ui.conf.PackagesPath = ui.formSettings.GetFormItem(0).(*tview.InputField).GetText()
		ui.conf.PacmanDbPath = ui.formSettings.GetFormItem(1).(*tview.InputField).GetText()
		ui.conf.PacmanConfigPath = ui.formSettings.GetFormItem(2).(*tview.InputField).GetText()
		ui.conf.InstallCommand = ui.formSettings.GetFormItem(3).(*tview.InputField).GetText()
		ui.conf.UninstallCommand = ui.formSettings.GetFormItem(4).(*tview.InputField).GetText()
		ui.conf.SysUpgradeCmd = ui.formSettings.GetFormItem(5).(*tview.InputField).GetText()
		
		maxRes, err := strconv.Atoi(ui.formSettings.GetFormItem(6).(*tview.InputField).GetText())
		if err == nil {
			ui.conf.MaxResults = maxRes
		}
		
		ui.conf.DisableAur = ui.formSettings.GetFormItem(7).(*tview.Checkbox).IsChecked()

		if err := ui.conf.Save(); err != nil {
			ui.setStatus("Error saving settings: " + err.Error())
		} else {
			ui.setStatus("Settings saved successfully!")
		}
		_ = ui.reinitPacmanDbs()
	})

	ui.formSettings.AddButton("Defaults", func() {
		ui.conf = config.Defaults()
		ui.formSettings.GetFormItem(0).(*tview.InputField).SetText(ui.conf.PackagesPath)
		ui.formSettings.GetFormItem(1).(*tview.InputField).SetText(ui.conf.PacmanDbPath)
		ui.formSettings.GetFormItem(2).(*tview.InputField).SetText(ui.conf.PacmanConfigPath)
		ui.formSettings.GetFormItem(3).(*tview.InputField).SetText(ui.conf.InstallCommand)
		ui.formSettings.GetFormItem(4).(*tview.InputField).SetText(ui.conf.UninstallCommand)
		ui.formSettings.GetFormItem(5).(*tview.InputField).SetText(ui.conf.SysUpgradeCmd)
		ui.formSettings.GetFormItem(6).(*tview.InputField).SetText(strconv.Itoa(ui.conf.MaxResults))
		ui.formSettings.GetFormItem(7).(*tview.Checkbox).SetChecked(ui.conf.DisableAur)
	})

	ui.pages.AddPage("install", installPage, true, true)
	ui.pages.AddPage("update", updatePage, true, false)
	ui.pages.AddPage("manage", managePage, true, false)
	ui.pages.AddPage("settings", ui.formSettings, true, false)

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
	case 2:
		ui.pages.SwitchToPage("manage")
	case 3:
		ui.pages.SwitchToPage("settings")
		ui.app.SetFocus(ui.formSettings)
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
		if key == tcell.KeyEnter {
			term := strings.TrimSpace(ui.searchField.GetText())
			if term != "" {
				ui.performSearch(term)
			}
		} else if key == tcell.KeyTAB || key == tcell.KeyDown {
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

func (ui *UI) performSearch(term string) {
	if ui.searching {
		return
	}
	ui.searching = true
	ui.lastSearchTerm = term
	ui.setStatus("Searching...")

	go func() {
		// ALPM repository search
		ui.alpmMutex.Lock()
		reposPkgs, localPkgs, err := SearchRepos(ui.alpmHandle, term, ui.conf.MaxResults)
		ui.alpmMutex.Unlock()

		if err != nil {
			ui.app.QueueUpdateDraw(func() {
				ui.setStatus("[red]Search error: " + err.Error())
				ui.searching = false
			})
			return
		}

		// AUR search
		var aurPkgs []Package
		if !ui.conf.DisableAur {
			aurPkgs, _ = SearchAur("", term, 5000, ui.conf.MaxResults)
			// Deduplicate AUR search list with local DB
			for idx := range aurPkgs {
				ui.alpmMutex.Lock()
				aurPkgs[idx].IsInstalled = IsPackageInstalled(ui.alpmHandle, aurPkgs[idx].Name)
				ui.alpmMutex.Unlock()
			}
		}

		// Merge
		allPkgs := append(reposPkgs, localPkgs...)
		allPkgs = append(allPkgs, aurPkgs...)

		// Deduplicate and filter results
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

		// Sort packages by search term relevance
		sort.Slice(resultList, func(i, j int) bool {
			a, b := resultList[i], resultList[j]
			termLower := strings.ToLower(term)
			aNameLower := strings.ToLower(a.Name)
			bNameLower := strings.ToLower(b.Name)

			// 1. AUR-specific blended reputation sorting
			if a.Source == "AUR" && b.Source == "AUR" {
				aScore := getAurScore(a, term)
				bScore := getAurScore(b, term)
				if aScore != bScore {
					return aScore > bScore
				}
			}

			// 2. Exact match
			aExact := aNameLower == termLower
			bExact := bNameLower == termLower
			if aExact != bExact {
				return aExact
			}

			// 3. Starts with search term
			aStarts := strings.HasPrefix(aNameLower, termLower)
			bStarts := strings.HasPrefix(bNameLower, termLower)
			if aStarts != bStarts {
				return aStarts
			}

			// 4. Official repository priority (anything not AUR/local is official)
			aOfficial := a.Source != "AUR" && a.Source != "local"
			bOfficial := b.Source != "AUR" && b.Source != "local"
			if aOfficial != bOfficial {
				return aOfficial
			}

			// 5. Shorter name length first (relevance)
			if len(a.Name) != len(b.Name) {
				return len(a.Name) < len(b.Name)
			}

			// 6. Alphabetical fallback
			return a.Name < b.Name
		})

		ui.app.QueueUpdateDraw(func() {
			ui.shownPackages = resultList
			ui.renderPackageTable()
			ui.searching = false
			if len(resultList) == 0 {
				ui.setStatus("No packages found.")
			} else {
				ui.setStatus(fmt.Sprintf("Found %d packages.", len(resultList)))
			}
		})
	}()
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
					sb.WriteString(fmt.Sprintf("[blue]%s:[-]\n%s\n\n", f.label, f.value))
				} else {
					sb.WriteString(fmt.Sprintf("[blue]%s:[-] %s\n", f.label, f.value))
				}
			}

			// Dependencies
			if len(record.Depends) > 0 {
				sb.WriteString(fmt.Sprintf("\n[blue]Dependencies:[-]\n%s\n", strings.Join(record.Depends, ", ")))
			}

			ui.detailsView.SetText(sb.String())
			ui.detailsView.ScrollToBeginning()
		})
	}()
}

func getAurScore(p Package, term string) float64 {
	nameLower := strings.ToLower(p.Name)
	termLower := strings.ToLower(term)

	var matchScore float64
	if nameLower == termLower {
		matchScore = 5000.0
	} else if strings.HasPrefix(nameLower, termLower) {
		matchScore = 2000.0
	} else if strings.Contains(nameLower, termLower) {
		matchScore = 500.0
	} else {
		matchScore = 0.0
	}

	nameLen := len(p.Name)
	if nameLen == 0 {
		nameLen = 1
	}
	return matchScore + float64(p.Votes) + (10.0 / float64(nameLen))
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
				fmt.Printf("\n[SUCCESS] Package '%s' installed and added to drxboot.packages.\n", pkg.Name)
			} else {
				_ = pkglist.RemovePackage(ui.conf.PackagesPath, pkg.Name)
				fmt.Printf("\n[SUCCESS] Package '%s' uninstalled and removed from drxboot.packages.\n", pkg.Name)
			}
			fmt.Println("Press ENTER to return to drxpkg...")
			_, _ = os.Stdin.Read(make([]byte, 1))
		} else {
			fmt.Printf("\n[ERROR] Command failed: %v\nPress ENTER to return to drxpkg...", err)
			_, _ = os.Stdin.Read(make([]byte, 1))
		}
	})

	_ = ui.reinitPacmanDbs()
	if ui.lastSearchTerm != "" {
		ui.performSearch(ui.lastSearchTerm)
	}
}

func (ui *UI) setStatus(msg string) {
	ui.statusText.SetText(msg)
}
