// Package tui manages the Terminal User Interface, layouts, user input, and screen rendering.
package tui

import (
	"fmt"
	"os"
	"os/exec"

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

func (ui *UI) getMaintenanceTasks() []maintenanceTask {
	var tasks []maintenanceTask

	tasks = append(tasks, maintenanceTask{
		Name:         "Clean Pacman Cache (Keep 3 versions)",
		Description:  "Runs 'paccache -r' to remove cached packages that are older than the 3 most recent versions. Safe and recommended to reclaim space.",
		Command:      "paccache -r",
		RequiresSudo: false,
	})
	tasks = append(tasks, maintenanceTask{
		Name:         "Clean Pacman Cache (Remove uninstalled)",
		Description:  "Runs 'pacman -Sc' to remove cached package files of uninstalled packages. Prompts for confirmation.",
		Command:      "sudo pacman -Sc",
		RequiresSudo: true,
	})
	tasks = append(tasks, maintenanceTask{
		Name:         "Clean Pacman Cache (Remove all)",
		Description:  "Runs 'pacman -Scc' to completely empty the pacman package cache. Reclaims maximum space, but removes all local package fallback versions.",
		Command:      "sudo pacman -Scc",
		RequiresSudo: true,
	})

	switch ui.aurHelper {
	case "yay":
		tasks = append(tasks, maintenanceTask{
			Name:         "Clean Yay Cache (AUR)",
			Description:  "Runs 'yay -Sc' to clean the build cache and package cache of the yay AUR helper. Prompts for confirmation.",
			Command:      "yay -Sc",
			RequiresSudo: false,
		})
		tasks = append(tasks, maintenanceTask{
			Name:         "Remove Yay Build Directories",
			Description:  "Removes directories under ~/.cache/yay to free up storage space from AUR builds.",
			Command:      "rm -rf ~/.cache/yay",
			RequiresSudo: false,
		})
	case "paru":
		tasks = append(tasks, maintenanceTask{
			Name:         "Clean Paru Cache (AUR)",
			Description:  "Runs 'paru -Sc' to clean the build cache and package cache of the paru AUR helper. Prompts for confirmation.",
			Command:      "paru -Sc",
			RequiresSudo: false,
		})
		tasks = append(tasks, maintenanceTask{
			Name:         "Remove Paru Build Directories",
			Description:  "Removes directories under ~/.cache/paru to free up storage space from AUR builds.",
			Command:      "rm -rf ~/.cache/paru",
			RequiresSudo: false,
		})
	}

	tasks = append(tasks, maintenanceTask{
		Name:         "Fix Pacman Database Lock File",
		Description:  "Removes '/var/lib/pacman/db.lck'. Run this if pacman or yay is locked due to a previous crash/interruption.",
		Command:      "sudo rm -f /var/lib/pacman/db.lck",
		RequiresSudo: true,
	})
	tasks = append(tasks, maintenanceTask{
		Name:         "Vacuum Systemd Journal Logs (Keep 50MB)",
		Description:  "Reduces systemd journal logs to a maximum size of 50MB. Safe way to reclaim disk space from old log files.",
		Command:      "sudo journalctl --vacuum-size=50M",
		RequiresSudo: true,
	})

	if ui.isCachyOS {
		tasks = append(tasks, maintenanceTask{
			Name:         "Update Package Mirrors (Benchmark)",
			Description:  "Updates package mirrors by benchmarking them using 'cachyrate-mirrors'.\n\nThis task does NOT run as sudo directly.",
			Command:      "cachyrate-mirrors",
			RequiresSudo: false,
		})
	} else {
		tasks = append(tasks, maintenanceTask{
			Name:         "Update Package Mirrors (Benchmark)",
			Description:  "Updates package mirrors by benchmarking them using 'rate-mirrors' and saving the results to the mirrorlist.",
			Command:      "rate-mirrors arch | sudo tee /etc/pacman.d/mirrorlist",
			RequiresSudo: false,
		})
	}

	return tasks
}


func (ui *UI) setupManagePage() tview.Primitive {
	tasks := ui.getMainMaintenanceTasks()

	ui.manageTable = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	ui.manageTable.SetSelectedStyle(tcell.StyleDefault.Background(ui.theme.PrimaryColor).Foreground(ui.theme.SelectedTextColor).Attributes(tcell.AttrBold))
	ui.manageTable.SetBorder(true).SetBorderColor(ui.theme.UnfocusedBorderColor).SetTitle(" Maintenance Tasks ")
	ui.setupFocusBorder(ui.manageTable)

	ui.manageDetails = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true)
	ui.manageDetails.SetBorder(true).SetBorderColor(ui.theme.UnfocusedBorderColor).SetTitle(" Description ")
	ui.setupFocusBorder(ui.manageDetails)

	ui.manageTable.SetSelectionChangedFunc(func(row, column int) {
		if row <= 0 || row > len(tasks) {
			ui.manageDetails.Clear()
			return
		}
		task := tasks[row-1]
		sudoStr := ""
		if task.RequiresSudo {
			sudoStr = "\n\n[red::b]Requires Administrator Privileges (sudo)[-]"
		}
		ui.manageDetails.SetText(fmt.Sprintf("[blue::b]%s[-]\n\n[yellow]Command:[-] %s%s\n\n%s", task.Name, task.Command, sudoStr, task.Description))
	})

	ui.manageTable.SetCell(0, 0, tview.NewTableCell("Task").SetTextColor(ui.theme.PrimaryColor).SetSelectable(false).SetExpansion(1))
	for i, task := range tasks {
		row := i + 1
		cell := tview.NewTableCell(task.Name).SetExpansion(1)
		if task.RequiresSudo {
			cell.SetTextColor(tcell.ColorRed)
		} else {
			cell.SetTextColor(tcell.ColorDefault)
		}
		ui.manageTable.SetCell(row, 0, cell)
	}

	if len(tasks) > 0 {
		ui.manageTable.Select(1, 0)
		task := tasks[0]
		sudoStr := ""
		if task.RequiresSudo {
			sudoStr = "\n\n[red::b]Requires Administrator Privileges (sudo)[-]"
		}
		ui.manageDetails.SetText(fmt.Sprintf("[blue::b]%s[-]\n\n[yellow]Command:[-] %s%s\n\n%s", task.Name, task.Command, sudoStr, task.Description))
	}

	ui.manageTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, _ := ui.manageTable.GetSelection()
		if event.Key() == tcell.KeyTAB {
			ui.app.SetFocus(ui.manageDetails)
			return nil
		}
		if event.Key() == tcell.KeyEnter {
			if row > 0 && row <= len(tasks) {
				ui.promptMaintenance(tasks[row-1])
			}
			return nil
		}
		return event
	})

	ui.manageDetails.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB || event.Key() == tcell.KeyBacktab {
			ui.app.SetFocus(ui.manageTable)
			return nil
		}
		return event
	})

	ui.manageFlex = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ui.manageTable, 0, 1, true).
		AddItem(ui.manageDetails, 0, 1, false)

	return ui.manageFlex
}


func (ui *UI) getMainMaintenanceTasks() []maintenanceTask {
	return ui.getMaintenanceTasks()
}


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
