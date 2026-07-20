// Package tui manages the Terminal User Interface, layouts, user input, and screen rendering.
package tui

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/druxorey/drxpkg/internal/config"
	"github.com/druxorey/drxpkg/internal/util"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type maintenanceTask struct {
	Name         string
	Description  string
	Command      string
	RequiresSudo bool
}

type maintenanceMenuItem struct {
	label       string
	itemType    string // "trash", "cache", "logs", "separator", "lockfile", "mirrors", "custom"
	scriptPath  string
	description string
}

type trashFile struct {
	Name     string
	Path     string
	Size     int64
	Selected bool
}

type cacheOption struct {
	Name         string
	Description  string
	Command      string
	RequiresSudo bool
}

type logOption struct {
	Name         string
	Description  string
	Command      string
	RequiresSudo bool
}

// getTrashFiles scans the user's local trash directory and returns a slice of trashFile objects
func getTrashFiles() []trashFile {
	trashPath := filepath.Join(os.Getenv("HOME"), ".local/share/Trash/files")
	files, err := os.ReadDir(trashPath)
	if err != nil {
		return nil
	}
	var list []trashFile
	for _, f := range files {
		info, err := f.Info()
		if err != nil {
			continue
		}
		list = append(list, trashFile{
			Name: f.Name(),
			Path: filepath.Join(trashPath, f.Name()),
			Size: info.Size(),
		})
	}
	return list
}

// formatSize converts a byte count into a human-readable string using SI prefixes
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// getCacheOptions returns a list of available cache cleaning tasks based on installed software
func (ui *UI) getCacheOptions() []cacheOption {
	var options []cacheOption

	// 1) Pacman Cache tasks
	options = append(options, cacheOption{
		Name:         "Clean Pacman Cache (Keep 3 versions)",
		Description:  "Runs 'paccache -r' to remove cached packages that are older than the 3 most recent versions. Safe and recommended.",
		Command:      "paccache -r",
		RequiresSudo: false,
	})
	options = append(options, cacheOption{
		Name:         "Clean Pacman Cache (Remove uninstalled)",
		Description:  "Runs 'pacman -Sc' to remove cached packages of uninstalled packages. Prompts for confirmation.",
		Command:      "sudo pacman -Sc",
		RequiresSudo: true,
	})
	options = append(options, cacheOption{
		Name:         "Clean Pacman Cache (Remove all)",
		Description:  "Runs 'pacman -Scc' to completely empty the pacman package cache. Reclaims maximum space.",
		Command:      "sudo pacman -Scc",
		RequiresSudo: true,
	})

	home := os.Getenv("HOME")
	cacheDir := filepath.Join(home, ".cache")

	// 2) Specific application caches from .cache or home
	if _, err := os.Stat(filepath.Join(cacheDir, "BraveSoftware")); err == nil {
		options = append(options, cacheOption{
			Name:        "Clean Brave Browser Cache",
			Description: "Removes Brave Browser cache directories.",
			Command:     "rm -rf " + filepath.Join(cacheDir, "BraveSoftware"),
		})
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "google-chrome")); err == nil {
		options = append(options, cacheOption{
			Name:        "Clean Google Chrome Cache",
			Description: "Removes Google Chrome cache directories.",
			Command:     "rm -rf " + filepath.Join(cacheDir, "google-chrome"),
		})
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "mozilla")); err == nil {
		options = append(options, cacheOption{
			Name:        "Clean Firefox Cache",
			Description: "Removes Firefox Cache directories.",
			Command:     "rm -rf " + filepath.Join(cacheDir, "mozilla"),
		})
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "go-build")); err == nil {
		options = append(options, cacheOption{
			Name:        "Clean Go Build Cache",
			Description: "Removes Go compiler build cache and pkg cache.",
			Command:     "rm -rf " + filepath.Join(cacheDir, "go-build") + " " + filepath.Join(cacheDir, "go"),
		})
	}

	// 3) AUR helper cache directories
	switch ui.aurHelper {
	case "yay":
		options = append(options, cacheOption{
			Name:        "Clean Yay Cache (AUR)",
			Description: "Runs 'yay -Sc' to clean yay helper package and build caches.",
			Command:     "yay -Sc",
		})
		options = append(options, cacheOption{
			Name:        "Remove Yay Build Cache",
			Description: "Removes all directories under ~/.cache/yay.",
			Command:     "rm -rf " + filepath.Join(cacheDir, "yay"),
		})
	case "paru":
		options = append(options, cacheOption{
			Name:        "Clean Paru Cache (AUR)",
			Description: "Runs 'paru -Sc' to clean paru helper package and build caches.",
			Command:     "paru -Sc",
		})
		options = append(options, cacheOption{
			Name:        "Remove Paru Build Cache",
			Description: "Removes all directories under ~/.cache/paru.",
			Command:     "rm -rf " + filepath.Join(cacheDir, "paru"),
		})
	}

	// 4) Other common dev / tool caches
	if _, err := os.Stat(filepath.Join(cacheDir, "pip")); err == nil {
		options = append(options, cacheOption{
			Name:        "Clean Python Pip Cache",
			Description: "Removes python pip cached packages.",
			Command:     "rm -rf " + filepath.Join(cacheDir, "pip"),
		})
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "uv")); err == nil {
		options = append(options, cacheOption{
			Name:        "Clean Python Uv Cache",
			Description: "Removes python uv cached packages.",
			Command:     "rm -rf " + filepath.Join(cacheDir, "uv"),
		})
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "mesa_shader_cache")); err == nil {
		options = append(options, cacheOption{
			Name:        "Clean Mesa Shader Cache",
			Description: "Removes GPU Mesa shader cache directories.",
			Command:     "rm -rf " + filepath.Join(cacheDir, "mesa_shader_cache"),
		})
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "thumbnails")); err == nil {
		options = append(options, cacheOption{
			Name:        "Clean Image Thumbnails",
			Description: "Removes generated file thumbnails cache.",
			Command:     "rm -rf " + filepath.Join(cacheDir, "thumbnails"),
		})
	}

	return options
}

// getLogOptions returns a list of available system and user log cleaning tasks
func (ui *UI) getLogOptions() []logOption {
	var options []logOption

	options = append(options, logOption{
		Name:         "Systemd: Vacuum Journal (50MB)",
		Description:  "Reduces systemd journal logs to a maximum size of 50MB. Safe way to reclaim disk space from old logs.",
		Command:      "sudo journalctl --vacuum-size=50M",
		RequiresSudo: true,
	})
	options = append(options, logOption{
		Name:         "Systemd: Vacuum Journal (2 weeks)",
		Description:  "Reduces systemd journal logs to a maximum age of 2 weeks. Safe way to reclaim disk space.",
		Command:      "sudo journalctl --vacuum-time=2weeks",
		RequiresSudo: true,
	})
	options = append(options, logOption{
		Name:         "System Logs: Truncate /var/log/*.log",
		Description:  "Truncates all log files under /var/log to 0 bytes, releasing space without deleting files.",
		Command:      "sudo find /var/log -type f -name '*.log' -exec truncate -s 0 {} +",
		RequiresSudo: true,
	})

	home := os.Getenv("HOME")
	cacheDir := filepath.Join(home, ".cache")

	if _, err := os.Stat(filepath.Join(cacheDir, "sysbackup.log")); err == nil {
		options = append(options, logOption{
			Name:        "User: Clean sysbackup.log",
			Description: "Removes ~/.cache/sysbackup.log.",
			Command:     "rm -f " + filepath.Join(cacheDir, "sysbackup.log"),
		})
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "xsel.log")); err == nil {
		options = append(options, logOption{
			Name:        "User: Clean xsel.log",
			Description: "Removes ~/.cache/xsel.log.",
			Command:     "rm -f " + filepath.Join(cacheDir, "xsel.log"),
		})
	}

	options = append(options, logOption{
		Name:        "User Cache: Remove all .log files",
		Description: "Finds and removes all files ending in .log under ~/.cache/.",
		Command:     "find " + cacheDir + " -type f -name '*.log' -delete",
	})

	return options
}

// loadMaintenanceItems loads standard items and custom scripts from the hooks/maintenance directory
func (ui *UI) loadMaintenanceItems() {
	ui.maintenanceItems = []maintenanceMenuItem{
		{label: "Trash", itemType: "trash"},
		{label: "Cache", itemType: "cache"},
		{label: "Logs", itemType: "logs"},
		{label: "───────────────", itemType: "separator"},
		{label: "Pacman Lock File", itemType: "lockfile"},
		{label: "Update Mirrors", itemType: "mirrors"},
	}

	configDir, err := config.GetConfigDir()
	if err == nil {
		customDir := filepath.Join(configDir, "hooks", "maintenance")
		_ = os.MkdirAll(customDir, 0755)
		files, err := os.ReadDir(customDir)
		if err == nil {
			hasCustom := false
			var customItems []maintenanceMenuItem
			for _, file := range files {
				if file.IsDir() {
					continue
				}
				path := filepath.Join(customDir, file.Name())
				desc := ui.parseScriptDescription(path)
				
				customItems = append(customItems, maintenanceMenuItem{
					label:       file.Name(),
					itemType:    "custom",
					scriptPath:  path,
					description: desc,
				})
				hasCustom = true
			}

			if hasCustom {
				// Sort custom scripts alphabetically
				sort.Slice(customItems, func(i, j int) bool {
					return customItems[i].label < customItems[j].label
				})

				ui.maintenanceItems = append(ui.maintenanceItems, maintenanceMenuItem{
					label:    "───────────────",
					itemType: "separator",
				})
				ui.maintenanceItems = append(ui.maintenanceItems, customItems...)
			}
		}
	}
}

// parseScriptDescription extracts the comment under the shebang line to use as a description
func (ui *UI) parseScriptDescription(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return "Failed to read script."
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() && len(lines) < 5 {
		lines = append(lines, scanner.Text())
	}

	if len(lines) >= 2 {
		firstLine := strings.TrimSpace(lines[0])
		if strings.HasPrefix(firstLine, "#!") {
			secondLine := strings.TrimSpace(lines[1])
			if strings.HasPrefix(secondLine, "#") {
				return strings.TrimSpace(strings.TrimPrefix(secondLine, "#"))
			}
		}
	}

	// Fallback: look for the first non-shebang comment line
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "#!") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
		}
	}

	return "No description provided."
}

// setupMaintenanceSection initializes the maintenance UI layout, including menus and sub-panels
func (ui *UI) setupMaintenanceSection() tview.Primitive {
	// Load the dynamic list of maintenance menu items
	ui.loadMaintenanceItems()

	// Initialize Left Menu Table
	ui.manageTable = ui.createStandardTable(" Menu ", 0, 0)

	for i, item := range ui.maintenanceItems {
		cell := tview.NewTableCell(item.label).SetExpansion(1)
		if item.itemType == "separator" {
			cell.SetSelectable(false).SetTextColor(ui.theme.NeutralGrayColor).SetAlign(tview.AlignCenter)
		} else if item.itemType == "lockfile" || item.itemType == "mirrors" {
			cell.SetTextColor(tcell.ColorRed)
		} else if item.itemType == "custom" {
			cell.SetTextColor(tcell.ColorGreen)
		} else {
			cell.SetTextColor(tcell.ColorDefault)
		}
		ui.manageTable.SetCell(i, 0, cell)
	}

	// 1) Trash sub-panel
	ui.trashTable = ui.createStandardTable(" Trash Files ", 1, 0)

	trashStatus := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	trashStatus.SetText("[yellow]Space[-]: Select  |  [yellow]v[-]: Visual Mode  |  [yellow]d[-]: Delete Selected  |  [yellow]TAB[-]: Back to Menu")

	trashFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.trashTable, 0, 1, true).
		AddItem(trashStatus, 1, 0, false)

	// 2) Cache sub-panel
	ui.cacheTable = ui.createStandardTable(" Cache Clean Options ", 1, 0)
	cacheDesc := ui.createStandardTextView(" Description ", true)

	ui.cacheTable.SetSelectionChangedFunc(func(row, column int) {
		if row <= 0 || row > len(ui.cacheOptions) {
			cacheDesc.Clear()
			return
		}
		opt := ui.cacheOptions[row-1]
		sudoStr := ""
		if opt.RequiresSudo {
			sudoStr = "\n\n[red::b]Requires Administrator Privileges (sudo)[-]"
		}
		cacheDesc.SetText(fmt.Sprintf("[blue::b]%s[-]\n\n[yellow]Command:[-] %s%s\n\n%s", opt.Name, opt.Command, sudoStr, opt.Description))
	})

	cacheFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.cacheTable, 0, 1, true).
		AddItem(cacheDesc, 7, 0, false)

	// 3) Logs sub-panel
	ui.logsTable = ui.createStandardTable(" Logs Clean Options ", 1, 0)
	logsDesc := ui.createStandardTextView(" Description ", true)

	ui.logsTable.SetSelectionChangedFunc(func(row, column int) {
		if row <= 0 || row > len(ui.logOptions) {
			logsDesc.Clear()
			return
		}
		opt := ui.logOptions[row-1]
		sudoStr := ""
		if opt.RequiresSudo {
			sudoStr = "\n\n[red::b]Requires Administrator Privileges (sudo)[-]"
		}
		logsDesc.SetText(fmt.Sprintf("[blue::b]%s[-]\n\n[yellow]Command:[-] %s%s\n\n%s", opt.Name, opt.Command, sudoStr, opt.Description))
	})

	logsFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.logsTable, 0, 1, true).
		AddItem(logsDesc, 7, 0, false)

	// 4) Details View (for tasks that run from left menu, like database lock/mirrors)
	ui.manageDetails = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetWrap(true).
		SetWordWrap(true)

	// Center-aligned explanation box container
	detailsContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	ui.applyStandardBorder(detailsContainer.Box, " Task Details ")
	detailsContainer.SetBorderPadding(textPaddingTop, textPaddingBottom, textPaddingLeft, textPaddingRight)
	ui.setupFocusBorder(detailsContainer)

	detailsContainer.
		AddItem(nil, 0, 1, false).
		AddItem(ui.manageDetails, 9, 0, false).
		AddItem(nil, 0, 1, false)

	// Pages layout
	ui.managePages = tview.NewPages().
		AddPage("trash", trashFlex, true, true).
		AddPage("cache", cacheFlex, true, false).
		AddPage("logs", logsFlex, true, false).
		AddPage("details", detailsContainer, true, false)

	// Input captures
	var lastSelectedLeftRow = 0
	ui.manageTable.SetSelectionChangedFunc(func(row, column int) {
		if row < 0 || row >= len(ui.maintenanceItems) {
			return
		}
		item := ui.maintenanceItems[row]
		if item.itemType == "separator" {
			if row > lastSelectedLeftRow {
				nextRow := row + 1
				if nextRow < len(ui.maintenanceItems) {
					ui.manageTable.Select(nextRow, 0)
				} else {
					ui.manageTable.Select(lastSelectedLeftRow, 0)
				}
			} else {
				prevRow := row - 1
				if prevRow >= 0 {
					ui.manageTable.Select(prevRow, 0)
				} else {
					ui.manageTable.Select(lastSelectedLeftRow, 0)
				}
			}
			return
		}
		lastSelectedLeftRow = row
		ui.updateMaintenanceRightPanel(row)
	})

	ui.manageTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, _ := ui.manageTable.GetSelection()
		if row < 0 || row >= len(ui.maintenanceItems) {
			return event
		}
		if ui.handleTableVimNavigation(event, ui.manageTable, len(ui.maintenanceItems)) {
			return nil
		}
		item := ui.maintenanceItems[row]
		if event.Key() == tcell.KeyTAB {
			switch item.itemType {
			case "trash":
				ui.app.SetFocus(ui.trashTable)
			case "cache":
				ui.app.SetFocus(ui.cacheTable)
			case "logs":
				ui.app.SetFocus(ui.logsTable)
			}
			return nil
		}
		if event.Key() == tcell.KeyEnter {
			switch item.itemType {
			case "lockfile":
				ui.promptMaintenance(maintenanceTask{
					Name:         "Fix Pacman Database Lock File",
					Command:      "sudo rm -f /var/lib/pacman/db.lck",
					RequiresSudo: true,
				})
			case "mirrors":
				cmd := "rate-mirrors arch | sudo tee /etc/pacman.d/mirrorlist"
				if ui.isCachyOS {
					cmd = "cachyrate-mirrors"
				}
				ui.promptMaintenance(maintenanceTask{
					Name:         "Update Package Mirrors (Benchmark)",
					Command:      cmd,
					RequiresSudo: false,
				})
			case "custom":
				ui.promptMaintenance(maintenanceTask{
					Name:         item.label,
					Command:      item.scriptPath,
					RequiresSudo: false,
				})
			}
			return nil
		}
		return event
	})

	ui.trashTable.SetSelectionChangedFunc(func(row, column int) {
		if ui.isRendering {
			return
		}
		if ui.inVisualMode {
			ui.visualEndRow = row
			ui.renderTrashTable()
		}
	})

	ui.trashTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, _ := ui.trashTable.GetSelection()
		if ui.handleTableVimNavigation(event, ui.trashTable, len(ui.trashFiles)) {
			return nil
		}
		if event.Key() == tcell.KeyTAB || event.Key() == tcell.KeyBacktab {
			ui.app.SetFocus(ui.manageTable)
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			if ui.inVisualMode {
				ui.inVisualMode = false
				ui.updateStatusDisplay()
				ui.renderTrashTable()
				return nil
			}
		}
		if ui.handleVisualModeToggle(event, ui.trashTable, ui.renderTrashTable) {
			return nil
		}
		if event.Key() == tcell.KeyRune && event.Rune() == ' ' {
			if ui.inVisualMode {
				minRow := min(ui.visualStartRow, ui.visualEndRow)
				maxRow := max(ui.visualStartRow, ui.visualEndRow)
				for r := minRow; r <= maxRow; r++ {
					if r > 0 && r <= len(ui.trashFiles) {
						ui.trashFiles[r-1].Selected = !ui.trashFiles[r-1].Selected
					}
				}
				ui.inVisualMode = false
				ui.updateStatusDisplay()
				ui.renderTrashTable()
			} else {
				if row > 0 && row <= len(ui.trashFiles) {
					ui.trashFiles[row-1].Selected = !ui.trashFiles[row-1].Selected
					ui.renderTrashTable()
					ui.trashTable.Select(row, 0)
				}
			}
			return nil
		}
		if event.Key() == tcell.KeyRune && event.Rune() == 'd' {
			ui.deleteSelectedTrash()
			return nil
		}
		return event
	})

	ui.cacheTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, _ := ui.cacheTable.GetSelection()
		if ui.handleTableVimNavigation(event, ui.cacheTable, len(ui.cacheOptions)) {
			return nil
		}
		if event.Key() == tcell.KeyTAB || event.Key() == tcell.KeyBacktab {
			ui.app.SetFocus(ui.manageTable)
			return nil
		}
		if event.Key() == tcell.KeyEnter {
			if row > 0 && row <= len(ui.cacheOptions) {
				ui.promptCacheClean(ui.cacheOptions[row-1])
			}
			return nil
		}
		return event
	})

	ui.logsTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, _ := ui.logsTable.GetSelection()
		if ui.handleTableVimNavigation(event, ui.logsTable, len(ui.logOptions)) {
			return nil
		}
		if event.Key() == tcell.KeyTAB || event.Key() == tcell.KeyBacktab {
			ui.app.SetFocus(ui.manageTable)
			return nil
		}
		if event.Key() == tcell.KeyEnter {
			if row > 0 && row <= len(ui.logOptions) {
				ui.promptLogsClean(ui.logOptions[row-1])
			}
			return nil
		}
		return event
	})

	// Initial selection load
	ui.updateMaintenanceRightPanel(0)

	ui.manageFlex = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ui.manageTable, 24, 0, true).
		AddItem(ui.managePages, 0, 1, false)

	return ui.manageFlex
}

// updateMaintenanceRightPanel updates the right-hand panel view based on the selected menu item
func (ui *UI) updateMaintenanceRightPanel(row int) {
	if ui.managePages == nil || row < 0 || row >= len(ui.maintenanceItems) {
		return
	}
	item := ui.maintenanceItems[row]
	switch item.itemType {
	case "trash":
		ui.trashFiles = getTrashFiles()
		ui.renderTrashTable()
		ui.managePages.SwitchToPage("trash")
		if len(ui.trashFiles) > 0 {
			ui.trashTable.Select(1, 0)
		}
	case "cache":
		ui.cacheOptions = ui.getCacheOptions()
		ui.renderCacheTable()
		ui.managePages.SwitchToPage("cache")
		if len(ui.cacheOptions) > 0 {
			ui.cacheTable.Select(1, 0)
		}
	case "logs":
		ui.logOptions = ui.getLogOptions()
		ui.renderLogsTable()
		ui.managePages.SwitchToPage("logs")
		if len(ui.logOptions) > 0 {
			ui.logsTable.Select(1, 0)
		}
	case "lockfile":
		ui.manageDetails.SetText("Fix Pacman Database Lock File\n\nRemoves /var/lib/pacman/db.lck.\nRun this if pacman or yay is locked due to a previous crash or interruption.\n\n[red]Requires Administrator Privileges (sudo)[-]\n\n[yellow::b]Press ENTER on the left menu to execute this task.[-]")
		ui.managePages.SwitchToPage("details")
	case "mirrors":
		cmd := "rate-mirrors arch | sudo tee /etc/pacman.d/mirrorlist"
		if ui.isCachyOS {
			cmd = "cachyrate-mirrors"
		}
		ui.manageDetails.SetText(fmt.Sprintf("Update Package Mirrors (Benchmark)\n\nBenchmarks and updates package mirrors.\nCommand: %s\n\n[yellow::b]Press ENTER on the left menu to execute this task.[-]", cmd))
		ui.managePages.SwitchToPage("details")
	case "custom":
		ui.manageDetails.SetText(fmt.Sprintf("[blue::b]%s[-]\n\n%s\n\n[yellow]Path:[-] %s\n\n[yellow::b]Press ENTER on the left menu to execute this task.[-]", item.label, item.description, item.scriptPath))
		ui.managePages.SwitchToPage("details")
	}
}

// renderTrashTable refreshes the trash files table content and visual selection states
func (ui *UI) renderTrashTable() {
	ui.isRendering = true
	defer func() { ui.isRendering = false }()

	selectedRow, _ := ui.trashTable.GetSelection()
	ui.trashTable.Clear()

	ui.trashTable.SetCell(0, 0, tview.NewTableCell("").SetSelectable(false).SetMaxWidth(8))
	ui.trashTable.SetCell(0, 1, tview.NewTableCell("File").SetTextColor(ui.theme.PrimaryColor).SetSelectable(false).SetExpansion(1))
	ui.trashTable.SetCell(0, 2, tview.NewTableCell("Size").SetTextColor(ui.theme.PrimaryColor).SetSelectable(false))

	if len(ui.trashFiles) == 0 {
		cell := tview.NewTableCell("Trash is empty").SetAlign(tview.AlignCenter).SetTextColor(ui.theme.NeutralGrayColor).SetSelectable(false)
		ui.trashTable.SetCell(1, 1, cell)
		return
	}

	for i, tf := range ui.trashFiles {
		row := i + 1
		isHighlighted := false
		if ui.inVisualMode && ui.activeTab == 2 {
			minRow := min(ui.visualStartRow, ui.visualEndRow)
			maxRow := max(ui.visualStartRow, ui.visualEndRow)
			if row >= minRow && row <= maxRow {
				isHighlighted = true
			}
		}

		selStr := "   "
		if tf.Selected {
			selStr = "  x"
		}
		selCell := tview.NewTableCell(selStr).SetMaxWidth(8).SetAlign(tview.AlignLeft)
		if tf.Selected {
			selCell.SetTextColor(tcell.ColorGreen)
		}

		nameCell := tview.NewTableCell(tf.Name).SetExpansion(1)
		sizeCell := tview.NewTableCell(formatSize(tf.Size)).SetAlign(tview.AlignRight)

		if isHighlighted {
			bgColor := tcell.NewHexColor(0x1a3a5c)
			selCell.SetBackgroundColor(bgColor)
			nameCell.SetBackgroundColor(bgColor)
			sizeCell.SetBackgroundColor(bgColor)
		}

		ui.trashTable.SetCell(row, 0, selCell)
		ui.trashTable.SetCell(row, 1, nameCell)
		ui.trashTable.SetCell(row, 2, sizeCell)
	}

	if selectedRow > 0 && selectedRow <= len(ui.trashFiles) {
		ui.trashTable.Select(selectedRow, 0)
	} else if len(ui.trashFiles) > 0 {
		ui.trashTable.Select(1, 0)
	}
}

// renderCacheTable updates the cache cleaning options table
func (ui *UI) renderCacheTable() {
	ui.cacheTable.Clear()
	ui.cacheTable.SetCell(0, 0, tview.NewTableCell("Cache Option").SetTextColor(ui.theme.PrimaryColor).SetSelectable(false).SetExpansion(1))

	for i, opt := range ui.cacheOptions {
		row := i + 1
		nameCell := tview.NewTableCell(opt.Name).SetExpansion(1)
		if opt.RequiresSudo {
			nameCell.SetTextColor(tcell.ColorRed)
		} else {
			nameCell.SetTextColor(tcell.ColorDefault)
		}
		ui.cacheTable.SetCell(row, 0, nameCell)
	}
}

// renderLogsTable updates the log cleaning options table
func (ui *UI) renderLogsTable() {
	ui.logsTable.Clear()
	ui.logsTable.SetCell(0, 0, tview.NewTableCell("Log Option").SetTextColor(ui.theme.PrimaryColor).SetSelectable(false).SetExpansion(1))

	for i, opt := range ui.logOptions {
		row := i + 1
		nameCell := tview.NewTableCell(opt.Name).SetExpansion(1)
		if opt.RequiresSudo {
			nameCell.SetTextColor(tcell.ColorRed)
		} else {
			nameCell.SetTextColor(tcell.ColorDefault)
		}
		ui.logsTable.SetCell(row, 0, nameCell)
	}
}

// deleteSelectedTrash prompts the user for confirmation and removes selected files from the trash
func (ui *UI) deleteSelectedTrash() {
	var toDelete []trashFile
	for _, tf := range ui.trashFiles {
		if tf.Selected {
			toDelete = append(toDelete, tf)
		}
	}

	if len(toDelete) == 0 {
		row, _ := ui.trashTable.GetSelection()
		if row > 0 && row <= len(ui.trashFiles) {
			toDelete = append(toDelete, ui.trashFiles[row-1])
		}
	}

	if len(toDelete) == 0 {
		ui.showAlert("No files in trash selected or highlighted to delete.")
		return
	}

	msg := fmt.Sprintf("Are you sure you want to permanently delete these %d file(s) from Trash?", len(toDelete))
	ui.showConfirmation(msg, func() {
		trashPath := filepath.Join(os.Getenv("HOME"), ".local/share/Trash/files")
		infoPath := filepath.Join(os.Getenv("HOME"), ".local/share/Trash/info")

		for _, tf := range toDelete {
			_ = os.RemoveAll(filepath.Join(trashPath, tf.Name))
			_ = os.RemoveAll(filepath.Join(infoPath, tf.Name+".trashinfo"))
		}

		ui.trashFiles = getTrashFiles()
		ui.renderTrashTable()
	})
}

// promptCacheClean displays a confirmation dialog before executing a cache cleaning task
func (ui *UI) promptCacheClean(opt cacheOption) {
	message := fmt.Sprintf("Are you sure you want to clean: %s?", opt.Name)
	ui.showConfirmation(message, func() {
		ui.performMaintenance(maintenanceTask{
			Name:         opt.Name,
			Command:      opt.Command,
			RequiresSudo: opt.RequiresSudo,
		})
		ui.cacheOptions = ui.getCacheOptions()
		ui.renderCacheTable()
	})
}

// promptLogsClean displays a confirmation dialog before executing a log cleaning task
func (ui *UI) promptLogsClean(opt logOption) {
	message := fmt.Sprintf("Are you sure you want to clean logs: %s?", opt.Name)
	ui.showConfirmation(message, func() {
		ui.performMaintenance(maintenanceTask{
			Name:         opt.Name,
			Command:      opt.Command,
			RequiresSudo: opt.RequiresSudo,
		})
		ui.logOptions = ui.getLogOptions()
		ui.renderLogsTable()
	})
}

// promptMaintenance verifies dependencies and prompts the user before running administrative tasks
func (ui *UI) promptMaintenance(task maintenanceTask) {
	if task.Name == "Update Package Mirrors (Benchmark)" {
		binary := "rate-mirrors"
		if ui.isCachyOS {
			binary = "cachyrate-mirrors"
		}
		if _, err := exec.LookPath(binary); err != nil {
			msg := fmt.Sprintf("The mirror update tool '%s' is not installed.\n\nPlease install it to continue (e.g., run 'pacman -S %s' or use your AUR helper).", binary, binary)
			ui.showAlert(msg)
			return
		}
	}

	message := fmt.Sprintf("Are you sure you want to run: %s?", task.Name)
	ui.showConfirmation(message, func() {
		ui.performMaintenance(task)
	})
}

// performMaintenance runs the given shell command, managing UI suspension and terminal restoration
func (ui *UI) performMaintenance(task maintenanceTask) {
	ui.app.Suspend(func() {
		fmt.Print("\033[H\033[2J")
		fmt.Printf("Executing: %s\n", task.Name)
		fmt.Printf("Command:   %s\n\n", task.Command)

		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}

		cmd := exec.Command(shell, "-c", task.Command)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err == nil {
			util.PrintSuccess("\nTask completed successfully!\n")
		} else {
			util.PrintError("\nCommand failed: %v\n", err)
		}

		fmt.Println("Press ENTER to return to drxpkg...")
		_, _ = os.Stdin.Read(make([]byte, 1))
	})

	_ = ui.reinitPacmanDbs()
}
