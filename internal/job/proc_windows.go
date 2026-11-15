//go:build windows

package job

import (
	"os/exec"
	"strconv"
	"syscall"
)

// prepare makes the child start without a console window
// (CreationFlags 0x08000000 is CREATE_NO_WINDOW).
func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

// kill stops the process and its descendants. It falls back to killing only
// the process when taskkill fails.
func kill(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	tk := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	prepare(tk)
	if err := tk.Run(); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
