//go:build !linux

package capture

import (
	"context"
	"fmt"
)

// screenshotPortal is Linux-only (xdg-desktop-portal); stub elsewhere.
func screenshotPortal(ctx context.Context, outPath string) error {
	return fmt.Errorf("desktop portal screenshot is only available on Linux")
}

// screenshotX11Root is Linux-only (X11/XWayland root capture); stub elsewhere.
func screenshotX11Root(outPath string) error {
	return fmt.Errorf("x11 root screenshot is only available on Linux")
}
