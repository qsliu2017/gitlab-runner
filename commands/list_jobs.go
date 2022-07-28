package commands

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

//nolint:lll
type ListJobsCommand struct {
	configOptions
	common.RunnerCredentials
	network common.Network
}

func (c *ListJobsCommand) Execute(context *cli.Context) {
	// 	read the configuration file
	err := c.loadConfig()
	if err != nil {
		logrus.Warningln(err)
		return
	}

	// check if metrics are enabled
	if c.config.ListenAddress == "" {
		logrus.Errorln("listen_address not defined, listing of jobs is unavailable")
		return
	}

	// if yes - read data from the debug/jobs/list endpoint and print it to STDOUT
	for _, runner := range c.config.Runners {
		state := c.network.ListJobs(runner.RunnerCredentials)
		fmt.Printf(
			"runner-name=%s state=%s\n",
			runner.Name,
			state,
		)
	}
}

func init() {
	common.RegisterCommand2("list-jobs", "List all currently running jobs on configured runners", &ListJobsCommand{
		network: network.NewGitLabClient(),
	})
}
