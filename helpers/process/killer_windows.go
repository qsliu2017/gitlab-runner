package process

import (
	"os/exec"
	"strconv"

	"golang.org/x/sys/windows"
)

type windowsKiller struct {
	logger Logger
	cmd    Commander
}

func newKiller(logger Logger, cmd Commander) killer {
	return &windowsKiller{
		logger: logger,
		cmd:    cmd,
	}
}

// https://docs.microsoft.com/en-us/windows/console/generateconsolectrlevent
func (pk *windowsKiller) Terminate() {
	if pk.cmd.Process() == nil {
		return
	}

	err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(pk.cmd.Process().Pid))
	if err != nil {
		pk.logger.Warn("Failed to terminate process:", err)

		// try to kill right-after
		pk.ForceKill()
	}
}

func (pk *windowsKiller) ForceKill() {
	if pk.cmd.Process() == nil {
		return
	}

	err := taskKill(pk.cmd.Process().Pid)
	if err != nil {
		pk.logger.Warn("Failed to force-kill:", err)
	}
}

func taskKill(pid int) error {
	return exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
}
