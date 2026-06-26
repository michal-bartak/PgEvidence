//go:build !linux

package main

// IsSystemDark is only needed on Linux; macOS/Windows webviews honour
// prefers-color-scheme, so the frontend uses matchMedia there.
func (a *App) IsSystemDark() bool {
	return false
}
