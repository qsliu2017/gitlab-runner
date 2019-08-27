// +build darwin dragonfly freebsd linux netbsd openbsd

package process

import (
	"syscall"
)

type unixKiller struct {
	logger Logger
	cmd    Commander
}

func newKiller(logger Logger, cmd Commander) killer {
	return &unixKiller{
		logger: logger,
		cmd:    cmd,
	}
}

func (pk *unixKiller) Terminate() {
	pid := pk.cmd.Process().Pid
	if pk.isProcessGroupCommand() {
		pid *= -1
	}

	err := syscall.Kill(pid, syscall.SIGTERM)
	if err != nil {
		pk.logger.Errorln("Failed to terminate process:", err)

		// try to kill right-after
		pk.ForceKill()
	}
}

func (pk *unixKiller) isProcessGroupCommand() bool {
	attr := pk.cmd.SysProcAttr()

	return attr != nil && attr.Setpgid
}

func (pk *unixKiller) ForceKill() {
	err := pk.cmd.Process().Kill()
	if err != nil {
		pk.logger.Errorln("Failed to force-kill:", err)
	}
}
