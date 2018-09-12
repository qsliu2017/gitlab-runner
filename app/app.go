package app

import (
	"os"
	"path"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/log"
)

var (
	authors = []cli.Author{
		{
			Name:  "GitLab Inc.",
			Email: "support@gitlab.com",
		},
	}
)

type Handler func(cliCtx *cli.Context) error
type Handlers []Handler

func (a *Handlers) Handle(cliCtx *cli.Context) error {
	for _, f := range *a {
		err := f(cliCtx)
		if err != nil {
			return err
		}
	}

	return nil
}

type App struct {
	app *cli.App

	beforeFunctions Handlers
	afterFunctions  Handlers
}

func (a *App) init(usage string) {
	app := cli.NewApp()
	a.app = app

	app.Name = path.Base(os.Args[0])
	app.Usage = usage
	app.Authors = authors
	app.Version = common.AppVersion.ShortLine()
	cli.VersionPrinter = common.AppVersion.Printer

	app.Commands = common.GetCommands()
	app.CommandNotFound = func(cliCtx *cli.Context, command string) {
		logrus.Fatalf("Command %s not found", command)
	}

	a.beforeFunctions = make(Handlers, 0)
	app.Before = a.beforeFunctions.Handle

	a.afterFunctions = make(Handlers, 0)
	app.After = a.afterFunctions.Handle
}

func (a *App) Run() {
	if err := a.app.Run(os.Args); err != nil {
		logrus.WithError(err).Fatal("Application execution failed")
	}
}

func (a *App) Extend(extension func(*cli.App)) {
	extension(a.app)
}

func (a *App) AppendBeforeFunc(f Handler) {
	a.beforeFunctions = append(a.beforeFunctions, f)
}

func (a *App) AppendAfterFunc(f Handler) {
	a.afterFunctions = append(a.afterFunctions, f)
}

func New(usage string) *App {
	app := new(App)
	app.init(usage)
	app.Extend(log.AddFlags)
	app.AppendBeforeFunc(log.ConfigureLogging)

	return app
}

func Recover() {
	r := recover()
	if r != nil {
		// log panics forces exit
		if _, ok := r.(*logrus.Entry); ok {
			os.Exit(1)
		}
		panic(r)
	}
}
