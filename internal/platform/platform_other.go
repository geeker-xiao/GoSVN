//go:build !windows && !linux

package platform

import "errors"

// InstallIntegration is a no-op on platforms without a supported desktop
// integration (only Windows Explorer and Linux Nautilus are supported).
func InstallIntegration() error {
	return errors.New("desktop integration is not supported on this platform")
}

// UninstallIntegration is a no-op on unsupported platforms.
func UninstallIntegration() error {
	return errors.New("desktop integration is not supported on this platform")
}
