package machine

import (
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/docker" // Force to load docker executor
	"gitlab.com/gitlab-org/gitlab-runner/referees"
)

const (
	DockerMachineExecutorStageUseMachine     common.ExecutorStage = "docker_machine_use_machine"
	DockerMachineExecutorStageReleaseMachine common.ExecutorStage = "docker_machine_release_machine"
)

type machineExecutor struct {
	machineProvider *machineProvider

	executor common.Executor
	build    *common.Build
	data     common.ExecutorData
	config   common.RunnerConfig

	currentStage common.ExecutorStage
}

func (e *machineExecutor) log() (log *logrus.Entry) {
	log = e.build.Log()

	machine, _ := e.build.ExecutorData.(*machineDetails)
	if machine == nil {
		machine, _ = e.data.(*machineDetails)
	}

	if machine != nil {
		log = log.WithFields(logrus.Fields{
			"name":      machine.Name,
			"usedcount": machine.UsedCount,
			"created":   machine.Created,
			"now":       time.Now(),
		})
	}

	if e.config.Docker != nil {
		log = log.WithField("docker", e.config.Docker.Host)
	}

	return
}

func (e *machineExecutor) Shell() *common.ShellScriptInfo {
	if e.executor == nil {
		return nil
	}
	return e.executor.Shell()
}

func (e *machineExecutor) Prepare(options common.ExecutorPrepareOptions) error {
	e.build = options.Build

	err := e.selectMachineForBuild(&options)
	if err != nil {
		return fmt.Errorf("couldn't select machine for build: %w", err)
	}

	err = e.createInnerExecutor(options)
	if err != nil {
		return fmt.Errorf("couldn't create inner executor: %w", err)
	}

	return nil
}

func (e *machineExecutor) selectMachineForBuild(options *common.ExecutorPrepareOptions) error {
	if options.Config.Docker == nil {
		options.Config.Docker = &common.DockerConfig{}
	}

	var err error

	// Use the machine
	e.SetCurrentStage(DockerMachineExecutorStageUseMachine)
	e.config, e.data, err = e.machineProvider.Use(options.Config, options.Build.ExecutorData)
	if err != nil {
		return fmt.Errorf("couldn't select machine: %w", err)
	}

	// assign Docker Credentials from chosen machine
	options.Config.Docker.Credentials = e.config.Docker.Credentials

	// TODO: Currently the docker-machine doesn't support multiple builds
	e.build.ProjectRunnerID = 0

	var machine *machineDetails

	machine, _ = options.Build.ExecutorData.(*machineDetails)
	if machine == nil {
		machine, _ = e.data.(*machineDetails)
	}

	if machine != nil {
		options.Build.Hostname = machine.Name
	}

	// e.data is only set if the docker-machine created is new
	if e.data == nil {
		e.log().Infoln("Using existing docker-machine")
	} else {
		e.log().Infoln("Created docker-machine")
	}

	return nil
}

func (e *machineExecutor) createInnerExecutor(options common.ExecutorPrepareOptions) error {
	e.executor = e.machineProvider.createExecutor()
	if e.executor == nil {
		return errors.New("failed to create an executor")
	}

	err := e.executor.Prepare(options)
	if err != nil {
		e.log().Infoln("Preparing docker-machine wrapped executor failed")
		return err
	}

	e.log().Infoln("Starting docker-machine build...")

	return nil
}

func (e *machineExecutor) Run(cmd common.ExecutorCommand) error {
	if e.executor == nil {
		return errors.New("missing executor")
	}

	return e.executor.Run(cmd)
}

func (e *machineExecutor) Finish(err error) {
	if e.executor != nil {
		e.executor.Finish(err)
	}

	e.log().Infoln("Finished docker-machine build:", err)
}

func (e *machineExecutor) Cleanup() {
	// Cleanup executor if were created
	if e.executor != nil {
		e.executor.Cleanup()
	}

	e.log().Infoln("Cleaned up docker-machine")

	// Release allocated machine
	if e.data == nil {
		return
	}

	// Release allocated machine
	e.SetCurrentStage(DockerMachineExecutorStageReleaseMachine)
	e.machineProvider.Release(&e.config, e.data)
	e.data = nil
}

func (e *machineExecutor) GetCurrentStage() common.ExecutorStage {
	if e.executor == nil {
		return common.ExecutorStage("")
	}

	return e.executor.GetCurrentStage()
}

func (e *machineExecutor) SetCurrentStage(stage common.ExecutorStage) {
	if e.executor == nil {
		e.currentStage = stage
		return
	}

	e.executor.SetCurrentStage(stage)
}

func (e *machineExecutor) GetMetricsSelector() string {
	refereed, ok := e.executor.(referees.MetricsExecutor)
	if !ok {
		return ""
	}

	return refereed.GetMetricsSelector()
}

func init() {
	common.RegisterExecutorProvider("docker+machine", newMachineProvider("docker+machine", "docker"))
	common.RegisterExecutorProvider("docker-ssh+machine", newMachineProvider("docker-ssh+machine", "docker-ssh"))
}
