//go:build !windows

// Package proc holds OS-specific tweaks for child processes.
package proc

import "os/exec"

// Hide is a no-op on non-Windows platforms (no stray console windows there).
func Hide(cmd *exec.Cmd) {}
