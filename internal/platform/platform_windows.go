//go:build windows

package platform

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// menuEntries describes the right-click entries to (un)install. Each installs
// under a registry root path; %1 / %V are filled by Explorer with the target.
type menuEntry struct {
	keyPath string // relative to HKCU
	name    string // display label
	arg     string // subcommand passed to gosvn
	target  string // "%1" for files/dirs, "%V" for folder background
}

func menuEntries() []menuEntry {
	return []menuEntry{
		{`Software\Classes\Directory\shell\GoSVN-Commit`, "GoSVN Commit…", "commit", `"%1"`},
		{`Software\Classes\Directory\shell\GoSVN-Update`, "GoSVN Update…", "update", `"%1"`},
		{`Software\Classes\Directory\shell\GoSVN-Log`, "GoSVN Show Log…", "log", `"%1"`},
		{`Software\Classes\*\shell\GoSVN-Commit`, "GoSVN Commit…", "commit", `"%1"`},
		{`Software\Classes\*\shell\GoSVN-Diff`, "GoSVN Diff…", "diff", `"%1"`},
		{`Software\Classes\Directory\Background\shell\GoSVN-Commit`, "GoSVN Commit…", "commit", `"%V"`},
		{`Software\Classes\Directory\Background\shell\GoSVN-Update`, "GoSVN Update…", "update", `"%V"`},
		{`Software\Classes\Directory\Background\shell\GoSVN-Log`, "GoSVN Show Log…", "log", `"%V"`},
	}
}

func gosvnCommand(arg, target string) string {
	return fmt.Sprintf(`%s %s %s`, Quote(Executable()), arg, target)
}

func createMenu(entry menuEntry) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, entry.keyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create %s: %w", entry.keyPath, err)
	}
	if err := k.SetStringValue("", entry.name); err != nil {
		k.Close()
		return fmt.Errorf("set name %s: %w", entry.keyPath, err)
	}
	cmd, _, err := registry.CreateKey(k, "command", registry.SET_VALUE)
	k.Close()
	if err != nil {
		return fmt.Errorf("create command %s: %w", entry.keyPath, err)
	}
	defer cmd.Close()
	if err := cmd.SetStringValue("", gosvnCommand(entry.arg, entry.target)); err != nil {
		return fmt.Errorf("set command %s: %w", entry.keyPath, err)
	}
	return nil
}

func deleteKeyTree(root registry.Key, relPath string) error {
	k, err := registry.OpenKey(root, relPath, registry.READ)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	names, _ := k.ReadSubKeyNames(-1)
	k.Close()
	for _, n := range names {
		child := relPath + `\` + n
		if err := deleteKeyTree(root, child); err != nil {
			return err
		}
	}
	return registry.DeleteKey(root, relPath)
}

// InstallIntegration registers GoSVN entries in the current user's Explorer
// right-click menus (no administrator rights needed).
func InstallIntegration() error {
	if Executable() == "" {
		return errors.New("cannot determine executable path")
	}
	for _, e := range menuEntries() {
		if err := createMenu(e); err != nil {
			return err
		}
	}
	return nil
}

// UninstallIntegration removes the GoSVN right-click entries installed by
// InstallIntegration.
func UninstallIntegration() error {
	for _, e := range menuEntries() {
		if err := deleteKeyTree(registry.CURRENT_USER, e.keyPath); err != nil {
			return err
		}
	}
	return nil
}
