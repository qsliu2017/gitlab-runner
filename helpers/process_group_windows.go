package helpers

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

var (
	modkernel32 = syscall.MustLoadDLL("kernel32.dll")

	procGenerateConsoleCtrlEvent = modkernel32.MustFindProc("GenerateConsoleCtrlEvent")
)

const ErrInvalidParameter syscall.Errno = 87

// ProcessGroupKiller configures exec.Cmd and returns a function for killing
// the process.
func ProcessGroupKiller(cmd *exec.Cmd) func() {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	return func() {
		if cmd.Process == nil {
			return
		}

		_, _, err := procGenerateConsoleCtrlEvent.Call(syscall.CTRL_BREAK_EVENT, uintptr(cmd.Process.Pid))
		// GenerateConsoleCtrlEvent returns an ErrInvalidParameter if the
		// process is invalid (process already killed).
		if errors.Is(err, ErrInvalidParameter) {
			return
		}

		// Hopefully the process can no longer be found.
		_, err = os.FindProcess(cmd.Process.Pid)
		if err != nil {
			return
		}

		// Taskkill process as a last resort.
		exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	}
}
