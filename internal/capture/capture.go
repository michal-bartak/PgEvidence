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
	switch runtime.GOOS {
	case "darwin":
		return screenshotMac(displayIndex, outPath)
	case "linux":
		return screenshotLinux(displayIndex, outPath)
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

// linuxShotTool is a desktop screenshot CLI tried on Linux; args places outPath last.
type linuxShotTool struct {
	bin  string
	args func(out string) []string
}

// linuxShotTools are tried in order; the first one found on PATH that writes a
// valid PNG wins. gnome-screenshot (GNOME), spectacle (KDE), grim (wlroots).
var linuxShotTools = []linuxShotTool{
	{"gnome-screenshot", func(out string) []string { return []string{"-f", out} }},
	{"spectacle", func(out string) []string { return []string{"-b", "-n", "-f", "-o", out} }},
	{"grim", func(out string) []string { return []string{out} }},
}

// screenshotLinux captures the full screen with a compositor-native screenshot
// tool. The kbinani/X11 path (screenshotCG) clips under GNOME Wayland + fractional
// scaling — XWayland reports a scaled root geometry, so only a sub-region is
// captured. A native tool captures the real physical screen (incl. the top-bar
// clock). If no tool is available or all fail, fall back to the X11 path, which
// still works on a genuine X11 session.
//
// These tools capture the whole desktop, not a single monitor by index, so
// displayIndex is not honoured here (multi-monitor selection on Wayland is out of
// scope); the kbinani fallback still uses it.
func screenshotLinux(displayIndex int, outPath string) error {
	var lastErr error
	for _, t := range linuxShotTools {
		bin, err := exec.LookPath(t.bin)
		if err != nil {
			continue
		}
		cmd := exec.Command(bin, t.args(outPath)...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			lastErr = fmt.Errorf("%s: %v %s", t.bin, err, strings.TrimSpace(stderr.String()))
			os.Remove(outPath)
			continue
		}
		if info, serr := os.Stat(outPath); serr != nil || info.Size() < 1024 {
			lastErr = fmt.Errorf("%s produced no usable image", t.bin)
			os.Remove(outPath)
			continue
		}
		return nil
	}
	// No native tool worked — fall back to the X11/kbinani path (fine on X11).
	if err := screenshotCG(displayIndex, outPath); err != nil {
		if lastErr != nil {
			return fmt.Errorf("no Wayland-capable screenshot tool succeeded (%v); install gnome-screenshot. X11 fallback also failed: %w", lastErr, err)
		}
		return err
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
