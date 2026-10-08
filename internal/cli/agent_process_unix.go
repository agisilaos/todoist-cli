//go:build unix

package cli

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func configurePlannerCancellation(cmd *exec.Cmd) {
	// Give the planner a dedicated process group so cancellation kills its group
	// members without killing the invoking CLI's group. Detached groups are outside it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
