package app

import (
	"os"
	"runtime"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func LogRuntimePlatform(cliCtx *cli.Context) error {
	fields := logrus.Fields{
		"os":       runtime.GOOS,
		"arch":     runtime.GOARCH,
		"version":  common.VERSION,
		"revision": common.REVISION,
		"pid":      os.Getpid(),
	}

	logrus.WithFields(fields).Info("Runtime platform")

	return nil
}
