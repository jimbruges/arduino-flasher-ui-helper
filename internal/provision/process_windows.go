package provision

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// taskkill /T terminates CLI children as well as their parent on cancellation.
func configureCommand(cmd *exec.Cmd) func() {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		taskkill := exec.Command(filepath.Join(os.Getenv("SystemRoot"), "System32", "taskkill.exe"), "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
		taskkill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := taskkill.Run(); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
	return func() {}
}
