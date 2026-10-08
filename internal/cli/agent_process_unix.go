//go:build unix

package cli

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func configurePlannerCancellation(cmd *exec.Cmd) {
	// The shell and its descendants own a separate group, so cancellation cannot
	// leave planner children running or terminate the invoking CLI's group.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
