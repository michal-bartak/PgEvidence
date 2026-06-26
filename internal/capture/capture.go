// Package capture produces audit evidence of the screen: full-display PNG
// screenshots (the primary mechanism) and, optionally, a screen recording via
// ffmpeg when it is available on the system.
package capture

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/kbinani/screenshot"
)

// NumDisplays reports how many active displays the OS exposes.
func NumDisplays() int {
	return screenshot.NumActiveDisplays()
}

// Screenshot captures the full screen of the given display index and writes it
// as a PNG to outPath. Capturing the whole display (rather than just the app
// window) ensures the OS clock and status bar are part of the evidence.
//
// On macOS we shell out to the system `screencapture` tool: the cross-platform
// CoreGraphics path (kbinani) uses the deprecated CGDisplayCreateImage API,
// which on recent macOS omits the menu-bar "extras" rendered by ControlCenter
// (the clock and status icons). `screencapture` uses the modern capture path
// and includes the full menu bar.
func Screenshot(displayIndex int, outPath string) error {
	if displayIndex < 0 {
		displayIndex = 0
	}
	if runtime.GOOS == "darwin" {
		return screenshotMac(displayIndex, outPath)
	}
	return screenshotCG(displayIndex, outPath)
}

const permHint = "Screen Recording permission is required: enable PgEvidence under " +
	"System Settings > Privacy & Security > Screen Recording, then quit and reopen the app."

// screenshotMac captures via /usr/sbin/screencapture. Display numbers are
// 1-based for that tool, so a 0-based index maps to index+1.
//
// We do NOT pre-gate on HasScreenAccess (CGPreflightScreenCaptureAccess can
// report a false negative, e.g. just after a grant); we attempt the capture and
// only interpret an actual failure as a permission problem.
func screenshotMac(displayIndex int, outPath string) error {
	args := []string{"-x", "-t", "png", "-D", strconv.Itoa(displayIndex + 1), outPath}
	cmd := exec.Command("/usr/sbin/screencapture", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "could not create image") || msg == "" {
			// macOS reports this when capture is denied by TCC.
			RequestScreenAccess() // surface the prompt / register the app
			return fmt.Errorf("%s (%s)", permHint, msg)
		}
		return fmt.Errorf("screencapture failed: %v %s", err, msg)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		return fmt.Errorf("screenshot not written: %w", err)
	}
	if info.Size() < 1024 {
		return fmt.Errorf("screenshot looks empty — on macOS grant Screen Recording permission in System Settings > Privacy & Security")
	}
	return nil
}

// registerViaScreencapture runs a throwaway, silent `screencapture` so macOS
// adds the app to the Screen Recording list. A denied attempt is exactly what
// triggers registration (via the responsible-process chain), so the error and
// the output file are intentionally discarded. No-op off macOS.
func registerViaScreencapture() {
	if runtime.GOOS != "darwin" {
		return
	}
	tmp, err := os.CreateTemp("", "pgevidence-perm-*.png")
	if err != nil {
		return
	}
	name := tmp.Name()
	tmp.Close()
	cmd := exec.Command("/usr/sbin/screencapture", "-x", "-t", "png", "-D", "1", name)
	_ = cmd.Run()
	_ = os.Remove(name)
}

// screenshotCG captures via the cross-platform CoreGraphics/GDI/X11 path.
func screenshotCG(displayIndex int, outPath string) error {
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return fmt.Errorf("no active displays detected")
	}
	if displayIndex >= n {
		displayIndex = 0
	}
	img, err := screenshot.CaptureDisplay(displayIndex)
	if err != nil {
		return fmt.Errorf("capture display %d: %w", displayIndex, err)
	}
	if isBlank(img) {
		return fmt.Errorf("captured a blank image")
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// isBlank heuristically detects an all-black capture.
func isBlank(img *image.RGBA) bool {
	if img == nil {
		return true
	}
	b := img.Bounds()
	stepX := b.Dx()/8 + 1
	stepY := b.Dy()/8 + 1
	for y := b.Min.Y; y < b.Max.Y; y += stepY {
		for x := b.Min.X; x < b.Max.X; x += stepX {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r > 0 || g > 0 || bl > 0 {
				return false
			}
		}
	}
	return true
}

// configuredFFmpeg is an optional user-set ffmpeg path (from config); empty = auto.
var configuredFFmpeg string

// SetFFmpegPath sets a user override for the ffmpeg binary. Empty restores auto.
func SetFFmpegPath(p string) { configuredFFmpeg = p }

// FFmpegPath returns the ffmpeg binary path and whether it was found. It checks
// a user override first, then PATH, then common install dirs (macOS GUI apps get
// only a minimal PATH).
func FFmpegPath() (string, bool) {
	if configuredFFmpeg != "" {
		if info, err := os.Stat(configuredFFmpeg); err == nil && !info.IsDir() {
			return configuredFFmpeg, true
		}
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, true
	}
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name = "ffmpeg.exe"
	}
	for _, c := range []string{"/opt/homebrew/bin/" + name, "/usr/local/bin/" + name, "/opt/local/bin/" + name} {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, true
		}
	}
	return "", false
}

// FFmpegAvailable reports whether ffmpeg is installed.
func FFmpegAvailable() bool {
	_, ok := FFmpegPath()
	return ok
}
