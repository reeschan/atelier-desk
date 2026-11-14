//go:build !windows

package job

import (
	"os/exec"
	"syscall"
)

// prepare puts the child in its own process group so kill can stop descendants.
func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// kill sends SIGKILL to the whole process group, falling back to the process.
func kill(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
