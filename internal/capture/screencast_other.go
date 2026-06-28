//go:build !linux

package capture

import "fmt"

// startRecordingPortal is Linux-only (xdg-desktop-portal ScreenCast); stub elsewhere.
func startRecordingPortal(outPath string) (*Recorder, error) {
	return nil, fmt.Errorf("portal screencast recording is only available on Linux")
}
