package process

import (
	"os"
)

type windowsKiller struct {
	logger  Logger
	process *os.Process
}

func newKiller(logger Logger, cmd Commander) killer {
	return &windowsKiller{
		logger:  logger,
		process: cmd.Process(),
	}
}

func (pk *windowsKiller) Terminate() {
	err := pk.process.Kill()
	if err != nil {
		pk.logger.Errorln("Failed to terminate process:", err)

		// try to kill right-after
		pk.ForceKill()
	}
}

func (pk *windowsKiller) ForceKill() {
	err := pk.process.Kill()
	if err != nil {
		pk.logger.Errorln("Failed to force-kill:", err)
	}
}
