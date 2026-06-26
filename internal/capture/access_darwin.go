//go:build darwin

package capture

/*
#cgo LDFLAGS: -framework CoreGraphics
#include <CoreGraphics/CoreGraphics.h>
*/
import "C"

// HasScreenAccess reports whether the app currently holds macOS Screen Recording
// permission (TCC). Capturing the full screen — including the menu-bar clock —
// requires it.
func HasScreenAccess() bool {
	return bool(C.CGPreflightScreenCaptureAccess())
}

// RequestScreenAccess triggers the macOS Screen Recording permission prompt and
// adds the app to System Settings > Privacy & Security > Screen Recording. It
// returns whether access is granted. Note: a running process must usually be
// restarted after the user enables it for the grant to take effect.
func RequestScreenAccess() bool {
	granted := bool(C.CGRequestScreenCaptureAccess())
	// Also make a throwaway `screencapture` attempt so a freshly installed app
	// actually appears in the Screen Recording list. CGRequest alone is unreliable
	// at registering an app that captures via the external `screencapture` tool;
	// a real (even denied) attempt registers it via the responsible-process chain,
	// matching how a normal run registers it.
	registerViaScreencapture()
	return granted
}
