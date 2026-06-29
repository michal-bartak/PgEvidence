//go:build !linux

package capture

import (
	"context"
	"fmt"
)

// screenshotPortal is Linux-only (xdg-desktop-portal); stub elsewhere.
func screenshotPortal(ctx context.Context, displayIndex int, outPath string) error {
	return fmt.Errorf("desktop portal screenshot is only available on Linux")
}
