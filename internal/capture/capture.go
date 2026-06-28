// Package capture produces audit evidence of the screen: full-display PNG
// screenshots (the primary mechanism) and, optionally, a screen recording via
// ffmpeg when it is available on the system.
package capture

import (
	"bytes"
	"context"
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

// debugf writes a capture diagnostic to stderr when PGEVIDENCE_DEBUG is set.
func debugf(format string, a ...interface{}) {
	if os.Getenv("PGEVIDENCE_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[capture] "+format+"\n", a...)
	}
}

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
	return ScreenshotContext(context.Background(), displayIndex, outPath)
}

// ScreenshotContext is Screenshot with a cancellation context, so a run's Cancel
// (and the Linux portal's internal timeout) can interrupt a stuck capture.
func ScreenshotContext(ctx context.Context, displayIndex int, outPath string) error {
	if displayIndex < 0 {
		displayIndex = 0
	}
	switch runtime.GOOS {
	case "darwin":
		return screenshotMac(displayIndex, outPath)
	case "linux":
		return screenshotLinux(ctx, displayIndex, outPath)
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

// RegisterForScreenAccess forces the app into the macOS Screen Recording list by
// performing a real capture attempt. There is NO public API to query list
// membership (CGPreflight can't tell "not listed" from "listed but disabled"), so
// instead of detecting it we guarantee it: the attempt registers the app via the
// responsible-process chain. It is self-regulating — it shows the system prompt
// only when status is "not determined", and is silent when access was denied.
func RegisterForScreenAccess() { registerViaScreencapture() }

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

// onWayland reports whether this is a Wayland session.
func onWayland() bool {
	return os.Getenv("WAYLAND_DISPLAY") != "" ||
		strings.Contains(strings.ToLower(os.Getenv("XDG_SESSION_TYPE")), "wayland")
}

// screenshotLinux captures the full screen. On Wayland the kbinani/X11 path
// (screenshotCG) is unusable — it clips under fractional scaling (XWayland reports
// a scaled root geometry) and the desktop deliberately blocks silent X11 capture —
// so we use the xdg-desktop-portal Screenshot interface, which is the supported,
// scaling-correct method (GNOME may show a permission dialog; that's inherent to
// Wayland). On a genuine X11 session the kbinani path works fully and silently, so
// it's used directly. The portal also serves as a fallback if X11 capture fails.
//
// The portal captures the whole desktop, not a single monitor by index, so
// displayIndex is only honoured by the kbinani path.
// screenshotLinux captures the full screen. On Wayland the kbinani/X11 path clips
// under fractional scaling and silent capture is blocked, so we use the
// xdg-desktop-portal Screenshot interface — the supported, scaling-correct method
// that captures the real screen incl. the top-bar clock. On a genuine X11 session
// the kbinani path is full and silent, so it's preferred there. Each path falls
// back to the other. (The portal triggers GNOME's screenshot flash; that happens
// after the grab, so it is not in the captured image.)
func screenshotLinux(ctx context.Context, displayIndex int, outPath string) error {
	debugf("screenshotLinux: wayland=%v XDG_SESSION_TYPE=%q WAYLAND_DISPLAY=%q",
		onWayland(), os.Getenv("XDG_SESSION_TYPE"), os.Getenv("WAYLAND_DISPLAY"))
	if onWayland() {
		if err := screenshotPortal(ctx, outPath); err != nil {
			debugf("portal failed: %v; trying X11 fallback", err)
			if cgErr := screenshotCG(displayIndex, outPath); cgErr != nil {
				return fmt.Errorf("portal screenshot failed (%v); X11 fallback also failed: %w", err, cgErr)
			}
		}
		return nil
	}
	// X11 session: the kbinani path captures the full screen silently.
	if err := screenshotCG(displayIndex, outPath); err != nil {
		if pErr := screenshotPortal(ctx, outPath); pErr != nil {
			return fmt.Errorf("X11 screenshot failed (%v); portal fallback also failed: %w", err, pErr)
		}
	}
	return nil
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

// RecordingAvailable reports whether a screen-recording backend is present for
// this session: gst-launch-1.0 on Wayland (ffmpeg can't capture there), otherwise
// ffmpeg.
func RecordingAvailable() bool {
	if runtime.GOOS == "linux" && onWayland() {
		_, err := exec.LookPath("gst-launch-1.0")
		return err == nil
	}
	return FFmpegAvailable()
}
