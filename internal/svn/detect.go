package svn

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// commonSVNPaths returns well-known installation directories for svn.exe on
// Windows, plus PATH lookup on all platforms.
func commonSVNPaths() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	var paths []string
	// TortoiseSVN (most common)
	for _, base := range []string{
		`C:\Program Files\TortoiseSVN\bin`,
		`C:\Program Files (x86)\TortoiseSVN\bin`,
		filepath.Join(os.Getenv("LOCALAPPDATA"), `Programs\TortoiseSVN\bin`),
	} {
		paths = append(paths, filepath.Join(base, "svn.exe"))
	}
	// SlikSVN
	for _, base := range []string{
		`C:\Program Files\SlikSvn\bin`,
		`C:\Program Files (x86)\SlikSvn\bin`,
	} {
		paths = append(paths, filepath.Join(base, "svn.exe"))
	}
	// VisualSVN
	for _, base := range []string{
		`C:\Program Files\VisualSVN Server\bin`,
		`C:\Program Files (x86)\VisualSVN Server\bin`,
	} {
		paths = append(paths, filepath.Join(base, "svn.exe"))
	}
	// Generic Subversion / Apache Subversion
	for _, base := range []string{
		`C:\Program Files\Subversion\bin`,
		`C:\Program Files (x86)\Subversion\bin`,
		`D:\APP\ApacheSubversion\bin`,
		`C:\ApacheSubversion\bin`,
		`D:\ApacheSubversion\bin`,
	} {
		paths = append(paths, filepath.Join(base, "svn.exe"))
	}
	// Custom user path
	if p := os.Getenv("GOSVN_SVN_BIN"); p != "" {
		paths = append(paths, p)
	}
	return paths
}

// Detect returns the path to an svn executable, or "" if none is found.
// It checks the GOSVN_SVN_BIN env var, then common install locations, then PATH.
func Detect() string {
	// Explicit env var first.
	if p := os.Getenv("GOSVN_SVN_BIN"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Common install locations (Windows).
	for _, p := range commonSVNPaths() {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// PATH lookup.
	if p, err := exec.LookPath("svn"); err == nil {
		return p
	}
	return ""
}

// IsNotFound reports whether err indicates the svn binary was not found.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if ce, ok := err.(*CmdError); ok {
		return ce.ExitCode == -1 && ce.Err != nil
	}
	return false
}
