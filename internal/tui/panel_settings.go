// Package tui manages the Terminal User Interface, layouts, user input, and screen rendering.
package tui

import (
	"strconv"

	"github.com/druxorey/drxpkg/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

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

	// Initialize checkboxes
	ui.settingAurCb = tview.NewCheckbox().SetLabel("").SetChecked(ui.conf.DisableAur)
	ui.settingAurCb.SetFocusFunc(func() {
		ui.settingsFocusedIndex = 7
		ui.updateSettingsDisplay()
	})

	ui.settingHooksCb = tview.NewCheckbox().SetLabel("").SetChecked(ui.conf.RunUpdateHooks)
	ui.settingHooksCb.SetFocusFunc(func() {
		ui.settingsFocusedIndex = 8
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

	// Fields Grid: 7 inputs (height 3 each), 2 checkboxes (height 1 each)
	fieldsGrid := tview.NewGrid().
		SetRows(3, 3, 3, 3, 3, 3, 3, 1, 1).
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
		"Run Update Hooks",
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
		} else if i == 7 {
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

	settingsBox.
		AddItem(fieldsGrid, 0, 1, false).
		AddItem(nil, 1, 0, false). // spacer
		AddItem(buttonsFlex, 2, 0, false).
		AddItem(nil, 1, 0, false) // spacer

	// Center the settingsBox using a grid
	ui.settingsGrid = tview.NewGrid().
		SetRows(0, 30, 0).
		SetColumns(0, 75, 0).
		AddItem(settingsBox, 1, 1, 1, 1, 0, 0, true)

	// Set input capture on settingsGrid for navigation (11 indices: 0..10)
	ui.settingsGrid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if ui.settingsEditMode {
			return event
		}

		switch event.Key() {
		case tcell.KeyUp:
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 10) % 11
			ui.updateSettingsDisplay()
			return nil
		case tcell.KeyDown:
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 1) % 11
			ui.updateSettingsDisplay()
			return nil
		case tcell.KeyLeft:
			if ui.settingsFocusedIndex == 10 {
				ui.settingsFocusedIndex = 9
				ui.updateSettingsDisplay()
				return nil
			}
		case tcell.KeyRight:
			if ui.settingsFocusedIndex == 9 {
				ui.settingsFocusedIndex = 10
				ui.updateSettingsDisplay()
				return nil
			}
		case tcell.KeyTAB:
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 1) % 11
			ui.updateSettingsDisplay()
			return nil
		case tcell.KeyBacktab:
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 10) % 11
			ui.updateSettingsDisplay()
			return nil
		case tcell.KeyEnter:
			ui.handleSettingsSelect()
			return nil
		}

		switch event.Rune() {
		case 'j', 'J':
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 1) % 11
			ui.updateSettingsDisplay()
			return nil
		case 'k', 'K':
			ui.settingsFocusedIndex = (ui.settingsFocusedIndex + 10) % 11
			ui.updateSettingsDisplay()
			return nil
		case 'h', 'H':
			if ui.settingsFocusedIndex == 10 {
				ui.settingsFocusedIndex = 9
				ui.updateSettingsDisplay()
				return nil
			}
		case 'l', 'L':
			if ui.settingsFocusedIndex == 9 {
				ui.settingsFocusedIndex = 10
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

	// Disable AUR Checkbox styling
	if ui.settingsFocusedIndex == 7 {
		ui.settingAurCb.SetFieldBackgroundColor(tcell.ColorYellow)
		ui.settingAurCb.SetFieldTextColor(tcell.ColorBlack)
	} else {
		ui.settingAurCb.SetFieldBackgroundColor(tcell.ColorDefault)
		ui.settingAurCb.SetFieldTextColor(tcell.ColorWhite)
	}

	// Run Update Hooks Checkbox styling
	if ui.settingsFocusedIndex == 8 {
		ui.settingHooksCb.SetFieldBackgroundColor(tcell.ColorYellow)
		ui.settingHooksCb.SetFieldTextColor(tcell.ColorBlack)
	} else {
		ui.settingHooksCb.SetFieldBackgroundColor(tcell.ColorDefault)
		ui.settingHooksCb.SetFieldTextColor(tcell.ColorWhite)
	}

	// Save button styling
	if ui.settingsFocusedIndex == 9 {
		ui.btnSave.SetTextColor(tcell.ColorDefault)
		ui.btnSave.SetBackgroundColor(tcell.ColorBlue)
		ui.btnSave.SetText("Apply & Save")
	} else {
		ui.btnSave.SetTextColor(tcell.ColorWhite)
		ui.btnSave.SetBackgroundColor(tcell.ColorGray)
		ui.btnSave.SetText("Apply & Save")
	}

	// Defaults button styling
	if ui.settingsFocusedIndex == 10 {
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
		ui.settingHooksCb.SetChecked(!ui.settingHooksCb.IsChecked())
		ui.updateSettingsDisplay()
	} else if ui.settingsFocusedIndex == 9 {
		ui.saveSettingsAction()
	} else if ui.settingsFocusedIndex == 10 {
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
	ui.conf.RunUpdateHooks = ui.settingHooksCb.IsChecked()

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
	ui.settingHooksCb.SetChecked(ui.conf.RunUpdateHooks)
	ui.setStatus("Settings reset to defaults!")
}
