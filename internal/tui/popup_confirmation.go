package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *UI) showConfirmation(message string, onConfirm func()) {
	ui.confirmationFocusedIndex = 0
	prevFocus := ui.app.GetFocus()
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			ui.pages.RemovePage("confirmation")
			ui.app.SetFocus(prevFocus)
			if buttonLabel == "Yes" {
				onConfirm()
			}
		})
	modal.SetBackgroundColor(tcell.ColorBlack)
	modal.SetTextColor(tcell.ColorDefault)
	modal.SetButtonBackgroundColor(ui.theme.SelectedTextColor)
	modal.SetButtonTextColor(ui.theme.PrimaryColor)

	ui.pages.AddPage("confirmation", modal, true, true)
}


func (ui *UI) showAlert(message string) {
	prevFocus := ui.app.GetFocus()
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			ui.pages.RemovePage("alert")
			ui.app.SetFocus(prevFocus)
		})
	modal.SetBackgroundColor(tcell.ColorBlack)
	modal.SetTextColor(tcell.ColorDefault)
	modal.SetButtonBackgroundColor(ui.theme.SelectedTextColor)
	modal.SetButtonTextColor(ui.theme.PrimaryColor)

	ui.pages.AddPage("alert", modal, true, true)
}

