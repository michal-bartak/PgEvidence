// Package proc holds OS-specific tweaks for child processes.
package proc

import (
	"os/exec"
	"syscall"
)

// createNoWindow (CREATE_NO_WINDOW) makes a console child run without allocating
// a console, so launching psql/ffmpeg from the GUI doesn't flash a shell window.
const createNoWindow = 0x08000000

// Hide configures cmd so it spawns without a visible console window on Windows.
func Hide(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
