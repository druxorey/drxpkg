package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/druxorey/drxpkg/internal/config"
	"github.com/druxorey/drxpkg/internal/util"
)

// runHooks executes bash scripts located in config_dir/hooks/<hooksSubDirName>.
func (ui *UI) runHooks(hooksSubDirName string, isPre bool) {
	if !ui.conf.RunUpdateHooks {
		return
	}

	configDir, err := config.GetConfigDir()
	if err != nil {
		util.PrintError("\nFailed to get config directory: %v\n", err)
		return
	}

	hooksDir := filepath.Join(configDir, "hooks", hooksSubDirName)
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		util.PrintError("\nFailed to create hooks directory %s: %v\n", hooksDir, err)
		return
	}

	files, err := os.ReadDir(hooksDir)
	if err != nil {
		util.PrintError("\nFailed to read hooks directory: %v\n", err)
		return
	}

	var hookFiles []string
	for _, file := range files {
		if !file.IsDir() {
			name := file.Name()
			isPrev := strings.HasPrefix(name, "prev_")
			
			if isPre && isPrev {
				hookFiles = append(hookFiles, name)
			} else if !isPre && !isPrev {
				hookFiles = append(hookFiles, name)
			}
		}
	}

	if len(hookFiles) == 0 {
		return
	}

	// Sort alphabetically to run sequentially
	sort.Strings(hookFiles)

	phase := "post"
	if isPre {
		phase = "pre"
	}
	fmt.Printf("\n ➤ \033[1;34mRunning %s-%s hooks...\033[0m\n", phase, hooksSubDirName)
	for _, filename := range hookFiles {
		scriptPath := filepath.Join(hooksDir, filename)
		fmt.Printf("\n ➤ \033[1;34mExecuting hook: %s...\033[0m\n", filename)

		cmd := exec.Command("bash", scriptPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			util.PrintError("Hook '%s' failed: %v\n", filename, err)
		} else {
			util.PrintSuccess("Hook '%s' completed.\n", filename)
		}
	}
}
