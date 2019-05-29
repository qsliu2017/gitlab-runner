package generic_script

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
)

type executor struct {
	executors.AbstractExecutor

	tempDir string
}

func (e *executor) createCommand(cmd string, args ...string) *exec.Cmd {
	process := exec.Command(cmd, args...)
	process.Dir = e.tempDir
	process.Stdin = nil
	process.Stdout = e.Trace
	process.Stderr = e.Trace

	process.Env = os.Environ()
	process.Env = append(process.Env, "TMPDIR="+e.tempDir)
	for _, variable := range e.Build.GetAllVariables().PublicOrInternal() {
		process.Env = append(process.Env, "GENERIC_ENV_"+variable.Key+"="+variable.Value)
	}

	return process
}

func (e *executor) killAndWait(process *exec.Cmd, waitCh chan error) error {
	if process.Process == nil {
		return errors.New("process not started yet")
	}

	started := time.Now()
	log := e.BuildLogger.WithFields(logrus.Fields{"PID": process.Process.Pid})

	for time.Since(started) < killDeadline {
		// try to interrupt first
		err := process.Process.Signal(os.Interrupt)
		if err != nil {
			log.Errorln("Failed to send signal:", err)

			// try to kill right-after
			err = process.Process.Kill()
			if err != nil {
				log.Errorln("Failed to kill:", err)
			}
		}

		select {
		case <-time.After(gracePeriodDeadline):
			err = process.Process.Kill()
			if err != nil {
				log.Errorln("Failed to kill:", err)
			}

		case err := <-waitCh:
			return err
		}
	}

	return errors.New("failed to kill process, likely process is dormant")
}

func (e *executor) runCommand(ctx context.Context, cmd string, args ...string) error {
	process := e.createCommand(cmd, args...)

	// Start a process
	err := process.Start()
	if err != nil {
		return fmt.Errorf("failed to start process: %s", err)
	}

	// Wait for process to finish
	waitCh := make(chan error)
	go func() {
		err := process.Wait()
		if _, ok := err.(*exec.ExitError); ok {
			err = &common.BuildError{Inner: err}
		}
		waitCh <- err
	}()

	// Wait for process to finish
	select {
	case err = <-waitCh:
		return err

	case <-ctx.Done():
		return e.killAndWait(process, waitCh)
	}
}

func (e *executor) Prepare(options common.ExecutorPrepareOptions) error {
	err := e.AbstractExecutor.Prepare(options)
	if err != nil {
		return err
	}

	e.Println("Using GenericScript executor...")

	if e.Config.GenericScript == nil {
		return common.MakeBuildError("GenericScript executor not configured")
	}

	if e.Config.GenericScript.RunScript == "" {
		return common.MakeBuildError("GenericScript executor is missing RunScript")
	}

	e.tempDir, err = ioutil.TempDir("", "generic-executor")
	if err != nil {
		return err
	}

	// nothing to do, as there's no cleanup_script
	if e.Config.GenericScript.PrepareScript == "" {
		return nil
	}

	ctx, cancelFunc := context.WithTimeout(e.Context, prepareScriptTimeout)
	defer cancelFunc()

	return e.runCommand(ctx, e.Config.GenericScript.PrepareScript)
}

func (e *executor) Run(cmd common.ExecutorCommand) error {
	scriptDir, err := ioutil.TempDir(e.tempDir, "script")
	if err != nil {
		return err
	}

	scriptFile := filepath.Join(scriptDir, "script."+e.BuildShell.Extension)
	err = ioutil.WriteFile(scriptFile, []byte(cmd.Script), 0700)
	if err != nil {
		return err
	}

	return e.runCommand(cmd.Context, e.Config.GenericScript.RunScript,
		scriptFile, string(cmd.Stage))
}

func (e *executor) Cleanup() {
	e.AbstractExecutor.Cleanup()

	// nothing to do, as there's no cleanup_script
	if e.Config.GenericScript == nil || e.Config.GenericScript.CleanupScript == "" {
		return
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), cleanupScriptTimeout)
	defer cancelFunc()

	err := e.runCommand(ctx, e.Config.GenericScript.CleanupScript)
	if err != nil {
		e.BuildLogger.Warningln("Cleanup script failed:", err)
	}
}

func init() {
	options := executors.ExecutorOptions{
		DefaultCustomBuildsDirEnabled: false,
		Shell: common.ShellScriptInfo{
			Shell:         "bash",
			Type:          common.NormalShell,
			RunnerCommand: "gitlab-runner",
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
	}

	common.RegisterExecutor("generic_script", executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		DefaultShellName: options.Shell.Shell,
	})
}
