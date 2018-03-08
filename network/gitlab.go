package network

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"runtime"
	"strconv"
	"sync"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/core"
	"gitlab.com/gitlab-org/gitlab-runner/core/network"
)

const clientError = -100

type GitLabClient struct {
	clients map[string]*network.Client
	lock    sync.Mutex
}

func (n *GitLabClient) getClient(credentials network.RequestCredentials) (c *network.Client, err error) {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.clients == nil {
		n.clients = make(map[string]*network.Client)
	}
	key := fmt.Sprintf("%s_%s_%s_%s", credentials.GetURL(), credentials.GetToken(), credentials.GetTLSCAFile(), credentials.GetTLSCertFile())
	c = n.clients[key]
	if c == nil {
		c, err = network.NewClient(credentials)
		if err != nil {
			return
		}
		n.clients[key] = c
	}

	return
}

func (n *GitLabClient) getLastUpdate(credentials network.RequestCredentials) (lu string) {
	cli, err := n.getClient(credentials)
	if err != nil {
		return ""
	}
	return cli.GetLastUpdate()
}

func (n *GitLabClient) getRunnerVersion(config common.RunnerConfig) common.VersionInfo {
	info := common.VersionInfo{
		Name:         core.NAME,
		Version:      core.VERSION,
		Revision:     core.REVISION,
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
		Executor:     config.Executor,
	}

	if executor := common.GetExecutor(config.Executor); executor != nil {
		executor.GetFeatures(&info.Features)
	}

	if shell := common.GetShell(config.Shell); shell != nil {
		shell.GetFeatures(&info.Features)
	}

	return info
}

func (n *GitLabClient) doRaw(credentials network.RequestCredentials, method, uri string, request io.Reader, requestType string, headers http.Header) (res *http.Response, err error) {
	c, err := n.getClient(credentials)
	if err != nil {
		return nil, err
	}

	return c.Do(uri, method, request, requestType, headers)
}

func (n *GitLabClient) doJSON(credentials network.RequestCredentials, method, uri string, statusCode int, request interface{}, response interface{}) (int, string, network.ResponseTLSData) {
	c, err := n.getClient(credentials)
	if err != nil {
		return clientError, err.Error(), network.ResponseTLSData{}
	}

	return c.DoJSON(uri, method, statusCode, request, response)
}

func (n *GitLabClient) RegisterRunner(runner common.RunnerCredentials, description, tags string, runUntagged, locked bool) *common.RegisterRunnerResponse {
	// TODO: pass executor
	request := common.RegisterRunnerRequest{
		Token:       runner.Token,
		Description: description,
		Info:        n.getRunnerVersion(common.RunnerConfig{}),
		Locked:      locked,
		RunUntagged: runUntagged,
		Tags:        tags,
	}

	var response common.RegisterRunnerResponse
	result, statusText, _ := n.doJSON(&runner, "POST", "runners", http.StatusCreated, &request, &response)

	switch result {
	case http.StatusCreated:
		runner.Log().Println("Registering runner...", "succeeded")
		return &response
	case http.StatusForbidden:
		runner.Log().Errorln("Registering runner...", "forbidden (check registration token)")
		return nil
	case clientError:
		runner.Log().WithField("status", statusText).Errorln("Registering runner...", "error")
		return nil
	default:
		runner.Log().WithField("status", statusText).Errorln("Registering runner...", "failed")
		return nil
	}
}

func (n *GitLabClient) VerifyRunner(runner common.RunnerCredentials) bool {
	request := common.VerifyRunnerRequest{
		Token: runner.Token,
	}

	result, statusText, _ := n.doJSON(&runner, "POST", "runners/verify", http.StatusOK, &request, nil)

	switch result {
	case http.StatusOK:
		// this is expected due to fact that we ask for non-existing job
		runner.Log().Println("Verifying runner...", "is alive")
		return true
	case http.StatusForbidden:
		runner.Log().Errorln("Verifying runner...", "is removed")
		return false
	case clientError:
		runner.Log().WithField("status", statusText).Errorln("Verifying runner...", "error")
		return true
	default:
		runner.Log().WithField("status", statusText).Errorln("Verifying runner...", "failed")
		return true
	}
}

func (n *GitLabClient) UnregisterRunner(runner common.RunnerCredentials) bool {
	request := common.UnregisterRunnerRequest{
		Token: runner.Token,
	}

	result, statusText, _ := n.doJSON(&runner, "DELETE", "runners", http.StatusNoContent, &request, nil)

	const baseLogText = "Unregistering runner from GitLab"
	switch result {
	case http.StatusNoContent:
		runner.Log().Println(baseLogText, "succeeded")
		return true
	case http.StatusForbidden:
		runner.Log().Errorln(baseLogText, "forbidden")
		return false
	case clientError:
		runner.Log().WithField("status", statusText).Errorln(baseLogText, "error")
		return false
	default:
		runner.Log().WithField("status", statusText).Errorln(baseLogText, "failed")
		return false
	}
}

func addTLSData(response *common.JobResponse, tlsData network.ResponseTLSData) {
	if tlsData.CAChain != "" {
		response.TLSCAChain = tlsData.CAChain
	}

	if tlsData.CertFile != "" && tlsData.KeyFile != "" {
		data, err := ioutil.ReadFile(tlsData.CertFile)
		if err == nil {
			response.TLSAuthCert = string(data)
		}
		data, err = ioutil.ReadFile(tlsData.KeyFile)
		if err == nil {
			response.TLSAuthKey = string(data)
		}

	}
}

func (n *GitLabClient) RequestJob(config common.RunnerConfig) (*common.JobResponse, bool) {
	request := common.JobRequest{
		Info:       n.getRunnerVersion(config),
		Token:      config.Token,
		LastUpdate: n.getLastUpdate(&config.RunnerCredentials),
	}

	var response common.JobResponse
	result, statusText, tlsData := n.doJSON(&config.RunnerCredentials, "POST", "jobs/request", http.StatusCreated, &request, &response)

	switch result {
	case http.StatusCreated:
		config.Log().WithFields(logrus.Fields{
			"job":      strconv.Itoa(response.ID),
			"repo_url": response.RepoCleanURL(),
		}).Println("Checking for jobs...", "received")
		addTLSData(&response, tlsData)
		return &response, true
	case http.StatusForbidden:
		config.Log().Errorln("Checking for jobs...", "forbidden")
		return nil, false
	case http.StatusNoContent:
		config.Log().Debugln("Checking for jobs...", "nothing")
		return nil, true
	case clientError:
		config.Log().WithField("status", statusText).Errorln("Checking for jobs...", "error")
		return nil, false
	default:
		config.Log().WithField("status", statusText).Warningln("Checking for jobs...", "failed")
		return nil, true
	}
}

func (n *GitLabClient) UpdateJob(config common.RunnerConfig, jobCredentials *network.JobCredentials, jobInfo common.UpdateJobInfo) common.UpdateState {
	request := common.UpdateJobRequest{
		Info:          n.getRunnerVersion(config),
		Token:         jobCredentials.Token,
		State:         jobInfo.State,
		FailureReason: jobInfo.FailureReason,
		Trace:         jobInfo.Trace,
	}

	log := config.Log().WithField("job", jobInfo.ID)

	result, statusText, _ := n.doJSON(&config.RunnerCredentials, "PUT", fmt.Sprintf("jobs/%d", jobInfo.ID), http.StatusOK, &request, nil)
	switch result {
	case http.StatusOK:
		log.Debugln("Submitting job to coordinator...", "ok")
		return common.UpdateSucceeded
	case http.StatusNotFound:
		log.Warningln("Submitting job to coordinator...", "aborted")
		return common.UpdateAbort
	case http.StatusForbidden:
		log.WithField("status", statusText).Errorln("Submitting job to coordinator...", "forbidden")
		return common.UpdateAbort
	case clientError:
		log.WithField("status", statusText).Errorln("Submitting job to coordinator...", "error")
		return common.UpdateAbort
	default:
		log.WithField("status", statusText).Warningln("Submitting job to coordinator...", "failed")
		return common.UpdateFailed
	}
}

func (n *GitLabClient) PatchTrace(config common.RunnerConfig, jobCredentials *network.JobCredentials, tracePatch common.JobTracePatch) common.UpdateState {
	id := jobCredentials.ID

	contentRange := fmt.Sprintf("%d-%d", tracePatch.Offset(), tracePatch.Limit())
	headers := make(http.Header)
	headers.Set("Content-Range", contentRange)
	headers.Set("JOB-TOKEN", jobCredentials.Token)

	uri := fmt.Sprintf("jobs/%d/trace", id)
	request := bytes.NewReader(tracePatch.Patch())

	response, err := n.doRaw(&config.RunnerCredentials, "PATCH", uri, request, "text/plain", headers)
	if err != nil {
		config.Log().Errorln("Appending trace to coordinator...", "error", err.Error())
		return common.UpdateFailed
	}

	defer response.Body.Close()
	defer io.Copy(ioutil.Discard, response.Body)

	tracePatchResponse := NewTracePatchResponse(response)
	log := config.Log().WithFields(logrus.Fields{
		"job":        id,
		"sent-log":   contentRange,
		"job-log":    tracePatchResponse.RemoteRange,
		"job-status": tracePatchResponse.RemoteState,
		"code":       response.StatusCode,
		"status":     response.Status,
	})

	switch {
	case tracePatchResponse.IsAborted():
		log.Warningln("Appending trace to coordinator", "aborted")
		return common.UpdateAbort
	case response.StatusCode == http.StatusAccepted:
		log.Debugln("Appending trace to coordinator...", "ok")
		return common.UpdateSucceeded
	case response.StatusCode == http.StatusNotFound:
		log.Warningln("Appending trace to coordinator...", "not-found")
		return common.UpdateNotFound
	case response.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		log.Warningln("Appending trace to coordinator...", "range mismatch")
		tracePatch.SetNewOffset(tracePatchResponse.NewOffset())
		return common.UpdateRangeMismatch
	case response.StatusCode == clientError:
		log.Errorln("Appending trace to coordinator...", "error")
		return common.UpdateAbort
	default:
		log.Warningln("Appending trace to coordinator...", "failed")
		return common.UpdateFailed
	}
}

func (n *GitLabClient) ProcessJob(config common.RunnerConfig, jobCredentials *network.JobCredentials) common.JobTrace {
	trace := newJobTrace(n, config, jobCredentials)
	trace.start()
	return trace
}

func NewGitLabClient() *GitLabClient {
	return &GitLabClient{}
}
