package helpers

import (
	"io"
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var stdin io.Reader = os.Stdin

type WriteFileCommand struct {
	Mode string `long:"mode"`
}

func newWriteFileCommand() *WriteFileCommand {
	return &WriteFileCommand{}
}

func (c *WriteFileCommand) Execute(cliContext *cli.Context) {
	pathname := cliContext.Args().Get(0)
	if pathname == "" {
		logrus.Fatalln("No write file path provided")
	}

	f, err := os.Create(pathname)
	if err != nil {
		logrus.Fatalln("Creating file failed:", err)
	}
	defer func() {
		_ = f.Close()
	}()

	if _, err := io.Copy(f, io.TeeReader(stdin, os.Stdout)); err != nil {
		logrus.Fatalln("Writing file failed:", err)
	}

	if err := f.Close(); err != nil {
		logrus.Fatalln("Flushing file failed:", err)
	}

	if mode := c.Mode; mode != "" {
		m, err := strconv.ParseInt(mode, 8, 32)
		if err != nil {
			logrus.Fatalln("Invalid mode")
		}

		if err := os.Chmod(pathname, os.FileMode(m)); err != nil {
			logrus.Fatalln("Chmod:", err)
		}
	}
}

func init() {
	common.RegisterCommand2(
		"write-file",
		"write-file reads contents from stdin and writes it to the path provided as an argument (internal)",
		newWriteFileCommand(),
	)
}
