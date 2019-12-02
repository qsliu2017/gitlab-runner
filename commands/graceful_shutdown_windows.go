package commands

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const (
	defaultCheckQuitFileInterval = 5 * time.Second
)

func (mr *RunCommand) watchGracefulShutdownSignal() {
	mr.watchGracefulShutdownSignalWithInterval(defaultCheckQuitFileInterval)
}

func (mr *RunCommand) watchGracefulShutdownSignalWithInterval(interval time.Duration) {
	quitFile := quitFileFromConfig(mr.ConfigFile)

	for mr.stopSignal == nil {
		_, err := os.Stat(quitFile)
		if err == nil {
			break
		}

		time.Sleep(interval)
	}

	defer os.Remove(quitFile)

	logrus.WithField("file", quitFile).
		Warning("quit file detected; sending SIGQUIT to process")

	mr.stopSignals <- gracefulShutdownSignal
}

func quitFileFromConfig(configFilePath string) string {
	return fmt.Sprintf("%s.quit", configFilePath)
}

func RunGracefulShutdown(c *cli.Context) {
	configFile := c.String("config")
	if configFile == "" {
		logrus.Fatal("--config option can't be empty")
	}

	now := fmt.Sprintf("%d", time.Now().Unix())

	quitFile := quitFileFromConfig(configFile)
	log := logrus.WithField("file", quitFile)

	err := ioutil.WriteFile(quitFile, []byte(now), 0600)
	if err != nil {
		log.WithError(err).
			Fatal("couldn't create quit file")
	}

	log.Info("quit file created")
}

func init() {
	common.RegisterCommand(cli.Command{
		Name: "graceful-shutdown",
		Usage: "DEPRECATED: terminates Runner gracefully (temporary solution until graceful " +
			"shutdown will become the only termination strategy)",
		Action: RunGracefulShutdown,
		Flags:  getConfigFlags(),
	})
}
