package capture

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"time"

	"github.com/kbinani/screenshot"

	"pgevidence/internal/proc"
)

// Recorder wraps a running screen-capture process. Recording is best-effort and
// experimental: the backend differs per OS — ffmpeg (x11grab/gdigrab/avfoundation)
// on X11/Windows/macOS, and a GStreamer + xdg-desktop-portal pipeline on Wayland.
type Recorder struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser // ffmpeg: send "q" to finalise; nil for the GStreamer path
	cleanup func()         // Wayland: close the PipeWire fd + portal session
}

// StartRecording begins capturing the screen to outPath (an .mp4). It returns an
// error if no capture backend is available or the capture cannot start.
func StartRecording(outPath string, displayIndex int) (*Recorder, error) {
	// Wayland: ffmpeg/x11grab only sees a black XWayland framebuffer, so capture
	// via the ScreenCast portal + PipeWire, encoded by GStreamer.
	if runtime.GOOS == "linux" && onWayland() {
		return startRecordingPortal(outPath)
	}

	bin, ok := FFmpegPath()
	if !ok {
		return nil, fmt.Errorf("ffmpeg not found on PATH")
	}
	args, err := ffmpegArgs(bin, outPath, displayIndex)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, args...)
	proc.Hide(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &Recorder{cmd: cmd, stdin: stdin}, nil
}

// Stop finalises the recording and waits, killing the process if it doesn't exit
// cleanly. ffmpeg is asked to quit via "q" on stdin; the GStreamer pipeline (run
// with -e) is interrupted with SIGINT so it flushes EOS and writes a valid MP4.
func (r *Recorder) Stop() error {
	if r == nil || r.cmd == nil {
		return nil
	}
	if r.stdin != nil {
		_, _ = io.WriteString(r.stdin, "q\n")
		_ = r.stdin.Close()
	} else if r.cmd.Process != nil {
		_ = r.cmd.Process.Signal(os.Interrupt)
	}

	done := make(chan error, 1)
	go func() { done <- r.cmd.Wait() }()
	var werr error
	select {
	case werr = <-done:
	case <-time.After(10 * time.Second):
		_ = r.cmd.Process.Kill()
		werr = <-done
	}
	if r.cleanup != nil {
		r.cleanup()
	}
	return werr
}

func ffmpegArgs(bin, outPath string, displayIndex int) ([]string, error) {
	common := []string{"-y", "-framerate", "25"}
	tail := []string{"-pix_fmt", "yuv420p", outPath}
	switch runtime.GOOS {
	case "darwin":
		idx := macScreenDeviceIndex(bin, displayIndex)
		in := []string{"-f", "avfoundation", "-i", fmt.Sprintf("%d:none", idx)}
		return append(append(common, in...), tail...), nil
	case "windows":
		// gdigrab captures the whole virtual desktop by default; restrict it to the
		// selected monitor's rectangle via -offset_x/-offset_y/-video_size (these
		// must precede -i). Bounds come from the same source the screenshot path
		// trusts (kbinani GetDisplayBounds), so video matches the PNG.
		in := []string{"-f", "gdigrab"}
		if n := screenshot.NumActiveDisplays(); displayIndex >= 0 && displayIndex < n {
			b := screenshot.GetDisplayBounds(displayIndex)
			w, h := b.Dx()&^1, b.Dy()&^1 // round down to even; yuv420p requires it
			if w > 0 && h > 0 {
				in = append(in,
					"-offset_x", strconv.Itoa(b.Min.X),
					"-offset_y", strconv.Itoa(b.Min.Y),
					"-video_size", fmt.Sprintf("%dx%d", w, h))
			}
		}
		in = append(in, "-i", "desktop")
		return append(append(common, in...), tail...), nil
	case "linux":
		in := []string{"-f", "x11grab", "-i", ":0.0"}
		return append(append(common, in...), tail...), nil
	default:
		return nil, fmt.Errorf("screen recording not supported on %s", runtime.GOOS)
	}
}

var macScreenRe = regexp.MustCompile(`\[(\d+)\]\s+Capture screen (\d+)`)

// macScreenDeviceIndex asks avfoundation to list devices and returns the device
// index for the requested screen, falling back to a heuristic guess.
func macScreenDeviceIndex(bin string, screen int) int {
	cmd := exec.Command(bin, "-f", "avfoundation", "-list_devices", "true", "-i", "")
	var buf bytes.Buffer
	cmd.Stderr = &buf
	_ = cmd.Run() // always exits non-zero; we only want the listing
	for _, m := range macScreenRe.FindAllStringSubmatch(buf.String(), -1) {
		dev, _ := strconv.Atoi(m[1])
		scr, _ := strconv.Atoi(m[2])
		if scr == screen {
			return dev
		}
	}
	return screen
}
