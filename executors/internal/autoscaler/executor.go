package autoscaler

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/connect"
)

type executor struct {
	common.Executor

	provider *provider
	build    *common.Build
	config   common.RunnerConfig
}

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

	j := connect.GetJob(options.Build.JobResponse.ID)
	j.Acquisition = acq

	e.build.Log().WithField("key", acqRef.key).Trace("Acquired capacity...")

	acqRef.acq = acq

	return e.Executor.Prepare(options)
}

func (e *executor) Cleanup() {
	jobID := e.build.JobResponse.ID
	connect.DeleteJob(jobID)
	e.Executor.Cleanup()
}

func init() {
	go func() {
		http.HandleFunc("/hold", func(w http.ResponseWriter, r *http.Request) {
			errorResponse := func(msg string) {
				w.WriteHeader(http.StatusInternalServerError) // whatever
				w.Write([]byte(msg))
			}
			jobIDString := r.URL.Query().Get("jobID")
			if jobIDString == "" {
				errorResponse("hold: jobID not provided")
				return
			}
			jobID, err := strconv.Atoi(jobIDString)
			if err != nil {
				errorResponse("hold: invalid jobID: " + jobIDString)
				return
			}
			j := connect.GetJob(int64(jobID))
			j.HoldUntil = time.Now().Add(30 * time.Minute)
			w.WriteHeader(http.StatusOK)
		})
		http.HandleFunc("/release", func(w http.ResponseWriter, r *http.Request) {
			errorResponse := func(msg string) {
				w.WriteHeader(http.StatusInternalServerError) // whatever
				w.Write([]byte(msg))
			}
			jobIDString := r.URL.Query().Get("jobID")
			if jobIDString == "" {
				errorResponse("release: jobID not provided")
				return
			}
			jobID, err := strconv.Atoi(jobIDString)
			if err != nil {
				errorResponse("release: invalid jobID: " + jobIDString)
				return
			}
			j := connect.GetJob(int64(jobID))
			j.HoldUntil = time.Now()
			w.WriteHeader(http.StatusOK)
		})
		http.HandleFunc("/connect", func(w http.ResponseWriter, r *http.Request) {
			errorResponse := func(msg string) {
				w.WriteHeader(http.StatusInternalServerError) // whatever
				w.Write([]byte(msg))
			}
			jobIDString := r.URL.Query().Get("jobID")
			if jobIDString == "" {
				errorResponse("connect: jobID not provided")
				return
			}
			jobID, err := strconv.Atoi(jobIDString)
			if err != nil {
				errorResponse("connect: invalid jobID: " + jobIDString)
				return
			}
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				errorResponse(err.Error())
				return
			}
			publicKey := strings.TrimSpace(string(body))
			j := connect.GetJob(int64(jobID))
			if j.Acquisition == nil {
				errorResponse("connect: no acquisition")
				return
			}
			connectInfo, err := j.Acquisition.InstanceConnectInfo(context.Background(), publicKey)
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
