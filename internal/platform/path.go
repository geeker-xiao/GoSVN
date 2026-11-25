// Package platform isolates platform-specific code (Windows registry, Linux
// Nautilus scripts, path/encoding helpers) behind build tags so the rest of
// the application stays cross-platform.
package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

// Executable returns the absolute path of the running binary, or an empty
// string if it cannot be determined.
func Executable() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}

// Abs returns an absolute, cleaned path. On failure it returns the input.
func Abs(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// IsWindows reports the build target.
func IsWindows() bool { return runtime.GOOS == "windows" }

// Quote wraps s in double quotes if it contains spaces or quotes, escaping
// embedded quotes. Used when building command lines stored in the registry
// or shell scripts.
func Quote(s string) string {
	if s == "" {
		return `""`
	}
	needQuote := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '"' || r == '%' {
			needQuote = true
			break
		}
	}
	if !needQuote {
		return s
	}
	out := make([]byte, 0, len(s)+2)
	out = append(out, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' || c == '\\' {
			out = append(out, '\\')
		}
		out = append(out, c)
	}
	out = append(out, '"')
	return string(out)
}
