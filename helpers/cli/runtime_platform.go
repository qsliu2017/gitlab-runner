package cli_helpers

import (
	"runtime"

	"gitlab.com/gitlab-org/gitlab-runner/core"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func LogRuntimePlatform(app *cli.App) {
	appBefore := app.Before
	app.Before = func(c *cli.Context) error {
		logrus.WithFields(logrus.Fields{
			"os":       runtime.GOOS,
			"arch":     runtime.GOARCH,
			"version":  core.VERSION,
			"revision": core.REVISION,
		}).Debugln("Runtime platform")

		if appBefore != nil {
			return appBefore(c)
		}
		return nil
	}

}
