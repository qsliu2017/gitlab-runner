package helpers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/urfave/cli"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type EntryCommand struct {
	Idle                  bool   `long:"idle"`
	DetectShellScriptPath string `long:"detect-shell-script-path"`
	ProjectDir            string `long:"project-dir"`
	IdleTime              int    `long:"idle-time"`

	shell *exec.Cmd
}

func newEntryCommand() *EntryCommand {
	return &EntryCommand{}
}

func (c *EntryCommand) Execute(*cli.Context) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell := exec.Command("sh", c.DetectShellScriptPath)
	shell.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	shell.Stdout = os.Stdout
	shell.Stdin = os.Stdin
	shell.Stderr = os.Stderr

	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for range signals {
			//if sig == syscall.SIGINT {
			//	// SIGINT doesn't terminate the shell and does nothing for us in this case
			//	sig = syscall.SIGTERM
			//}
			//fmt.Println("signal", sig)
			//
			//s, ok := sig.(syscall.Signal)
			//if !ok {
			//	panic("os: unsupported signal type")
			//}

			if err := syscall.Kill(-shell.Process.Pid, syscall.SIGKILL); err != nil {
				fmt.Println("syscall.Kill", err)
			}
		}
	}()

	go func() {
		ctx, timeoutCancel := context.WithTimeout(ctx, time.Duration(c.IdleTime)*time.Second)
		defer timeoutCancel()

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		hasRunningBuild := func() bool {
			stat, err := os.Stat(c.ProjectDir)
			return stat != nil && err == nil
		}

		for {
			select {
			case <-ctx.Done():
				hasBuild := hasRunningBuild()
				fmt.Println("Timeout....checking for build: ", hasBuild)
				if !hasBuild {
					fmt.Println("Exiting container")
					os.Exit(1)
				}

				return
			case <-ticker.C:
				hasBuild := hasRunningBuild()
				fmt.Println("Checking for running build: ", hasBuild)
				if hasBuild {
					return
				}
			}
		}
	}()

	if err := shell.Run(); err != nil {
		fmt.Println(err)
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			os.Exit(exitError.ExitCode())
		}

		os.Exit(1)
	}

	os.Exit(0)
}

func init() {
	common.RegisterCommand2(
		"entry",
		"entry (internal)",
		newEntryCommand(),
	)
}
