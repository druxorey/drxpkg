package tui

import (
	"strconv"

	"github.com/druxorey/drxpkg/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// showConfirmation displays a modal dialog with Yes/No options and executes onConfirm if confirmed
func (ui *UI) showConfirmation(message string, onConfirm func()) {
	ui.confirmationFocusedIndex = 0
	prevFocus := ui.app.GetFocus()
	modal := ui.createStyledModal(message, []string{"Yes", "No"}, func(buttonIndex int, buttonLabel string) {
		ui.pages.RemovePage("confirmation")
		ui.app.SetFocus(prevFocus)
		if buttonLabel == "Yes" {
			onConfirm()
		}
	})

	ui.pages.AddPage("confirmation", modal, true, true)
}

// showAlert displays a modal dialog with an OK button
func (ui *UI) showAlert(message string) {
	prevFocus := ui.app.GetFocus()
	modal := ui.createStyledModal(message, []string{"OK"}, func(buttonIndex int, buttonLabel string) {
		ui.pages.RemovePage("alert")
		ui.app.SetFocus(prevFocus)
	})

	ui.pages.AddPage("alert", modal, true, true)
}

// setupHelpPopup initializes the help popup components and layout
func (ui *UI) setupHelpPopup() {
	helpText := `[yellow]Global Shortcuts:[-]
  [green]ESC[-]          Exit application / Close active popup
  [green].[-]            Open Settings popup
  [green]?[-]            Open Help popup
  [green][[-], [green]][-]        Switch between tabs
  [green]F1[-] - [green]F3[-]      Switch tab directly
  [green]F4[-]            Open Settings popup

[yellow]Install Tab:[-]
  [green]Up[-] / [green]Down[-]     Navigate search results list
  [green]Ctrl-N[-] / [green]Ctrl-P[-] Navigate search results list
  [green]Enter[-]        Install currently typed/selected package (must be valid)
  [green]TAB[-] / [green]BackTAB[-] Toggle focus between Search panel and Details panel

[yellow]Update Tab:[-]
  [green]Space[-]        Toggle update selection
  [green]a[-] / [green]A[-]        Toggle select all updates
  [green]v[-] / [green]V[-]        Toggle visual mode for range selection
  [green]i[-] / [green]I[-]        Run system upgrade (install updates)
  [green]gg[-]           Go to beginning of table
  [green]G[-]            Go to end of table

[yellow]Settings Popup:[-]
  [green]Up[-]/[green]Down[-]/[green]TAB[-] Navigate settings fields
  [green]Enter[-] / [green]i[-]   Edit text field / Toggle checkbox / Press button
  [green]Left[-] / [green]Right[-]   Navigate buttons`

	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetText(helpText)
	textView.SetBackgroundColor(tcell.ColorBlack)

	helpFrame := tview.NewFrame(textView).
		SetBorders(1, 1, 0, 0, 2, 2)
	helpFrame.SetBorder(true).
		SetTitle(" Keyboard Shortcuts ").
		SetBorderColor(ui.theme.PrimaryColor).
		SetTitleColor(ui.theme.PrimaryColor)
	helpFrame.SetBackgroundColor(tcell.ColorBlack)

	ui.helpGrid = ui.createCenteredGrid(helpFrame, 80, 26)

	ui.helpGrid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyEnter || event.Rune() == ' ' || event.Rune() == '?' {
			ui.closeHelpPopup()
			return nil
		}
		return event
	})
}

// showHelpPopup makes the help popup visible and focuses it
func (ui *UI) showHelpPopup() {
	ui.helpPopupOpen = true
	ui.pages.ShowPage("help")
	ui.app.SetFocus(ui.helpGrid)
}

// closeHelpPopup hides the help popup and restores focus to the active tab
func (ui *UI) closeHelpPopup() {
	ui.helpPopupOpen = false
	ui.pages.HidePage("help")
	ui.restoreFocusToActiveTab()
}

// setupSettingsPopup initializes the settings form, inputs, and button layout
func (ui *UI) setupSettingsPopup() {
	// Initialize inputs
	ui.settingInputs = make([]*tview.InputField, 6)
	ui.settingInputs[settingIdxSavePath] = tview.NewInputField().SetText(ui.conf.PackagesPath)
	ui.settingInputs[settingIdxFileName] = tview.NewInputField().SetText(ui.conf.PackagesFile)
	ui.settingInputs[settingIdxInstallCmd] = tview.NewInputField().SetText(ui.conf.InstallCommand)
	ui.settingInputs[settingIdxUninstallCmd] = tview.NewInputField().SetText(ui.conf.UninstallCommand)
	ui.settingInputs[settingIdxUpgradeCmd] = tview.NewInputField().SetText(ui.conf.SysUpgradeCmd)
	ui.settingInputs[settingIdxMaxResults] = tview.NewInputField().SetText(strconv.Itoa(ui.conf.MaxResults))

	for i, input := range ui.settingInputs {
		idx := i
		input.SetBorder(true).SetBorderColor(ui.theme.NeutralGrayColor)
		input.SetBackgroundColor(tcell.ColorBlack)
		input.SetFieldBackgroundColor(tcell.ColorBlack)
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

	// Initialize checkboxes
	ui.settingAurCb = tview.NewCheckbox().SetLabel("").SetChecked(ui.conf.DisableAur)
	ui.settingAurCb.SetBackgroundColor(tcell.ColorBlack)
	ui.settingAurCb.SetFieldBackgroundColor(tcell.ColorBlack)
	ui.settingAurCb.SetFieldTextColor(tcell.ColorDefault)
	ui.settingAurCb.SetFocusFunc(func() {
		ui.settingsFocusedIndex = settingIdxAurCb
		ui.updateSettingsDisplay()
	})

	ui.settingHooksCb = tview.NewCheckbox().SetLabel("").SetChecked(ui.conf.RunUpdateHooks)
	ui.settingHooksCb.SetBackgroundColor(tcell.ColorBlack)
	ui.settingHooksCb.SetFieldBackgroundColor(tcell.ColorBlack)
	ui.settingHooksCb.SetFieldTextColor(tcell.ColorDefault)
	ui.settingHooksCb.SetFocusFunc(func() {
		ui.settingsFocusedIndex = settingIdxHooksCb
		ui.updateSettingsDisplay()
	})

	// Initialize buttons
	ui.btnSave = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	ui.btnDefaults = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)

	// Fields Grid: 6 inputs (height 3 each), 2 checkboxes (height 1 each)
	fieldsGrid := tview.NewGrid().
		SetRows(3, 3, 3, 3, 3, 3, 1, 1).
		SetColumns(25, 0)
	fieldsGrid.SetBackgroundColor(tcell.ColorBlack)

	labels := make([]string, 8)
	labels[settingIdxSavePath] = "Packages Save Path"
	labels[settingIdxFileName] = "Packages File Name"
	labels[settingIdxInstallCmd] = "Install Command"
	labels[settingIdxUninstallCmd] = "Uninstall Command"
	labels[settingIdxUpgradeCmd] = "Upgrade Command"
	labels[settingIdxMaxResults] = "Max Results"
	labels[settingIdxAurCb] = "Disable AUR"
	labels[settingIdxHooksCb] = "Run Update Hooks"

	for i, name := range labels {
		lblText := "  " + name
		if i < 6 {
			lblText = "\n" + lblText
		}
		lbl := tview.NewTextView().SetDynamicColors(true).SetText(lblText)
		lbl.SetTextColor(tcell.ColorDefault)
		lbl.SetBackgroundColor(tcell.ColorBlack)

		if i < 6 {
			fieldsGrid.AddItem(lbl, i, 0, 1, 1, 0, 0, false)
			fieldsGrid.AddItem(ui.settingInputs[i], i, 1, 1, 1, 0, 0, false)
		} else if i == settingIdxAurCb {
			fieldsGrid.AddItem(lbl, i, 0, 1, 1, 0, 0, false)
			fieldsGrid.AddItem(ui.settingAurCb, i, 1, 1, 1, 0, 0, false)
		} else if i == settingIdxHooksCb {
			fieldsGrid.AddItem(lbl, i, 0, 1, 1, 0, 0, false)
			fieldsGrid.AddItem(ui.settingHooksCb, i, 1, 1, 1, 0, 0, false)
		}
	}

	// Buttons Flex Row
	buttonsFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).
		AddItem(ui.btnSave, 18, 0, false).
		AddItem(nil, 4, 0, false).
		AddItem(ui.btnDefaults, 14, 0, false).
		AddItem(nil, 0, 1, false)
	buttonsFlex.SetBackgroundColor(tcell.ColorBlack)

	// Settings Box layout (centered inside grid)
	settingsFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(fieldsGrid, 0, 1, true).
		AddItem(nil, 1, 0, false).
		AddItem(buttonsFlex, 2, 0, false)
	settingsFlex.SetBackgroundColor(tcell.ColorBlack)

	settingsFrame := tview.NewFrame(settingsFlex).
		SetBorders(1, 1, 0, 0, 3, 3)
	settingsFrame.SetBorder(true).
		SetTitle(" Settings ").
		SetBorderColor(ui.theme.PrimaryColor).
		SetTitleColor(ui.theme.PrimaryColor)
	settingsFrame.SetBackgroundColor(tcell.ColorBlack)

	// Center the settingsFrame using a grid
	ui.settingsGrid = ui.createCenteredGrid(settingsFrame, 85, 36)

	// Set input capture on settingsGrid for navigation
	ui.settingsGrid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if ui.settingsEditMode {
			return event
		}

		switch event.Key() {
		case tcell.KeyUp:
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + settingNumItems - 1) % settingNumItems
			ui.updateSettingsDisplay()
			return nil
		case tcell.KeyDown:
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 1) % settingNumItems
			ui.updateSettingsDisplay()
			return nil
		case tcell.KeyLeft:
			if ui.settingsFocusedIndex == settingIdxBtnDefaults {
				ui.settingsFocusedIndex = settingIdxBtnSave
				ui.updateSettingsDisplay()
				return nil
			}
		case tcell.KeyRight:
			if ui.settingsFocusedIndex == settingIdxBtnSave {
				ui.settingsFocusedIndex = settingIdxBtnDefaults
				ui.updateSettingsDisplay()
				return nil
			}
		case tcell.KeyTAB:
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 1) % settingNumItems
			ui.updateSettingsDisplay()
			return nil
		case tcell.KeyBacktab:
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + settingNumItems - 1) % settingNumItems
			ui.updateSettingsDisplay()
			return nil
		case tcell.KeyEnter:
			ui.handleSettingsSelect()
			return nil
		}

		switch event.Rune() {
		case 'j', 'J':
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 1) % settingNumItems
			ui.updateSettingsDisplay()
			return nil
		case 'k', 'K':
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + settingNumItems - 1) % settingNumItems
			ui.updateSettingsDisplay()
			return nil
		case 'h', 'H':
			if ui.settingsFocusedIndex == settingIdxBtnDefaults {
				ui.settingsFocusedIndex = settingIdxBtnSave
				ui.updateSettingsDisplay()
				return nil
			}
		case 'l', 'L':
			if ui.settingsFocusedIndex == settingIdxBtnSave {
				ui.settingsFocusedIndex = settingIdxBtnDefaults
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

// showSettingsPopup prepares the settings view with current config values and displays it
func (ui *UI) showSettingsPopup() {
	ui.settingsPopupOpen = true
	ui.settingInputs[settingIdxSavePath].SetText(ui.conf.PackagesPath)
	ui.settingInputs[settingIdxFileName].SetText(ui.conf.PackagesFile)
	ui.settingInputs[settingIdxInstallCmd].SetText(ui.conf.InstallCommand)
	ui.settingInputs[settingIdxUninstallCmd].SetText(ui.conf.UninstallCommand)
	ui.settingInputs[settingIdxUpgradeCmd].SetText(ui.conf.SysUpgradeCmd)
	ui.settingInputs[settingIdxMaxResults].SetText(strconv.Itoa(ui.conf.MaxResults))
	ui.settingAurCb.SetChecked(ui.conf.DisableAur)
	ui.settingHooksCb.SetChecked(ui.conf.RunUpdateHooks)
	ui.updateSettingsDisplay()

	ui.pages.ShowPage("settings")
	ui.app.SetFocus(ui.settingsGrid)
}

// closeSettingsPopup hides the settings popup and restores focus to the active tab
func (ui *UI) closeSettingsPopup() {
	ui.settingsPopupOpen = false
	ui.pages.HidePage("settings")
	ui.restoreFocusToActiveTab()
}

// updateSettingsDisplay updates the visual styling of inputs, checkboxes, and buttons based on selection state
func (ui *UI) updateSettingsDisplay() {
	// Reset all inputs border color
	for i, input := range ui.settingInputs {
		if i == ui.settingsFocusedIndex {
			if ui.settingsEditMode {
				input.SetBorderColor(ui.theme.EditingBorderColor)
			} else {
				input.SetBorderColor(ui.theme.FocusedBorderColor)
			}
		} else {
			input.SetBorderColor(ui.theme.NeutralGrayColor)
		}
	}

	// Disable AUR Checkbox styling
	if ui.settingsFocusedIndex == settingIdxAurCb {
		ui.settingAurCb.SetFieldBackgroundColor(ui.theme.SettingsFieldFocusedBg)
		ui.settingAurCb.SetFieldTextColor(ui.theme.SettingsFieldFocusedFg)
	} else {
		ui.settingAurCb.SetFieldBackgroundColor(tcell.ColorBlack)
		ui.settingAurCb.SetFieldTextColor(ui.theme.SelectedTextColor)
	}

	// Run Update Hooks Checkbox styling
	if ui.settingsFocusedIndex == settingIdxHooksCb {
		ui.settingHooksCb.SetFieldBackgroundColor(ui.theme.SettingsFieldFocusedBg)
		ui.settingHooksCb.SetFieldTextColor(ui.theme.SettingsFieldFocusedFg)
	} else {
		ui.settingHooksCb.SetFieldBackgroundColor(tcell.ColorBlack)
		ui.settingHooksCb.SetFieldTextColor(ui.theme.SelectedTextColor)
	}

	// Save button styling
	if ui.settingsFocusedIndex == settingIdxBtnSave {
		ui.btnSave.SetTextColor(tcell.ColorDefault)
		ui.btnSave.SetBackgroundColor(ui.theme.PrimaryColor)
	} else {
		ui.btnSave.SetTextColor(ui.theme.SelectedTextColor)
		ui.btnSave.SetBackgroundColor(ui.theme.NeutralGrayColor)
	}
	ui.btnSave.SetText("Apply & Save")

	// Defaults button styling
	if ui.settingsFocusedIndex == settingIdxBtnDefaults {
		ui.btnDefaults.SetTextColor(tcell.ColorDefault)
		ui.btnDefaults.SetBackgroundColor(ui.theme.PrimaryColor)
	} else {
		ui.btnDefaults.SetTextColor(ui.theme.SelectedTextColor)
		ui.btnDefaults.SetBackgroundColor(ui.theme.NeutralGrayColor)
	}
	ui.btnDefaults.SetText("Defaults")
}

// handleSettingsSelect processes input selection based on the currently focused element
func (ui *UI) handleSettingsSelect() {
	if ui.settingsFocusedIndex >= 0 && ui.settingsFocusedIndex < settingIdxAurCb {
		ui.settingsEditMode = true
		ui.updateSettingsDisplay()
		ui.app.SetFocus(ui.settingInputs[ui.settingsFocusedIndex])
	} else if ui.settingsFocusedIndex == settingIdxAurCb {
		ui.settingAurCb.SetChecked(!ui.settingAurCb.IsChecked())
		ui.updateSettingsDisplay()
	} else if ui.settingsFocusedIndex == settingIdxHooksCb {
		ui.settingHooksCb.SetChecked(!ui.settingHooksCb.IsChecked())
		ui.updateSettingsDisplay()
	} else if ui.settingsFocusedIndex == settingIdxBtnSave {
		ui.saveSettingsAction()
	} else if ui.settingsFocusedIndex == settingIdxBtnDefaults {
		ui.loadSettingsDefaults()
	}
}

// saveSettingsAction updates the configuration struct with current form values and persists them to disk
func (ui *UI) saveSettingsAction() {
	ui.conf.PackagesPath = ui.settingInputs[settingIdxSavePath].GetText()
	ui.conf.PackagesFile = ui.settingInputs[settingIdxFileName].GetText()
	ui.conf.InstallCommand = ui.settingInputs[settingIdxInstallCmd].GetText()
	ui.conf.UninstallCommand = ui.settingInputs[settingIdxUninstallCmd].GetText()
	ui.conf.SysUpgradeCmd = ui.settingInputs[settingIdxUpgradeCmd].GetText()

	maxRes, err := strconv.Atoi(ui.settingInputs[settingIdxMaxResults].GetText())
	if err == nil {
		ui.conf.MaxResults = maxRes
	}

	ui.conf.DisableAur = ui.settingAurCb.IsChecked()
	ui.conf.RunUpdateHooks = ui.settingHooksCb.IsChecked()

	if err := ui.conf.Save(); err != nil {
		ui.setStatus("Error saving settings: " + err.Error())
	} else {
		ui.setStatus("Settings saved successfully!")
		ui.closeSettingsPopup()
	}
	_ = ui.reinitPacmanDbs()
}

// loadSettingsDefaults resets the UI inputs to the default configuration values
func (ui *UI) loadSettingsDefaults() {
	ui.conf = config.Defaults()
	ui.settingInputs[settingIdxSavePath].SetText(ui.conf.PackagesPath)
	ui.settingInputs[settingIdxFileName].SetText(ui.conf.PackagesFile)
	ui.settingInputs[settingIdxInstallCmd].SetText(ui.conf.InstallCommand)
	ui.settingInputs[settingIdxUninstallCmd].SetText(ui.conf.UninstallCommand)
	ui.settingInputs[settingIdxUpgradeCmd].SetText(ui.conf.SysUpgradeCmd)
	ui.settingInputs[settingIdxMaxResults].SetText(strconv.Itoa(ui.conf.MaxResults))
	ui.settingAurCb.SetChecked(ui.conf.DisableAur)
	ui.settingHooksCb.SetChecked(ui.conf.RunUpdateHooks)
	ui.setStatus("Settings reset to defaults!")
}
