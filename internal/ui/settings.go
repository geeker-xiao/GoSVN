package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Settings is the user's persisted application configuration.
type Settings struct {
	ThemeMode string `json:"themeMode"` // "system" | "light" | "dark"
	SVNBin    string `json:"svnBin"`    // path to svn executable, empty = PATH lookup
	Timeout   int    `json:"timeout"`   // default command timeout in seconds, 0 = none
}

var (
	settingsMu sync.RWMutex
	settings   = Settings{ThemeMode: "system"}
)

func init() {
	loadSettings()
}

func settingsFile() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gosvn", "settings.json"), nil
}

func loadSettings() {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	f, err := settingsFile()
	if err != nil {
		return
	}
	b, err := os.ReadFile(f)
	if err != nil {
		return
	}
	var s Settings
	if err := json.Unmarshal(b, &s); err == nil {
		if s.ThemeMode == "" {
			s.ThemeMode = "system"
		}
		settings = s
	}
}

func saveSettings(s Settings) error {
	if s.ThemeMode == "" {
		s.ThemeMode = "system"
	}
	f, err := settingsFile()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(f, b, 0o644); err != nil {
		return err
	}
	settingsMu.Lock()
	settings = s
	settingsMu.Unlock()
	return nil
}

// GetSettings returns a copy of the current in-memory settings.
func GetSettings() Settings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return settings
}
