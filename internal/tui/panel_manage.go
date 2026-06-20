// Package tui manages the Terminal User Interface, layouts, user input, and screen rendering.
package tui

import (
	"github.com/rivo/tview"
)

func (ui *UI) setupManagePage() *tview.TextView {
	return tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("\n\n[blue]Package Management[-]\n\nThis page is currently a placeholder.")
}
