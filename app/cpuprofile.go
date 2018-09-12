package app

import (
	"os"
	"runtime/pprof"

	"github.com/urfave/cli"
)

func CPUProfileFlags(app *cli.App) {
	app.Flags = append(app.Flags, cli.StringFlag{
		Name:   "cpuprofile",
		Usage:  "write cpu profile to file",
		EnvVar: "CPU_PROFILE",
	})
}

func CPUProfileSetup(cliCtx *cli.Context) error {
	if cpuProfile := cliCtx.String("cpuprofile"); cpuProfile != "" {
		f, err := os.Create(cpuProfile)
		if err != nil {
			return err
		}
		pprof.StartCPUProfile(f)
	}

	return nil
}

func CPUProfileTeardown(cliCtx *cli.Context) error {
	pprof.StopCPUProfile()
	return nil
}
