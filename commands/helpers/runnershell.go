package helpers

import (
	"encoding/json"
	"io"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/shells/runnershell"
)

type RunnerShellCommand struct {
}

func (c *RunnerShellCommand) Execute(cliContext *cli.Context) {
	var script runnershell.Script

	var in io.Reader

	filepath := cliContext.Args().First()
	if filepath == "" {
		in = os.Stdin
	} else {
		f, err := os.Open(filepath)
		if err != nil {
			logrus.Fatalf("Error opening file: %v", err)
		}
		defer f.Close()
		in = f
	}

	if err := json.NewDecoder(in).Decode(&script); err != nil {
		logrus.Fatalf("Error decoding actions: %v", err)
	}

	err := script.Execute(runnershell.Options{
		Env:          os.Environ(),
		ShellCommand: script.ShellCommand,
		Trace:        script.Trace,
	})
	if err != nil {
		logrus.Fatalln(err)
	}
}

func init() {
	common.RegisterCommand2(
		"runnershell",
		"execute internal commands (internal)",
		&RunnerShellCommand{},
	)
}
