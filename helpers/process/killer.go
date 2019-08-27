package process

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type killer interface {
	Terminate()
	ForceKill()
}

var newKillerFactory = newKiller

type KillWaiter interface {
	KillAndWait(process *os.Process, waitCh chan error) error
}

type DefaultKillWaiter struct {
	logger common.BuildLogger

	gracefulKillTimeout time.Duration
	forceKillTimeout    time.Duration
}

func NewKillWaiter(logger common.BuildLogger, gracefulKillTimeout time.Duration, forceKillTimeout time.Duration) KillWaiter {
	return &DefaultKillWaiter{
		logger:              logger,
		gracefulKillTimeout: gracefulKillTimeout,
		forceKillTimeout:    forceKillTimeout,
	}
}

func (kw *DefaultKillWaiter) KillAndWait(process *os.Process, waitCh chan error) error {
	if process == nil {
		return errors.New("process not started yet")
	}

	log := kw.logger.WithFields(logrus.Fields{
		"PID": process.Pid,
	})

	processKiller := newKillerFactory(log, process)
	processKiller.Terminate()

	select {
	case err := <-waitCh:
		return err

	case <-time.After(kw.gracefulKillTimeout):
		processKiller.ForceKill()

		select {
		case err := <-waitCh:
			return err

		case <-time.After(kw.forceKillTimeout):
			return dormantProcessError(process)
		}
	}
}

func dormantProcessError(process *os.Process) error {
	return fmt.Errorf("failed to kill process PID=%d, likely process is dormant", process.Pid)
}
