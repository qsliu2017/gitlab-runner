package generic_fd

// TODO: started, but not working, nor finished

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

type commandResponse struct {
	Index    int64
	Response string
	Args     []string
}

type command struct {
	Index    int64
	Cmd      string
	Args     []string
	Response chan commandResponse
}

type executor struct {
	executors.AbstractExecutor

	tempDir      string
	process      *exec.Cmd
	commandFile  *os.File // FD: 3
	responseFile *os.File // FD: 4

	cmdSender   chan command
	cmdReceiver chan commandResponse
}

func (s *executor) newCommand() command {
	return command{
		Response: make(chan commandResponse),
	}
}

func (s *executor) sendCommand(cmd command) {
	s.cmdSender <- cmd
}

func (s *executor) handleCmdReceiver() {
	r := bufio.NewReader(s.responseFile)

	for {
		line, _, err := r.ReadLine()
		if err != nil {
			s.BuildLogger.Warningln("Received invalid line from process:", err)
			continue
		}

		// TODO: it does not handle escaping
		split := strings.Split(string(line), " ")
		if len(split) < 2 {
			s.BuildLogger.Warningln("Received invalid line from process:", string(line))
			continue
		}

		index, err := strconv.ParseInt(split[0], 0, 0)
		if err != nil {
			s.BuildLogger.Warningln("Received invalid line from process:", string(line), err)
			continue
		}

		s.cmdReceiver <- commandResponse{
			Index:    index,
			Response: split[1],
			Args:     split[2:],
		}
	}
}

func (s *executor) handleCmdSender() {
	w := bufio.NewWriter(s.cmdReceiver)

	for {
		select {
			case cmd := <-s.cmdSender {
				
			}
		}
	}
}

func (s *executor) Prepare(options common.ExecutorPrepareOptions) error {
	err := s.AbstractExecutor.Prepare(options)
	if err != nil {
		return err
	}

	s.Println("Using Generic executor...")

	if s.Config.Generic == nil {
		return common.MakeBuildError("Generic executor not configured")
	}

	if s.Config.Generic.Handler == "" {
		return common.MakeBuildError("Generic executor handler not configured")
	}

	s.tempDir, err = ioutil.TempDir("", "generic-executor")
	if err != nil {
		return err
	}

	s.commandFile, err = ioutil.TempFile(s.tempDir, "command-file")
	if err != nil {
		return err
	}

	s.responseFile, err = ioutil.TempFile(s.tempDir, "response-file")
	if err != nil {
		return err
	}

	s.process = exec.Command(s.Config.Generic.Handler)
	s.process.Env = os.Environ()
	s.process.Env = append(s.process.Env, "TMPDIR="+s.tempDir)
	for _, variable := range s.Build.GetAllVariables().PublicOrInternal() {
		s.process.Env = append(s.process.Env, "GENERIC_ENV_"+variable.Key+"="+variable.Value)
	}
	s.process.Stdin = nil
	s.process.Stdout = s.Trace
	s.process.Stderr = s.Trace
	s.process.ExtraFiles = []*os.File{
		s.commandFile,
		s.responseFile,
	}

	err = s.process.Start()
	if err != nil {
		return err
	}

	s.cmdSender = make(chan commandSender)
	go s.monitorProcess()

	return nil
}

func (s *executor) killAndWait(cmd *exec.Cmd, waitCh chan error) error {
	for {
		s.Debugln("Aborting command...")
		helpers.KillProcessGroup(cmd)
		select {
		case <-time.After(time.Second):
		case err := <-waitCh:
			return err
		}
	}
}

func (s *executor) Run(cmd common.ExecutorCommand) error {
	// Create execution command
	c := exec.Command(s.BuildShell.Command, s.BuildShell.Arguments...)
	if c == nil {
		return errors.New("Failed to generate execution command")
	}

	helpers.SetProcessGroup(c)
	defer helpers.KillProcessGroup(c)

	// Fill process environment variables
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
		return fmt.Errorf("Failed to start process: %s", err)
	}

	// Wait for process to finish
	waitCh := make(chan error)
	go func() {
		err := c.Wait()
		if _, ok := err.(*exec.ExitError); ok {
			err = &common.BuildError{Inner: err}
		}
		waitCh <- err
	}()

	// Support process abort
	select {
	case err = <-waitCh:
		return err

	case <-cmd.Context.Done():
		return s.killAndWait(c, waitCh)
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

	common.RegisterExecutor("generic", executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		DefaultShellName: options.Shell.Shell,
	})
}
