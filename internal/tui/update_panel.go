package tui

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type UpdatePackage struct {
	Name         string
	LocalVersion string
	NewVersion   string
	Source       string
	Selected     bool
}

func (ui *UI) setupUpdatePage() {
	ui.updateTable = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	ui.updateTable.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite).Attributes(tcell.AttrBold))
	ui.updateTable.SetBorder(true).SetBorderColor(tcell.ColorDefault).SetTitle(" Updates ")

	ui.updateDetails = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true)
	ui.updateDetails.SetBorder(true).SetBorderColor(tcell.ColorDefault).SetTitle(" Details / PKGBUILD Changes ")

	ui.updateTable.SetFocusFunc(func() {
		ui.updateTable.SetBorderColor(tcell.ColorBlue)
	})
	ui.updateTable.SetBlurFunc(func() {
		ui.updateTable.SetBorderColor(tcell.ColorDefault)
	})

	ui.updateDetails.SetFocusFunc(func() {
		ui.updateDetails.SetBorderColor(tcell.ColorBlue)
	})
	ui.updateDetails.SetBlurFunc(func() {
		ui.updateDetails.SetBorderColor(tcell.ColorDefault)
	})

	ui.updateTable.SetSelectionChangedFunc(func(row, column int) {
		if row <= 0 || row > len(ui.updatePackages) {
			ui.selectedUpdate = nil
			ui.updateDetails.Clear()
			return
		}
		pkg := ui.updatePackages[row-1]
		ui.selectedUpdate = &pkg
		ui.loadUpdateDetails(pkg)
	})

	ui.updateTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, _ := ui.updateTable.GetSelection()
		if event.Key() == tcell.KeyTAB {
			ui.app.SetFocus(ui.updateDetails)
			return nil
		}
		if event.Key() == tcell.KeyRune && event.Rune() == ' ' {
			if row > 0 && row <= len(ui.updatePackages) {
				ui.togglePackageSelection(row - 1)
				ui.updateTable.Select(row, 0)
			}
			return nil
		}
		if event.Key() == tcell.KeyRune && (event.Rune() == 'a' || event.Rune() == 'A') {
			ui.toggleSelectAll()
			return nil
		}
		if event.Key() == tcell.KeyRune && (event.Rune() == 'u' || event.Rune() == 'U') {
			if row > 0 && row <= len(ui.updatePackages) {
				ui.runSingleUpgrade(ui.updatePackages[row-1])
			}
			return nil
		}
		if event.Key() == tcell.KeyEnter {
			ui.runUpgradeProcess()
			return nil
		}
		return event
	})

	ui.updateDetails.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTAB || event.Key() == tcell.KeyBacktab {
			ui.app.SetFocus(ui.updateTable)
			return nil
		}
		return event
	})

	ui.updatePageFlex = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ui.updateTable, 0, 1, true).
		AddItem(ui.updateDetails, 0, 1, false)
}

func (ui *UI) togglePackageSelection(index int) {
	if index < 0 || index >= len(ui.updatePackages) {
		return
	}
	ui.updatePackages[index].Selected = !ui.updatePackages[index].Selected
	ui.renderUpdateTable()
}

func (ui *UI) toggleSelectAll() {
	allSelected := true
	for _, p := range ui.updatePackages {
		if !p.Selected {
			allSelected = false
			break
		}
	}
	for i := range ui.updatePackages {
		ui.updatePackages[i].Selected = !allSelected
	}
	ui.renderUpdateTable()
}

func (ui *UI) renderUpdateTable() {
	ui.updateTable.Clear()

	// Header row
	ui.updateTable.SetCell(0, 0, tview.NewTableCell("").SetSelectable(false).SetMaxWidth(8))
	ui.updateTable.SetCell(0, 1, tview.NewTableCell("Package").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetExpansion(1))
	ui.updateTable.SetCell(0, 2, tview.NewTableCell("Current").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetMaxWidth(20))
	ui.updateTable.SetCell(0, 3, tview.NewTableCell("").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetMaxWidth(4))
	ui.updateTable.SetCell(0, 4, tview.NewTableCell("New").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetMaxWidth(20))
	ui.updateTable.SetCell(0, 5, tview.NewTableCell("Source").SetTextColor(tcell.ColorBlue).SetSelectable(false).SetMaxWidth(12))

	for idx, p := range ui.updatePackages {
		selStr := "   "
		if p.Selected {
			selStr = "  x"
		}
		selCell := tview.NewTableCell(selStr).SetMaxWidth(8).SetAlign(tview.AlignLeft)
		if p.Selected {
			selCell.SetTextColor(tcell.ColorGreen)
		}

		pkgCell := tview.NewTableCell(p.Name).SetExpansion(1)
		currCell := tview.NewTableCell(p.LocalVersion).SetMaxWidth(20)
		arrowCell := tview.NewTableCell("->").SetTextColor(tcell.ColorGray).SetMaxWidth(4)
		newCell := tview.NewTableCell(p.NewVersion).SetMaxWidth(20)

		if p.Selected {
			pkgCell.SetTextColor(tcell.ColorDefault)
			currCell.SetTextColor(tcell.ColorDefault)
			newCell.SetTextColor(tcell.ColorGreen)
		} else {
			pkgCell.SetTextColor(tcell.ColorGray)
			currCell.SetTextColor(tcell.ColorGray)
			newCell.SetTextColor(tcell.ColorGray)
		}

		sourceColor := getSourceColor(p.Source)
		if !p.Selected {
			sourceColor = tcell.ColorGray
		}
		sourceCell := tview.NewTableCell(p.Source).SetTextColor(sourceColor).SetMaxWidth(12)

		ui.updateTable.SetCell(idx+1, 0, selCell)
		ui.updateTable.SetCell(idx+1, 1, pkgCell)
		ui.updateTable.SetCell(idx+1, 2, currCell)
		ui.updateTable.SetCell(idx+1, 3, arrowCell)
		ui.updateTable.SetCell(idx+1, 4, newCell)
		ui.updateTable.SetCell(idx+1, 5, sourceCell)
	}
}

func (ui *UI) getPackageSource(pkgName string) string {
	ui.alpmMutex.Lock()
	defer ui.alpmMutex.Unlock()
	if ui.alpmHandle == nil {
		return "AUR"
	}
	dbs, err := ui.alpmHandle.SyncDBs()
	if err != nil {
		return "AUR"
	}
	for _, db := range dbs.Slice() {
		if db.Pkg(pkgName) != nil {
			return db.Name()
		}
	}
	return "AUR"
}

func ParseUpdateLine(line string) (*UpdatePackage, error) {
	// Format: <pkgname> <current_version> -> <new_version>
	parts := strings.Fields(line)
	if len(parts) < 4 || parts[2] != "->" {
		return nil, fmt.Errorf("invalid line format: %s", line)
	}
	return &UpdatePackage{
		Name:         parts[0],
		LocalVersion: parts[1],
		NewVersion:   parts[3],
		Selected:     true,
	}, nil
}

func (ui *UI) checkForUpdates() {
	if ui.updatePackages != nil {
		ui.renderUpdateTable()
		if len(ui.updatePackages) == 0 {
			ui.setStatus("System is up to date.")
			ui.updateDetails.SetText("All packages are up to date!")
		} else {
			ui.setStatus(fmt.Sprintf("Found %d updates (%d AUR).", len(ui.updatePackages), countAur(ui.updatePackages)))
		}
		return
	}

	ui.backgroundUpdateCheck()
}

func (ui *UI) backgroundUpdateCheck() {
	ui.setStatus("Checking for updates...")
	if ui.updateDetails != nil {
		ui.updateDetails.SetText("Updating databases and checking for updates in the background...")
	}

	go func() {
		// 1. Run checkupdates to get pacman updates
		cmdRepo := exec.Command("checkupdates")
		outRepo, _ := cmdRepo.Output()

		// 2. Run yay -Qua to get AUR updates
		cmdAur := exec.Command("yay", "-Qua")
		outAur, _ := cmdAur.Output()

		var pkgs []UpdatePackage

		// Parse AUR updates
		linesAur := strings.Split(string(outAur), "\n")
		for _, line := range linesAur {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			pkg, err := ParseUpdateLine(line)
			if err == nil {
				pkg.Source = "AUR"
				pkgs = append(pkgs, *pkg)
			}
		}

		// Parse Repo updates
		linesRepo := strings.Split(string(outRepo), "\n")
		for _, line := range linesRepo {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			pkg, err := ParseUpdateLine(line)
			if err == nil {
				pkg.Source = ui.getPackageSource(pkg.Name)
				if pkg.Source == "AUR" {
					pkg.Source = "repo"
				}
				pkgs = append(pkgs, *pkg)
			}
		}

		// Sort: AUR packages first, then alphabetical by name
		sort.Slice(pkgs, func(i, j int) bool {
			aAur := pkgs[i].Source == "AUR"
			bAur := pkgs[j].Source == "AUR"
			if aAur != bAur {
				return aAur
			}
			return pkgs[i].Name < pkgs[j].Name
		})

		ui.app.QueueUpdateDraw(func() {
			ui.updatePackages = pkgs
			if ui.updateTable != nil {
				ui.renderUpdateTable()
			}
			if len(pkgs) == 0 {
				ui.setStatus("System is up to date.")
				if ui.updateDetails != nil {
					ui.updateDetails.SetText("All packages are up to date!")
				}
			} else {
				ui.setStatus(fmt.Sprintf("Found %d updates (%d AUR).", len(pkgs), countAur(pkgs)))
				if ui.updateTable != nil && len(pkgs) > 0 {
					ui.updateTable.Select(1, 0)
				}
			}
		})

		// Reset Details Cache
		ui.cacheMutex.Lock()
		ui.updateDetailsCache = make(map[string]string)
		ui.cacheMutex.Unlock()

		// Preload details sequentially in the background, top-to-bottom
		go func() {
			for _, p := range pkgs {
				ui.preloadUpdateDetails(p)
				time.Sleep(300 * time.Millisecond)
			}
		}()
	}()
}

func countAur(pkgs []UpdatePackage) int {
	count := 0
	for _, p := range pkgs {
		if p.Source == "AUR" {
			count++
		}
	}
	return count
}

func (ui *UI) loadUpdateDetails(pkg UpdatePackage) {
	ui.cacheMutex.RLock()
	cachedText, exists := ui.updateDetailsCache[pkg.Name]
	ui.cacheMutex.RUnlock()

	if exists {
		ui.updateDetails.SetText(cachedText)
		ui.updateDetails.ScrollToBeginning()
		return
	}

	ui.updateDetails.SetText("Fetching details...")
	go ui.preloadUpdateDetails(pkg)
}

func (ui *UI) preloadUpdateDetails(pkg UpdatePackage) {
	ui.cacheMutex.RLock()
	_, exists := ui.updateDetailsCache[pkg.Name]
	ui.cacheMutex.RUnlock()
	if exists {
		return
	}

	var info SearchResults
	if pkg.Source == "AUR" {
		info = InfoAur("", 5000, pkg.Name)
	} else {
		ui.alpmMutex.Lock()
		info = InfoPacman(ui.alpmHandle, pkg.Name)
		ui.alpmMutex.Unlock()
	}

	var pkgBase string
	if len(info.Results) > 0 {
		pkgBase = info.Results[0].PackageBase
	}
	if pkgBase == "" {
		pkgBase = pkg.Name
	}

	var localPKGBUILD string
	var localPath string
	var remotePKGBUILD string

	if pkg.Source == "AUR" {
		home, err := os.UserHomeDir()
		if err == nil {
			localPath = filepath.Join(home, ".cache/yay", pkgBase, "PKGBUILD")
			data, err := os.ReadFile(localPath)
			if err == nil {
				localPKGBUILD = string(data)
			}
		}
	}

	// Fetch remote PKGBUILD
	remoteURL := GetPkgbuildURL(pkg.Source, pkgBase)
	if remoteURL != "" {
		remotePKGBUILD, _ = getPkgbuildContentWithTimeout(remoteURL, 5*time.Second)
	}

	var sb strings.Builder
	if len(info.Results) > 0 {
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

		for _, f := range fields {
			if f.value == "" {
				continue
			}
			if f.label == "Description" {
				fmt.Fprintf(&sb, "[blue]%s:[-]\n%s\n\n", f.label, f.value)
			} else {
				fmt.Fprintf(&sb, "[blue]%s:[-] %s\n", f.label, f.value)
			}
		}
	} else {
		fmt.Fprintf(&sb, "[blue]Package:[-] %s\n", pkg.Name)
		fmt.Fprintf(&sb, "[blue]Current Version:[-] %s\n", pkg.LocalVersion)
		fmt.Fprintf(&sb, "[blue]New Version:[-] %s\n", pkg.NewVersion)
		fmt.Fprintf(&sb, "[blue]Source:[-] %s\n\n", pkg.Source)
	}

	if pkg.Source == "AUR" {
		fmt.Fprintf(&sb, "\n[yellow]----------------- PKGBUILD Diff -----------------[-]\n")
		if remotePKGBUILD == "" {
			fmt.Fprintf(&sb, "[red]Failed to fetch remote PKGBUILD from AUR cgit.[-]\n")
		} else if localPKGBUILD == "" {
			fmt.Fprintf(&sb, "%s", formatDiff(remotePKGBUILD))
		} else {
			diffOut, err := runDiff(localPKGBUILD, remotePKGBUILD)
			if err != nil || diffOut == "" {
				fmt.Fprintf(&sb, "%s", formatDiff(remotePKGBUILD))
			} else {
				fmt.Fprintf(&sb, "%s", formatDiff(diffOut))
			}
		}
	} else {
		if remotePKGBUILD != "" {
			fmt.Fprintf(&sb, "\n[yellow]----------------- Remote PKGBUILD -----------------[-]\n")
			fmt.Fprintf(&sb, "%s", formatDiff(remotePKGBUILD))
		} else {
			fmt.Fprintf(&sb, "\n[gray]No PKGBUILD diff/content available for repository packages.[-]\n")
		}
	}

	ui.cacheMutex.Lock()
	ui.updateDetailsCache[pkg.Name] = sb.String()
	ui.cacheMutex.Unlock()

	ui.app.QueueUpdateDraw(func() {
		if ui.selectedUpdate != nil && ui.selectedUpdate.Name == pkg.Name {
			ui.updateDetails.SetText(sb.String())
			ui.updateDetails.ScrollToBeginning()
		}
	})
}

func getPkgbuildContentWithTimeout(url string, timeout time.Duration) (string, error) {
	client := http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func runDiff(localContent, remoteContent string) (string, error) {
	tmpLocal, err := os.CreateTemp("", "drxpkg-local-")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpLocal.Name())
	defer tmpLocal.Close()

	if _, err := tmpLocal.WriteString(localContent); err != nil {
		return "", err
	}

	tmpRemote, err := os.CreateTemp("", "drxpkg-remote-")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpRemote.Name())
	defer tmpRemote.Close()

	if _, err := tmpRemote.WriteString(remoteContent); err != nil {
		return "", err
	}

	cmd := exec.Command("diff", "-u", tmpLocal.Name(), tmpRemote.Name())
	output, _ := cmd.CombinedOutput()
	return string(output), nil
}

func formatDiff(diffText string) string {
	lines := strings.Split(diffText, "\n")
	for i, line := range lines {
		escaped := strings.ReplaceAll(line, "[", "[[")
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			lines[i] = "[yellow]" + escaped + "[-]"
		} else if strings.HasPrefix(line, "@@") {
			lines[i] = "[cyan]" + escaped + "[-]"
		} else if strings.HasPrefix(line, "-") {
			lines[i] = "[red]" + escaped + "[-]"
		} else if strings.HasPrefix(line, "+") {
			lines[i] = "[green]" + escaped + "[-]"
		} else {
			lines[i] = escaped
		}
	}
	return strings.Join(lines, "\n")
}

func (ui *UI) runUpgradeProcess() {
	var selectedCount int
	var ignoreList []string
	for _, p := range ui.updatePackages {
		if p.Selected {
			selectedCount++
		} else {
			ignoreList = append(ignoreList, p.Name)
		}
	}

	if len(ui.updatePackages) > 0 && selectedCount == 0 {
		ui.setStatus("[red]Error: No packages selected for upgrade.")
		return
	}

	ui.app.Suspend(func() {
		fmt.Print("\033[H\033[2J")
		fmt.Printf(" ➤ \033[1;34mStarting system package update...\033[0m\n")

		cmdStr := ui.conf.SysUpgradeCmd
		if cmdStr == "" {
			cmdStr = "yay"
		}

		args := []string{}
		parts := strings.Fields(cmdStr)
		binary := parts[0]

		if binary == "yay" {
			if len(parts) == 1 {
				args = append(args, "-Syu", "--noconfirm")
			} else {
				args = append(args, parts[1:]...)
				if !hasFlag(args, "--noconfirm") {
					args = append(args, "--noconfirm")
				}
			}
		} else if binary == "pacman" {
			args = append(args, parts[1:]...)
			if !hasFlag(args, "--noconfirm") {
				args = append(args, "--noconfirm")
			}
		} else {
			if len(parts) > 1 {
				args = append(args, parts[1:]...)
			}
		}

		if len(ignoreList) > 0 {
			args = append(args, "--ignore", strings.Join(ignoreList, ","))
		}

		// Ensure we run system upgrade command. If pacman, run via sudo
		var upgradeCmd *exec.Cmd
		if binary == "pacman" {
			upgradeCmd = exec.Command("sudo", append([]string{binary}, args...)...)
			fmt.Printf("Running: sudo %s %s\n\n", binary, strings.Join(args, " "))
		} else {
			upgradeCmd = exec.Command(binary, args...)
			fmt.Printf("Running: %s %s\n\n", binary, strings.Join(args, " "))
		}

		upgradeCmd.Stdin = os.Stdin
		upgradeCmd.Stdout = os.Stdout
		upgradeCmd.Stderr = os.Stderr

		err := upgradeCmd.Run()
		if err != nil {
			fmt.Printf("\n\033[1;31m[ERROR]\033[0m System upgrade failed: %v\n", err)
		} else {
			fmt.Printf("\n\033[1;32m[SUCCESS]\033[0m System upgrade completed successfully.\n")
		}

		ui.runExtraUpdates()

		fmt.Println("\nPress ENTER to return to drxpkg...")
		_, _ = os.Stdin.Read(make([]byte, 1))
	})

	_ = ui.reinitPacmanDbs()
	ui.updatePackages = nil
	ui.checkForUpdates()
}

func (ui *UI) runSingleUpgrade(pkg UpdatePackage) {
	cmdStr := ui.conf.InstallCommand
	if cmdStr == "" {
		cmdStr = "yay -S"
	}

	ui.app.Suspend(func() {
		fmt.Print("\033[H\033[2J")

		var fullCommand string
		if strings.Contains(cmdStr, "{pkg}") {
			fullCommand = strings.ReplaceAll(cmdStr, "{pkg}", pkg.Name)
		} else {
			fullCommand = cmdStr + " " + pkg.Name
		}

		parts := strings.Fields(fullCommand)
		binary := parts[0]
		args := parts[1:]
		if !hasFlag(args, "--noconfirm") {
			args = append(args, "--noconfirm")
		}

		var upgradeCmd *exec.Cmd
		if binary == "pacman" {
			upgradeCmd = exec.Command("sudo", append([]string{binary}, args...)...)
			fmt.Printf("Running single upgrade: sudo %s %s\n\n", binary, strings.Join(args, " "))
		} else {
			upgradeCmd = exec.Command(binary, args...)
			fmt.Printf("Running single upgrade: %s %s\n\n", binary, strings.Join(args, " "))
		}

		upgradeCmd.Stdin = os.Stdin
		upgradeCmd.Stdout = os.Stdout
		upgradeCmd.Stderr = os.Stderr

		err := upgradeCmd.Run()
		if err != nil {
			fmt.Printf("\n\033[1;31m[ERROR]\033[0m Upgrade failed for '%s': %v\n", pkg.Name, err)
		} else {
			fmt.Printf("\n\033[1;32m[SUCCESS]\033[0m Package '%s' upgraded successfully.\n", pkg.Name)
		}

		fmt.Println("\nPress ENTER to return to drxpkg...")
		_, _ = os.Stdin.Read(make([]byte, 1))
	})

	_ = ui.reinitPacmanDbs()
	ui.updatePackages = nil
	ui.checkForUpdates()
}

func (ui *UI) runExtraUpdates() {
	home, err := os.UserHomeDir()
	if err == nil {
		vencordPaths := []string{
			filepath.Join(home, ".config/Vencord"),
			filepath.Join(home, ".config/vencord"),
			filepath.Join(home, ".local/share/Vencord"),
			filepath.Join(home, ".local/share/vencord"),
		}
		vencordInstalled := false
		for _, p := range vencordPaths {
			if _, err := os.Stat(p); err == nil {
				vencordInstalled = true
				break
			}
		}

		if vencordInstalled {
			fmt.Printf("\n ➤ \033[1;34mUpdating Discord hooks/Vencord...\033[0m\n")
			cmd := exec.Command("sh", "-c", "curl -sS https://raw.githubusercontent.com/Vendicated/VencordInstaller/main/install.sh | sh")
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Printf("\033[1;31m[ERROR]\033[0m Failed to update Vencord: %v\n", err)
			} else {
				fmt.Printf("\033[1;32m[SUCCESS]\033[0m Vencord updated.\n")
			}
		} else {
			fmt.Printf("\nNo Vencord installation detected. Skipping Vencord update.\n")
		}
	}

	if _, err := exec.LookPath("ya"); err == nil {
		fmt.Printf("\n ➤ \033[1;34mUpdating Yazi plugins...\033[0m\n")
		cmd := exec.Command("ya", "pkg", "upgrade")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("\033[1;31m[ERROR]\033[0m Failed to update Yazi plugins: %v\n", err)
		} else {
			fmt.Printf("\033[1;32m[SUCCESS]\033[0m Yazi plugins updated.\n")
		}
	} else {
		fmt.Printf("\n'ya' tool not found. Skipping Yazi plugins update.\n")
	}

	fmt.Printf("\n ➤ \033[1;34mUpdating drxutils...\033[0m\n")
	if err := ui.updateDrxutils(); err != nil {
		fmt.Printf("\033[1;31m[ERROR]\033[0m Failed to update drxutils: %v\n", err)
	} else {
		fmt.Printf("\033[1;32m[SUCCESS]\033[0m drxutils updated successfully.\n")
	}
}

func (ui *UI) updateDrxutils() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	repositoryURL := "https://github.com/druxorey/drxutils.git"
	srcDirectory := filepath.Join(home, ".cache/drxutils_src")
	binDirectory := filepath.Join(home, ".local/bin")

	if err := os.MkdirAll(binDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	var changedFiles []string
	var deletedFiles []string

	if _, err := os.Stat(srcDirectory); os.IsNotExist(err) {
		fmt.Println("Cloning drxutils repository...")
		cmd := exec.Command("git", "clone", repositoryURL, srcDirectory, "--quiet")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}

		cmdList := exec.Command("git", "ls-files")
		cmdList.Dir = srcDirectory
		out, err := cmdList.Output()
		if err != nil {
			return fmt.Errorf("failed to list git files: %w", err)
		}
		changedFiles = splitLines(string(out))
	} else {
		fmt.Println("Fetching drxutils updates...")
		cmdFetch := exec.Command("git", "fetch", "origin", "--quiet")
		cmdFetch.Dir = srcDirectory
		if err := cmdFetch.Run(); err != nil {
			return fmt.Errorf("git fetch failed: %w", err)
		}

		cmdBranch := exec.Command("git", "branch", "--show-current")
		cmdBranch.Dir = srcDirectory
		branchOut, err := cmdBranch.Output()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		branch := strings.TrimSpace(string(branchOut))
		if branch == "" {
			branch = "main"
		}

		cmdDel := exec.Command("git", "diff", "--name-only", "--diff-filter=D", "HEAD", "origin/"+branch)
		cmdDel.Dir = srcDirectory
		delOut, _ := cmdDel.Output()
		deletedFiles = splitLines(string(delOut))

		cmdChg := exec.Command("git", "diff", "--name-only", "--diff-filter=AM", "HEAD", "origin/"+branch)
		cmdChg.Dir = srcDirectory
		chgOut, _ := cmdChg.Output()
		changedFiles = splitLines(string(chgOut))

		cmdReset := exec.Command("git", "reset", "--hard", "origin/"+branch, "--quiet")
		cmdReset.Dir = srcDirectory
		if err := cmdReset.Run(); err != nil {
			return fmt.Errorf("git reset failed: %w", err)
		}
	}

	for _, file := range deletedFiles {
		if strings.HasPrefix(file, "bash/") {
			scriptName := strings.TrimPrefix(file, "bash/")
			scriptPath := filepath.Join(binDirectory, scriptName)
			_ = os.Remove(scriptPath)
			fmt.Printf("Bash script removed locally: %s\n", scriptName)
		} else if strings.Contains(file, "/") {
			parts := strings.Split(file, "/")
			dirName := parts[0]
			projectPath := filepath.Join(binDirectory, dirName)
			_ = os.Remove(projectPath)
			fmt.Printf("Go project removed locally: %s\n", dirName)
		}
	}

	bashDir := filepath.Join(srcDirectory, "bash")
	if entries, err := os.ReadDir(bashDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				srcFile := filepath.Join(bashDir, entry.Name())
				dstFile := filepath.Join(binDirectory, entry.Name())
				if err := copyFile(srcFile, dstFile); err == nil {
					_ = os.Chmod(dstFile, 0755)
				}
			}
		}
		fmt.Println("All bash scripts copied to " + binDirectory)
	}

	if len(changedFiles) > 0 {
		goDirsToCompile := make(map[string]bool)
		for _, file := range changedFiles {
			if strings.Contains(file, "/") {
				parts := strings.Split(file, "/")
				dirName := parts[0]
				if dirName != "bash" {
					goModPath := filepath.Join(srcDirectory, dirName, "go.mod")
					if _, err := os.Stat(goModPath); err == nil {
						goDirsToCompile[dirName] = true
					}
				}
			}
		}

		for dirName := range goDirsToCompile {
			fmt.Printf("Compiling Go project: %s...\n", dirName)
			projectDir := filepath.Join(srcDirectory, dirName)
			outBin := filepath.Join(binDirectory, dirName)
			cmdBuild := exec.Command("go", "build", "-o", outBin)
			cmdBuild.Dir = projectDir
			cmdBuild.Stdout = os.Stdout
			cmdBuild.Stderr = os.Stderr
			if err := cmdBuild.Run(); err != nil {
				fmt.Printf("[ERROR] Compiling failed for: %s: %v\n", dirName, err)
			} else {
				fmt.Printf("[SUCCESS] Installed: %s\n", dirName)
			}
		}
	} else {
		fmt.Println("Go projects are up to date.")
	}

	return nil
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
