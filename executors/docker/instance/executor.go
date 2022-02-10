package instance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/referees"
)

type executor struct {
	provider *instanceProvider
	executor common.Executor
	build    *common.Build
	data     common.ExecutorData
	config   common.RunnerConfig

	currentStage common.ExecutorStage
}

func (e *executor) Shell() *common.ShellScriptInfo {
	if e.executor == nil {
		return nil
	}

	return e.executor.Shell()
}

func (e *executor) Prepare(options common.ExecutorPrepareOptions) (err error) {
	e.build = options.Build
	e.build.Log().Infoln("Preparing instance...")

	acqRef, ok := options.Build.ExecutorData.(*acqusitionRef)
	if !ok {
		return fmt.Errorf("no acqusition ref data")
	}

	// generate key for acqusition
	key := options.Build.Token + strconv.Itoa(options.Build.ID)

	// update acquisition reference key
	acqRef.Set(key)

	// todo: allow configuration of how long we're willing to wait for.
	// Or is this already handled by the option's context?
	ctx, cancel := context.WithTimeout(options.Context, 5*time.Minute)
	defer cancel()

	acq, err := e.provider.getRunnerTaskscaler(options.Config).Acquire(ctx, key)
	if err != nil {
		return fmt.Errorf("unable to acquire instance: %w", err)
	}

	if options.Config.Docker == nil {
		options.Config.Docker = &common.DockerConfig{}
	}

	hash := sha256.Sum256([]byte(acq.InstanceID()))

	// todo: improve how we're setting this up, inc. configuration of port
	e.config = *options.Config
	e.config.Docker.TLSVerify = true
	e.config.Docker.CertPath = filepath.Join(os.TempDir(), hex.EncodeToString(hash[:]))
	e.build.Hostname = acq.InstanceID()
	e.config.Docker.Host = "tcp://" + acq.InstanceConnectInfo().ExternalAddr + ":443"

	// Create original executor
	e.build.Log().Infoln("Creating underlying docker executor...")
	e.executor = e.provider.ExecutorProvider.Create()
	if e.executor == nil {
		return errors.New("failed to create an executor")
	}

	if err = e.executor.Prepare(options); err != nil {
		e.build.Log().Infoln("Preparing docker-instance wrapped executor failed")
		return err
	}

	e.build.Log().Infoln("Starting docker-instance build...")

	return nil
}

func (e *executor) Run(cmd common.ExecutorCommand) error {
	if e.executor == nil {
		return errors.New("missing executor")
	}
	return e.executor.Run(cmd)
}

func (e *executor) Finish(err error) {
	if e.executor != nil {
		e.executor.Finish(err)
	}
	e.build.Log().Infoln("Finished docker-instance build:", err)
}

func (e *executor) Cleanup() {
	// Cleanup executor if were created
	if e.executor != nil {
		e.executor.Cleanup()
	}

	e.build.Log().Infoln("Cleaned up docker-instance")

	// Release acqusition
	if e.data != nil {
		e.provider.Release(&e.config, e.data)
		e.data = nil
	}
}

func (e *executor) GetCurrentStage() common.ExecutorStage {
	if e.executor == nil {
		return common.ExecutorStage("")
	}

	return e.executor.GetCurrentStage()
}

func (e *executor) SetCurrentStage(stage common.ExecutorStage) {
	if e.executor == nil {
		e.currentStage = stage
		return
	}

	e.executor.SetCurrentStage(stage)
}

func (e *executor) GetMetricsSelector() string {
	refereed, ok := e.executor.(referees.MetricsExecutor)
	if !ok {
		return ""
	}

	return refereed.GetMetricsSelector()
}
