package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)


func (ui *UI) setupHelpPopup() {
	helpText := `[yellow]Global Shortcuts:[-]
  [green]ESC[-]          Exit application / Close active popup
  [green].[-]            Open Settings popup
  [green]?[-]            Open Help popup
  [green][[-], [green]][-]        Switch between tabs
  [green]F1[-] - [green]F3[-]      Switch tab directly
  [green]F4[-]            Open Settings popup

[yellow]Install Tab:[-]
  [green]TAB[-]          Toggle focus between search and table
  [green]Space[-]        Toggle package selection
  [green]v[-] / [green]V[-]        Toggle visual mode for range selection
  [green]i[-] / [green]I[-]        Install selected package(s)
  [green]u[-] / [green]U[-]        Uninstall selected package(s)
  [green]gg[-]           Go to beginning of table
  [green]G[-]            Go to end of table

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

	ui.helpGrid = tview.NewGrid().
		SetRows(0, 26, 0).
		SetColumns(0, 80, 0).
		AddItem(helpFrame, 1, 1, 1, 1, 0, 0, true)

	ui.helpGrid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyEnter || event.Rune() == ' ' || event.Rune() == '?' {
			ui.closeHelpPopup()
			return nil
		}
		return event
	})
}


func (ui *UI) showHelpPopup() {
	ui.helpPopupOpen = true
	ui.pages.ShowPage("help")
	ui.app.SetFocus(ui.helpGrid)
}


func (ui *UI) closeHelpPopup() {
	ui.helpPopupOpen = false
	ui.pages.HidePage("help")
	ui.restoreFocusToActiveTab()
}
