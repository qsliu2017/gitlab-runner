package main

import (
	"os"
	"path"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/core"
	"gitlab.com/gitlab-org/gitlab-runner/core/commands"
	"gitlab.com/gitlab-org/gitlab-runner/core/formatter"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			// log panics forces exit
			if _, ok := r.(*logrus.Entry); ok {
				os.Exit(1)
			}
			panic(r)
		}
	}()

	formatter.SetRunnerFormatter()
	formatter.AddSecretsCleanupLogHook()

	app := cli.NewApp()
	app.Name = path.Base(os.Args[0])
	app.Usage = "a GitLab Runner Helper"
	app.Version = core.AppVersion.ShortLine()
	cli.VersionPrinter = core.AppVersion.Printer
	app.Authors = []cli.Author{
		{
			Name:  "GitLab Inc.",
			Email: "support@gitlab.com",
		},
	}
	formatter.SetupLogLevelOptions(app)
	app.Commands = commands.GetCommands()
	app.CommandNotFound = func(context *cli.Context, command string) {
		logrus.Fatalln("Command", command, "not found")
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
