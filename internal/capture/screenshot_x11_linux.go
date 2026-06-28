//go:build linux

package capture

import (
	"fmt"
	"image/png"
	"os"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/kbinani/screenshot"
)

// screenshotX11Root captures the entire X11 root window at its ACTUAL size and
// writes it to outPath. This fixes the clipped capture under GNOME Wayland with
// fractional scaling: kbinani's CaptureDisplay sizes the grab from XRandR's
// physical monitor bounds (e.g. 3840×2160) while XWayland's root is the logical
// size (e.g. 2560×1440 at 150%), so it grabbed only the top-left corner. Querying
// the root geometry directly and capturing exactly that yields the full screen,
// silently, at any scaling. Works on a real X11 session too.
func screenshotX11Root(outPath string) error {
	w, h, err := x11RootSize()
	if err != nil {
		return fmt.Errorf("x11 root size: %w", err)
	}
	if w <= 0 || h <= 0 {
		return fmt.Errorf("x11 root size invalid: %dx%d", w, h)
	}
	debugf("x11 root: capturing %dx%d", w, h)
	img, err := screenshot.Capture(0, 0, w, h)
	if err != nil {
		return fmt.Errorf("capture root %dx%d: %w", w, h, err)
	}
	if isBlank(img) {
		return fmt.Errorf("captured a blank image")
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		os.Remove(outPath)
		return err
	}
	return f.Close()
}

// x11RootSize returns the default screen's root window size in pixels via a direct
// X11 connection (XWayland under Wayland, or the real X server on X11).
func x11RootSize() (int, int, error) {
	c, err := xgb.NewConn()
	if err != nil {
		return 0, 0, err
	}
	defer c.Close()
	screen := xproto.Setup(c).DefaultScreen(c)
	return int(screen.WidthInPixels), int(screen.HeightInPixels), nil
}
