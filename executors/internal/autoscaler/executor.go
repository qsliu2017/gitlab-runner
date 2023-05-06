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

type job struct {
	acquisition taskscaler.Acquisition
	holdUntil   time.Time
}

var jobsMux = sync.Mutex{}
var jobs = map[int64]*job{}

func getJob(jobID int64) *job {
	jobsMux.Lock()
	defer jobsMux.Unlock()
	j, ok := jobs[jobID]
	if !ok {
		j = &job{}
		jobs[jobID] = j
	}
	return j
}

func deleteJob(jobID int64) {
	jobsMux.Lock()
	defer jobsMux.Unlock()
	_, ok := jobs[jobID]
	if ok {
		delete(jobs, jobID)
	}
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

	j := getJob(options.Build.JobResponse.ID)
	j.acquisition = acq

	e.build.Log().WithField("key", acqRef.key).Trace("Acquired capacity...")

	acqRef.acq = acq

	return e.Executor.Prepare(options)
}

func (e *executor) Cleanup() {
	jobID := e.build.JobResponse.ID
	j := getJob(jobID)
	for {
		if time.Now().After(j.holdUntil) {
			deleteJob(jobID)
			e.Executor.Cleanup()
			return
		}
		time.Sleep(time.Second)
	}
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
			j := getJob(int64(jobID))
			j.holdUntil = time.Now().Add(30 * time.Minute)
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
			j := getJob(int64(jobID))
			j.holdUntil = time.Now()
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
			j := getJob(int64(jobID))
			if j.acquisition == nil {
				errorResponse("connect: no acquisition yet")
				return
			}
			connectInfo, err := j.acquisition.InstanceConnectInfo(context.Background(), publicKey)
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
