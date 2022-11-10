package helpers

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"io"
	"os"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const (
	exitCodeNoPath = 100
)

type ReadGitlabEnvCommand struct {
	Path           string `long:"path"`
	LimitSizeBytes int64  `long:"limit-size-bytes"`
}

func newReadGitLabEnvCommand() *ReadGitlabEnvCommand {
	return &ReadGitlabEnvCommand{
		LimitSizeBytes: common.DefaultEnvFileSizeLimit,
	}
}

func (c *ReadGitlabEnvCommand) Execute(*cli.Context) {
	if c.Path == "" {
		log.Errorln("No path specified")
		os.Exit(exitCodeNoPath)
	}

	if err := c.execute(); err != nil {
		log.Errorln("Executing command error: %w", err)
		os.Exit(1)
	}
}

func (c *ReadGitlabEnvCommand) execute() error {
	f, err := os.Open(c.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	r := io.LimitReader(f, c.LimitSizeBytes)
	if _, err := io.Copy(os.Stdout, r); err != nil {
		return fmt.Errorf("piping file to stdout: %w", err)
	}

	return nil
}

func init() {
	common.RegisterCommand2(
		"read-gitlab-env",
		"reads the contents of $GITLAB_ENV file (internal)",
		newReadGitLabEnvCommand(),
	)
}
