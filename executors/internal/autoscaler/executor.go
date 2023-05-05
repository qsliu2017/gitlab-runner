package autoscaler

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"gitlab.com/gitlab-org/fleeting/taskscaler"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type executor struct {
	common.Executor

	provider *provider
	build    *common.Build
	config   common.RunnerConfig
}

var jobsMux = sync.Mutex{}
var jobs = map[int64]taskscaler.Acquisition{}

func (e *executor) Prepare(options common.ExecutorPrepareOptions) (err error) {
	e.build = options.Build
	e.config = *options.Config

	e.build.Log().Infoln("Preparing instance...")

	acqRef, ok := options.Build.ExecutorData.(*acquisitionRef)
	if !ok {
		return fmt.Errorf("no acquisition ref data")
	}

	// if we already have an acquisition just retry preparing it
	if acqRef.acq != nil {
		return e.Executor.Prepare(options)
	}

	// todo: allow configuration of how long we're willing to wait for.
	// Or is this already handled by the option's context?
	ctx, cancel := context.WithTimeout(options.Context, 5*time.Minute)
	defer cancel()

	acq, err := e.provider.getRunnerTaskscaler(options.Config).Acquire(ctx, acqRef.key)
	if err != nil {
		return fmt.Errorf("unable to acquire instance: %w", err)
	}

	jobsMux.Lock()
	jobID := options.Build.JobResponse.ID
	jobs[jobID] = acq
	jobsMux.Unlock()

	e.build.Log().WithField("key", acqRef.key).Trace("Acquired capacity...")

	acqRef.acq = acq

	return e.Executor.Prepare(options)
}

func (e *executor) Cleanup() {
	jobsMux.Lock()
	jobID := e.build.JobResponse.ID
	delete(jobs, jobID)
	jobsMux.Unlock()
	e.Executor.Cleanup()
}

func init() {
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			errorResponse := func(msg string) {
				w.WriteHeader(http.StatusInternalServerError) // whatever
				w.Write([]byte(msg))
			}
			jobIDString := r.URL.Query().Get("jobID")
			if jobIDString == "" {
				errorResponse("jobID not provided")
				return
			}
			jobID, err := strconv.Atoi(jobIDString)
			if err != nil {
				errorResponse("invalid jobID: " + jobIDString)
				return
			}
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				errorResponse(err.Error())
				return
			}
			publicKey := strings.TrimSpace(string(body))
			jobsMux.Lock()
			acq, ok := jobs[int64(jobID)]
			jobsMux.Unlock()
			if !ok {
				errorResponse("jobID not found")
				return
			}
			connectInfo, err := acq.InstanceConnectInfo(context.Background(), publicKey)
			if err != nil {
				errorResponse(err.Error())
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(connectInfo.ExternalAddr))
		})
		log.Fatal(http.ListenAndServe(":12345", nil))
	}()
}
