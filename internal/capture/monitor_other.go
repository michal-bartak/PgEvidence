//go:build !darwin

package capture

// DisplayContainingWindow has no native implementation off macOS; callers fall
// back to resolving the display from Wails' window position (which is in global
// coordinates on X11/Windows) via DisplayContaining.
func DisplayContainingWindow() (int, bool) { return 0, false }
