// Package tui manages the Terminal User Interface, layouts, user input, and screen rendering.
package tui

import (
	"strconv"

	"github.com/druxorey/drxpkg/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)


func (ui *UI) setupSettingsPopup() {
	// Initialize inputs
	ui.settingInputs = make([]*tview.InputField, 8)
	ui.settingInputs[0] = tview.NewInputField().SetText(ui.conf.PackagesPath)
	ui.settingInputs[1] = tview.NewInputField().SetText(ui.conf.PackagesFile)
	ui.settingInputs[2] = tview.NewInputField().SetText(ui.conf.PacmanDBPath)
	ui.settingInputs[3] = tview.NewInputField().SetText(ui.conf.PacmanConfigPath)
	ui.settingInputs[4] = tview.NewInputField().SetText(ui.conf.InstallCommand)
	ui.settingInputs[5] = tview.NewInputField().SetText(ui.conf.UninstallCommand)
	ui.settingInputs[6] = tview.NewInputField().SetText(ui.conf.SysUpgradeCmd)
	ui.settingInputs[7] = tview.NewInputField().SetText(strconv.Itoa(ui.conf.MaxResults))

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
		ui.settingsFocusedIndex = 8
		ui.updateSettingsDisplay()
	})

	ui.settingHooksCb = tview.NewCheckbox().SetLabel("").SetChecked(ui.conf.RunUpdateHooks)
	ui.settingHooksCb.SetBackgroundColor(tcell.ColorBlack)
	ui.settingHooksCb.SetFieldBackgroundColor(tcell.ColorBlack)
	ui.settingHooksCb.SetFieldTextColor(tcell.ColorDefault)
	ui.settingHooksCb.SetFocusFunc(func() {
		ui.settingsFocusedIndex = 9
		ui.updateSettingsDisplay()
	})

	// Initialize buttons
	ui.btnSave = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	ui.btnDefaults = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)

	// Fields Grid: 8 inputs (height 3 each), 2 checkboxes (height 1 each)
	fieldsGrid := tview.NewGrid().
		SetRows(3, 3, 3, 3, 3, 3, 3, 3, 1, 1).
		SetColumns(25, 0)
	fieldsGrid.SetBackgroundColor(tcell.ColorBlack)

	labels := []string{
		"Packages Save Path",
		"Packages File Name",
		"Pacman DB Path",
		"Pacman Config Path",
		"Install Command",
		"Uninstall Command",
		"Upgrade Command",
		"Max Results",
		"Disable AUR",
		"Run Update Hooks",
	}

	for i, name := range labels {
		lblText := "  " + name
		if i < 8 {
			lblText = "\n" + lblText
		}
		lbl := tview.NewTextView().SetDynamicColors(true).SetText(lblText)
		lbl.SetTextColor(tcell.ColorDefault)
		lbl.SetBackgroundColor(tcell.ColorBlack)

		if i < 8 {
			fieldsGrid.AddItem(lbl, i, 0, 1, 1, 0, 0, false)
			fieldsGrid.AddItem(ui.settingInputs[i], i, 1, 1, 1, 0, 0, false)
		} else if i == 8 {
			fieldsGrid.AddItem(lbl, i, 0, 1, 1, 0, 0, false)
			fieldsGrid.AddItem(ui.settingAurCb, i, 1, 1, 1, 0, 0, false)
		} else {
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
	ui.settingsGrid = tview.NewGrid().
		SetRows(0, 36, 0).
		SetColumns(0, 85, 0).
		AddItem(settingsFrame, 1, 1, 1, 1, 0, 0, true)

	// Set input capture on settingsGrid for navigation (12 indices: 0..11)
	ui.settingsGrid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if ui.settingsEditMode {
			return event
		}

		switch event.Key() {
		case tcell.KeyUp:
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 11) % 12
			ui.updateSettingsDisplay()
			return nil
		case tcell.KeyDown:
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 1) % 12
			ui.updateSettingsDisplay()
			return nil
		case tcell.KeyLeft:
			if ui.settingsFocusedIndex == 11 {
				ui.settingsFocusedIndex = 10
				ui.updateSettingsDisplay()
				return nil
			}
		case tcell.KeyRight:
			if ui.settingsFocusedIndex == 10 {
				ui.settingsFocusedIndex = 11
				ui.updateSettingsDisplay()
				return nil
			}
		case tcell.KeyTAB:
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 1) % 12
			ui.updateSettingsDisplay()
			return nil
		case tcell.KeyBacktab:
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 11) % 12
			ui.updateSettingsDisplay()
			return nil
		case tcell.KeyEnter:
			ui.handleSettingsSelect()
			return nil
		}

		switch event.Rune() {
		case 'j', 'J':
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 1) % 12
			ui.updateSettingsDisplay()
			return nil
		case 'k', 'K':
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 11) % 12
			ui.updateSettingsDisplay()
			return nil
		case 'h', 'H':
			if ui.settingsFocusedIndex == 11 {
				ui.settingsFocusedIndex = 10
				ui.updateSettingsDisplay()
				return nil
			}
		case 'l', 'L':
			if ui.settingsFocusedIndex == 10 {
				ui.settingsFocusedIndex = 11
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
				input.SetBorderColor(ui.theme.EditingBorderColor)
			} else {
				input.SetBorderColor(ui.theme.FocusedBorderColor)
			}
		} else {
			input.SetBorderColor(ui.theme.NeutralGrayColor)
		}
	}

	// Disable AUR Checkbox styling
	if ui.settingsFocusedIndex == 8 {
		ui.settingAurCb.SetFieldBackgroundColor(ui.theme.SettingsFieldFocusedBg)
		ui.settingAurCb.SetFieldTextColor(ui.theme.SettingsFieldFocusedFg)
	} else {
		ui.settingAurCb.SetFieldBackgroundColor(tcell.ColorBlack)
		ui.settingAurCb.SetFieldTextColor(ui.theme.SelectedTextColor)
	}

	// Run Update Hooks Checkbox styling
	if ui.settingsFocusedIndex == 9 {
		ui.settingHooksCb.SetFieldBackgroundColor(ui.theme.SettingsFieldFocusedBg)
		ui.settingHooksCb.SetFieldTextColor(ui.theme.SettingsFieldFocusedFg)
	} else {
		ui.settingHooksCb.SetFieldBackgroundColor(tcell.ColorBlack)
		ui.settingHooksCb.SetFieldTextColor(ui.theme.SelectedTextColor)
	}

	// Save button styling
	if ui.settingsFocusedIndex == 10 {
		ui.btnSave.SetTextColor(tcell.ColorDefault)
		ui.btnSave.SetBackgroundColor(ui.theme.PrimaryColor)
		ui.btnSave.SetText("Apply & Save")
	} else {
		ui.btnSave.SetTextColor(ui.theme.SelectedTextColor)
		ui.btnSave.SetBackgroundColor(ui.theme.NeutralGrayColor)
		ui.btnSave.SetText("Apply & Save")
	}

	// Defaults button styling
	if ui.settingsFocusedIndex == 11 {
		ui.btnDefaults.SetTextColor(tcell.ColorDefault)
		ui.btnDefaults.SetBackgroundColor(ui.theme.PrimaryColor)
		ui.btnDefaults.SetText("Defaults")
	} else {
		ui.btnDefaults.SetTextColor(ui.theme.SelectedTextColor)
		ui.btnDefaults.SetBackgroundColor(ui.theme.NeutralGrayColor)
		ui.btnDefaults.SetText("Defaults")
	}
}


func (ui *UI) handleSettingsSelect() {
	if ui.settingsFocusedIndex >= 0 && ui.settingsFocusedIndex < 8 {
		ui.settingsEditMode = true
		ui.updateSettingsDisplay()
		ui.app.SetFocus(ui.settingInputs[ui.settingsFocusedIndex])
	} else if ui.settingsFocusedIndex == 8 {
		ui.settingAurCb.SetChecked(!ui.settingAurCb.IsChecked())
		ui.updateSettingsDisplay()
	} else if ui.settingsFocusedIndex == 9 {
		ui.settingHooksCb.SetChecked(!ui.settingHooksCb.IsChecked())
		ui.updateSettingsDisplay()
	} else if ui.settingsFocusedIndex == 10 {
		ui.saveSettingsAction()
	} else if ui.settingsFocusedIndex == 11 {
		ui.loadSettingsDefaults()
	}
}


func (ui *UI) saveSettingsAction() {
	ui.conf.PackagesPath = ui.settingInputs[0].GetText()
	ui.conf.PackagesFile = ui.settingInputs[1].GetText()
	ui.conf.PacmanDBPath = ui.settingInputs[2].GetText()
	ui.conf.PacmanConfigPath = ui.settingInputs[3].GetText()
	ui.conf.InstallCommand = ui.settingInputs[4].GetText()
	ui.conf.UninstallCommand = ui.settingInputs[5].GetText()
	ui.conf.SysUpgradeCmd = ui.settingInputs[6].GetText()

	maxRes, err := strconv.Atoi(ui.settingInputs[7].GetText())
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


func (ui *UI) loadSettingsDefaults() {
	ui.conf = config.Defaults()
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
	ui.setStatus("Settings reset to defaults!")
}
