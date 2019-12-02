// +build darwin dragonfly freebsd linux netbsd openbsd

package commands

import (
	"os/signal"
)

func (mr *RunCommand) watchGracefulShutdownSignal() {
	signal.Notify(mr.stopSignals, gracefulShutdownSignal)
}
