// +build darwin dragonfly freebsd linux netbsd openbsd

package process

import (
	"os"
	"syscall"
)

type unixKiller struct {
	logger  Logger
	process *os.Process
}

func newKiller(logger Logger, process *os.Process) killer {
	return &unixKiller{
		logger:  logger,
		process: process,
	}
}

func (pk *unixKiller) Terminate() {
	err := pk.process.Signal(syscall.SIGTERM)
	if err != nil {
		pk.logger.Errorln("Failed to send SIGTERM signal:", err)

		// try to kill right-after
		pk.ForceKill()
	}
}

func (pk *unixKiller) ForceKill() {
	err := pk.process.Kill()
	if err != nil {
		pk.logger.Errorln("Failed to force-kill:", err)
	}
}
