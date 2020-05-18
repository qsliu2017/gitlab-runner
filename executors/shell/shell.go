package shell

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/kardianos/osext"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/jobcontrol"
)

type executor struct {
	executors.AbstractExecutor
}

func (s *executor) Prepare(options common.ExecutorPrepareOptions) error {
	if options.User != "" {
		s.Shell().User = options.User
	}

	// expand environment variables to have current directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	mapping := func(key string) string {
		switch key {
		case "PWD":
			return wd
		default:
			return ""
		}
	}

	s.DefaultBuildsDir = os.Expand(s.DefaultBuildsDir, mapping)
	s.DefaultCacheDir = os.Expand(s.DefaultCacheDir, mapping)

	// Pass control to executor
	err = s.AbstractExecutor.Prepare(options)
	if err != nil {
		return err
	}

	s.Println("Using Shell executor...")
	return nil
}

func (s *executor) Run(cmd common.ExecutorCommand) error {
	c := jobcontrol.Command(cmd.Context, s.BuildShell.Command, s.BuildShell.Arguments...)
	c.Env = append(os.Environ(), s.BuildShell.Environment...)
	c.Stdout = s.Trace
	c.Stderr = s.Trace

	if s.BuildShell.PassFile {
		scriptDir, err := ioutil.TempDir("", "build_script")
		if err != nil {
			return err
		}
		defer os.RemoveAll(scriptDir)

		scriptFile := filepath.Join(scriptDir, "script."+s.BuildShell.Extension)
		err = ioutil.WriteFile(scriptFile, []byte(cmd.Script), 0700)
		if err != nil {
			return err
		}

		c.Args = append(c.Args, scriptFile)
	} else {
		c.Stdin = bytes.NewBufferString(cmd.Script)
	}

	// Start a process
	err := c.Start()
	if err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Wait for process to finish
	err = c.Wait()
	if _, ok := err.(*exec.ExitError); ok {
		err = &common.BuildError{Inner: err}
	}
	return err
}

func init() {
	// Look for self
	runnerCommand, err := osext.Executable()
	if err != nil {
		logrus.Warningln(err)
	}

	options := executors.ExecutorOptions{
		DefaultCustomBuildsDirEnabled: false,
		DefaultBuildsDir:              "$PWD/builds",
		DefaultCacheDir:               "$PWD/cache",
		SharedBuildsDir:               true,
		Shell: common.ShellScriptInfo{
			Shell:         common.GetDefaultShell(),
			Type:          common.LoginShell,
			RunnerCommand: runnerCommand,
		},
		ShowHostname: false,
	}

	creator := func() common.Executor {
		return &executor{
			AbstractExecutor: executors.AbstractExecutor{
				ExecutorOptions: options,
			},
		}
	}

	featuresUpdater := func(features *common.FeaturesInfo) {
		features.Variables = true
		features.Shared = true

		if runtime.GOOS != "windows" {
			features.Session = true
			features.Terminal = true
		}
	}

	common.RegisterExecutorProvider("shell", executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		DefaultShellName: options.Shell.Shell,
	})
}
