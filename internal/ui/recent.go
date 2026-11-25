package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Recent projects are remembered in a small JSON file under the user's config
// directory so the welcome window can list them. Kept in the UI layer (not
// internal/svn) because it is pure presentation state.

const recentCap = 12

var recentMu sync.Mutex

func recentFile() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gosvn", "recent.json"), nil
}

func readRecentLocked() []string {
	f, err := recentFile()
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(f)
	if err != nil {
		return nil
	}
	var out []string
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

func writeRecentLocked(list []string) {
	f, err := recentFile()
	if err != nil {
		return
	}
	b, err := json.Marshal(list)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(f), 0o755)
	_ = os.WriteFile(f, b, 0o644)
}

// loadRecent returns the remembered working-copy paths, newest first.
func loadRecent() []string {
	recentMu.Lock()
	defer recentMu.Unlock()
	return readRecentLocked()
}

// addRecent moves p to the front of the recent list (deduplicated, capped).
func addRecent(p string) {
	if p == "" {
		return
	}
	recentMu.Lock()
	defer recentMu.Unlock()
	out := []string{p}
	for _, x := range readRecentLocked() {
		if x != p {
			out = append(out, x)
		}
	}
	if len(out) > recentCap {
		out = out[:recentCap]
	}
	writeRecentLocked(out)
}

// removeRecent drops p from the recent list.
func removeRecent(p string) {
	recentMu.Lock()
	defer recentMu.Unlock()
	var out []string
	for _, x := range readRecentLocked() {
		if x != p {
			out = append(out, x)
		}
	}
	writeRecentLocked(out)
}
