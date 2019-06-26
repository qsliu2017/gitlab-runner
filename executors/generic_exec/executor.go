package generic_exec

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/generic_exec/process"
)

const (
	BuildFailureExitCode  = 1
	SystemFailureExitCode = 2

	BuildFailureExitCodeVariable  = "GENERIC_BUILD_FAILURE_EXIT_CODE"
	SystemFailureExitCodeVariable = "GENERIC_SYSTEM_FAILURE_EXIT_CODE"
)

type ErrUnknownFailure struct {
	Inner    error
	ExitCode int
}

func (e *ErrUnknownFailure) Error() string {
	return fmt.Sprintf("unknown Generic Executor script exit code %d; script execution terminated with: %v", e.ExitCode, e.Inner)
}

type executor struct {
	executors.AbstractExecutor

	config  *config
	tempDir string
}

func (e *executor) Prepare(options common.ExecutorPrepareOptions) error {
	err := e.AbstractExecutor.Prepare(options)
	if err != nil {
		return err
	}

	err = e.prepareConfig()
	if err != nil {
		return err
	}

	e.Println("Using GenericExec executor...")

	e.tempDir, err = ioutil.TempDir("", "generic-executor")
	if err != nil {
		return err
	}

	// nothing to do, as there's no prepare_script
	if e.config.PrepareExec == "" {
		return nil
	}

	ctx, cancelFunc := context.WithTimeout(e.Context, e.config.GetPrepareScriptTimeout())
	defer cancelFunc()

	return e.runCommand(ctx, e.config.PrepareExec)
}

func (e *executor) prepareConfig() error {
	if e.Config.GenericExec == nil {
		return common.MakeBuildError("Generic executor not configured")
	}

	e.config = &config{
		GenericExecConfig: e.Config.GenericExec,
	}

	if e.config.RunExec == "" {
		return common.MakeBuildError("Generic executor is missing RunExec")
	}

	return nil
}

func (e *executor) runCommand(ctx context.Context, script string, args ...string) error {
	scriptDef := strings.Split(script, " ")
	args = append(scriptDef[1:], args...)

	cmd := e.createCommand(scriptDef[0], args...)

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start command: %s", err)
	}

	waitCh := make(chan error)
	go e.waitForCommand(cmd, waitCh)

	select {
	case err = <-waitCh:
		return err

	case <-ctx.Done():
		return e.killAndWait(cmd, waitCh)
	}
}

func (e *executor) createCommand(cmd string, args ...string) *exec.Cmd {
	process := exec.Command(cmd, args...)
	process.Dir = e.tempDir
	process.Stdin = nil
	process.Stdout = e.Trace
	process.Stderr = e.Trace

	process.Env = os.Environ()

	defaultVariables := map[string]interface{}{
		"TMPDIR":                      e.tempDir,
		BuildFailureExitCodeVariable:  strconv.Itoa(BuildFailureExitCode),
		SystemFailureExitCodeVariable: strconv.Itoa(SystemFailureExitCode),
	}
	for key, value := range defaultVariables {
		process.Env = append(process.Env, fmt.Sprintf("%s=%s", key, value))
	}

	for _, variable := range e.Build.GetAllVariables().PublicOrInternal() {
		process.Env = append(process.Env, fmt.Sprintf("GENERIC_ENV_%s=%s", variable.Key, variable.Value))
	}

	return process
}

func (e *executor) waitForCommand(cmd *exec.Cmd, waitCh chan error) {
	err := cmd.Wait()

	if eerr, ok := err.(*exec.ExitError); ok {
		// TODO: simplify when we will update to Go 1.12. ExitStatus()
		//       is available there directly from err.Sys().
		exitCode := eerr.Sys().(syscall.WaitStatus).ExitStatus()

		if exitCode == BuildFailureExitCode {
			err = &common.BuildError{Inner: eerr}
		} else if exitCode != SystemFailureExitCode {
			err = &ErrUnknownFailure{Inner: eerr, ExitCode: exitCode}
		}
	}

	waitCh <- err
}

func (e *executor) killAndWait(cmd *exec.Cmd, waitCh chan error) error {
	if cmd.Process == nil {
		return errors.New("process not started yet")
	}

	log := e.BuildLogger.WithFields(logrus.Fields{"PID": cmd.Process.Pid})

	processKiller := process.NewKiller(log, cmd.Process)
	processKiller.Terminate()

	select {
	case err := <-waitCh:
		return err
	case <-time.After(e.config.GetProcessKillTimeout()):
		processKiller.ForceKill()

		select {
		case err := <-waitCh:
			return err
		}
	case <-time.After(e.config.GetProcessKillGracePeriod()):
		return errors.New("failed to kill process, likely process is dormant")
	}
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

	return e.runCommand(cmd.Context, e.config.RunExec, scriptFile, string(cmd.Stage))
}

func (e *executor) Cleanup() {
	e.AbstractExecutor.Cleanup()

	err := e.prepareConfig()
	if err != nil {
		// at this moment we don't care about the errors
		return
	}

	// nothing to do, as there's no cleanup_script
	if e.config.CleanupExec == "" {
		return
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), e.config.GetCleanupScriptTimeout())
	defer cancelFunc()

	err = e.runCommand(ctx, e.config.CleanupExec)
	if err != nil {
		e.BuildLogger.Warningln("Cleanup script failed:", err)
	}
}

func init() {
	options := executors.ExecutorOptions{
		DefaultCustomBuildsDirEnabled: false,
		DefaultBuildsDir:              "builds",
		DefaultCacheDir:               "cache",
		Shell: common.ShellScriptInfo{
			Shell:         common.GetDefaultShell(),
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

	common.RegisterExecutor("generic_exec", executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		DefaultShellName: options.Shell.Shell,
	})
}
