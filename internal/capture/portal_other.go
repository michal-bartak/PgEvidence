//go:build !linux

package capture

import "fmt"

// screenshotPortal is Linux-only (xdg-desktop-portal); stub elsewhere.
func screenshotPortal(outPath string) error {
	return fmt.Errorf("desktop portal screenshot is only available on Linux")
}
