//go:build linux

package main

import (
	"os/exec"
	"strings"
)

// IsSystemDark reports whether the desktop environment prefers a dark theme.
// WebKitGTK on Linux does not reliably expose this via prefers-color-scheme, so
// the frontend asks the backend, which probes GNOME/KDE settings.
func (a *App) IsSystemDark() bool {
	if dark, ok := gnomeColorScheme(); ok {
		return dark
	}
	if dark, ok := gnomeGtkTheme(); ok {
		return dark
	}
	if dark, ok := kdeColorScheme(); ok {
		return dark
	}
	return false
}

func gsettingsGet(schema, key string) (string, bool) {
	out, err := exec.Command("gsettings", "get", schema, key).Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

func gnomeColorScheme() (bool, bool) {
	val, ok := gsettingsGet("org.gnome.desktop.interface", "color-scheme")
	if !ok {
		return false, false
	}
	switch val {
	case "'prefer-dark'":
		return true, true
	case "'prefer-light'", "'default'":
		return false, true
	default:
		return false, false
	}
}

func gnomeGtkTheme() (bool, bool) {
	val, ok := gsettingsGet("org.gnome.desktop.interface", "gtk-theme")
	if !ok {
		return false, false
	}
	theme := strings.ToLower(strings.Trim(val, "'"))
	return strings.Contains(theme, "dark"), true
}

func kdeColorScheme() (bool, bool) {
	out, err := exec.Command("kreadconfig6", "--file", "kdeglobals", "--group", "General", "--key", "ColorScheme").Output()
	if err != nil {
		out, err = exec.Command("kreadconfig5", "--file", "kdeglobals", "--group", "General", "--key", "ColorScheme").Output()
		if err != nil {
			return false, false
		}
	}
	scheme := strings.ToLower(strings.TrimSpace(string(out)))
	return strings.Contains(scheme, "dark"), true
}
