package capture

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"time"
)

// Recorder wraps a running ffmpeg screen-capture process. Recording is
// best-effort and experimental: inputs differ per OS and may require extra
// permissions (e.g. Screen Recording on macOS).
type Recorder struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
}

// StartRecording begins capturing the screen to outPath (an .mp4). It returns an
// error if ffmpeg is not installed or the capture command cannot start.
func StartRecording(outPath string, displayIndex int) (*Recorder, error) {
	bin, ok := FFmpegPath()
	if !ok {
		return nil, fmt.Errorf("ffmpeg not found on PATH")
	}
	args, err := ffmpegArgs(bin, outPath, displayIndex)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &Recorder{cmd: cmd, stdin: stdin}, nil
}

// Stop asks ffmpeg to finalise the file (by sending "q" on stdin) and waits up
// to a few seconds, killing the process if it does not exit cleanly.
func (r *Recorder) Stop() error {
	if r == nil || r.cmd == nil {
		return nil
	}
	_, _ = io.WriteString(r.stdin, "q\n")
	_ = r.stdin.Close()

	done := make(chan error, 1)
	go func() { done <- r.cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		_ = r.cmd.Process.Kill()
		return <-done
	}
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
		in := []string{"-f", "gdigrab", "-i", "desktop"}
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
