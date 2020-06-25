package helpers

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type PauseCommand struct{}

func (c *PauseCommand) Execute(ctx *cli.Context) {
	sig := make(chan os.Signal)
	done := make(chan int, 1)

	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		switch <-sig {
		case syscall.SIGINT:
			done <- 1
		case syscall.SIGTERM:
			done <- 2
		}
	}()

	os.Exit(<-done)
}

func init() {
	common.RegisterCommand2("pause", "pause", &PauseCommand{})
}
