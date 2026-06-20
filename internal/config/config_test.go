package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadSave(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	s, err := Load()
	if err != nil {
		t.Fatalf("expected no error loading config: %v", err)
	}

	if s.InstallCommand != "yay -S" {
		t.Errorf("expected default InstallCommand to be 'yay -S', got '%s'", s.InstallCommand)
	}
	if !s.RunUpdateHooks {
		t.Errorf("expected default RunUpdateHooks to be true")
	}

	s.InstallCommand = "sudo pacman -S"
	s.MaxResults = 1000
	s.RunUpdateHooks = false
	if err := s.Save(); err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}

	expectedFile := filepath.Join(tempDir, "drxpkg", "config.json")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Fatalf("expected config file to exist at %s", expectedFile)
	}

	expectedHooksDir := filepath.Join(tempDir, "drxpkg", "update_hooks")
	if fi, err := os.Stat(expectedHooksDir); os.IsNotExist(err) || !fi.IsDir() {
		t.Fatalf("expected update_hooks directory to exist at %s", expectedHooksDir)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("failed to load modified settings: %v", err)
	}

	if loaded.InstallCommand != "sudo pacman -S" {
		t.Errorf("expected loaded InstallCommand to be 'sudo pacman -S', got '%s'", loaded.InstallCommand)
	}
	if loaded.MaxResults != 1000 {
		t.Errorf("expected loaded MaxResults to be 1000, got %d", loaded.MaxResults)
	}
	if loaded.RunUpdateHooks {
		t.Errorf("expected loaded RunUpdateHooks to be false")
	}
}
