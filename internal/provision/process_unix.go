//go:build darwin || linux

package provision

import (
	"errors"
	"os/exec"
	"syscall"
)

func configureCommand(cmd *exec.Cmd) func() {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	kill := func() error {
		if cmd.Process == nil {
			return nil
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	cmd.Cancel = kill
	return func() { _ = kill() }
}
