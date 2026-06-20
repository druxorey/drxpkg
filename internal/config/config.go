// Package config does somethin
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	PackagesPath     string `json:"packages_path"`
	PacmanDBPath     string `json:"pacman_db_path"`
	PacmanConfigPath string `json:"pacman_config_path"`
	InstallCommand   string `json:"install_command"`
	UninstallCommand string `json:"uninstall_command"`
	SysUpgradeCmd    string `json:"sys_upgrade_cmd"`
	MaxResults       int    `json:"max_results"`
	DisableAur       bool   `json:"disable_aur"`
	RunUpdateHooks   bool   `json:"run_update_hooks"`
}

func Defaults() *Settings {
	return &Settings{
		PackagesPath:     "$HOME/.local/share/",
		PacmanDBPath:     "/var/lib/pacman/",
		PacmanConfigPath: "/etc/pacman.conf",
		InstallCommand:   "yay -S",
		UninstallCommand: "yay -Rs",
		SysUpgradeCmd:    "yay",
		MaxResults:       300,
		DisableAur:       false,
		RunUpdateHooks:   true,
	}
}

func GetConfigDir() (string, error) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(userConfigDir, "drxpkg"), nil
}

func Load() (*Settings, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return Defaults(), err
	}
	file := filepath.Join(dir, "config.json")
	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return Defaults(), nil
		}
		return Defaults(), err
	}
	s := Defaults()
	if err := json.Unmarshal(data, s); err != nil {
		return Defaults(), err
	}
	return s, nil
}

func (s *Settings) Save() error {
	dir, err := GetConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	hooksDir := filepath.Join(dir, "update_hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	file := filepath.Join(dir, "config.json")
	return os.WriteFile(file, data, 0644)
}
