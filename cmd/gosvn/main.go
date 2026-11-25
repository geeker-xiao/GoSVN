// Command gosvn is a native (no WebView) desktop SVN client built with MyGo.
//
// Usage:
//
//	gosvn [command] [path]
//
// Commands:
//
//	(no command)   open the project picker (or the main window if PATH given)
//	commit         open the commit window for path (alias: status, st)
//	update         open the update window for path
//	log            open the log window for path
//	diff           open the diff window for path
//	install        register right-click integration (Windows Explorer / Nautilus)
//	uninstall      remove right-click integration
//	help, -h       show this help
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/egoist/mygo"
	"github.com/zoo/gosvn/internal/platform"
	"github.com/zoo/gosvn/internal/svn"
	"github.com/zoo/gosvn/internal/ui"
)

const version = "0.1.0"

const usage = `GoSVN - a native desktop SVN client (no WebView)

Usage:
  gosvn [command] [path]

Commands:
  (default)   open the project picker; gosvn PATH opens the main window directly
  commit      open the commit window for path (aliases: status, st)
  update      open the update window for path
  log         open the log window for path
  diff        open the diff window for path
  checkout    open the checkout dialog (check out a remote repository)
  create-repo open the create-local-repository dialog
  remote      open the remote repository browser
  settings    open the preferences window
  install     register right-click integration (Explorer / Nautilus)
  uninstall   remove right-click integration
  help, -h    show this help
  version     print version

With no arguments the project picker opens. Subcommands default PATH to ".".
`

func main() {
	args := os.Args[1:]
	cmd := ""
	path := "."
	hasPath := false
	if len(args) > 0 {
		cmd = args[0]
		if len(args) > 1 {
			path = args[1]
			hasPath = true
		}
	}

	switch cmd {
	case "", "main":
		// fall through to GUI
	case "help", "-h", "--help":
		fmt.Print(usage)
		return
	case "version", "--version", "-v":
		fmt.Printf("gosvn %s\n", version)
		return
	case "install":
		if err := platform.InstallIntegration(); err != nil {
			log.Fatalf("install integration failed: %v", err)
		}
		fmt.Println("GoSVN right-click integration installed.")
		return
	case "uninstall":
		if err := platform.UninstallIntegration(); err != nil {
			log.Fatalf("uninstall integration failed: %v", err)
		}
		fmt.Println("GoSVN right-click integration removed.")
		return
	case "commit", "status", "st", "update", "log", "diff", "settings", "checkout", "create-repo", "remote":
		// valid GUI command, fall through
	default:
		// Unknown command: treat it as a path so `gosvn /some/path` opens
		// the main window directly.
		path = cmd
		hasPath = true
		cmd = ""
	}

	client := svn.DefaultClient()
	// Apply persisted preferences (SVN binary path and timeout).
	if s := gui.GetSettings(); s.SVNBin != "" {
		client.Bin = s.SVNBin
	}
	if s := gui.GetSettings(); s.Timeout > 0 {
		client.Timeout = time.Duration(s.Timeout) * time.Second
	}
	mygo.App.WhenReady(func() {
		switch cmd {
		case "commit", "status", "st":
			gui.OpenCommitWindow(client, path)
		case "update":
			gui.OpenUpdateWindow(client, path)
		case "log":
			gui.OpenLogWindow(client, path)
		case "diff":
			gui.OpenDiffWindow(client, path, "")
		case "settings":
			gui.OpenSettingsWindow()
		case "checkout":
			gui.OpenCheckoutWindow(client, func(p string) {
				gui.OpenMainWindow(client, p)
			})
		case "create-repo":
			gui.OpenCreateRepoWindow(client)
		case "remote":
			gui.OpenRemoteWindow(client)
		default:
			if hasPath {
				gui.OpenMainWindow(client, path)
			} else {
				gui.OpenWelcomeWindow(client, func(p string) {
					gui.OpenMainWindow(client, p)
				})
			}
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
