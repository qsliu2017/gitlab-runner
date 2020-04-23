package network

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/network/internal/response"
)

const clientError = -100

var apiRequestStatuses = prometheus.NewDesc(
	"gitlab_runner_api_request_statuses_total",
	"The total number of api requests, partitioned by runner, endpoint and status.",
	[]string{"runner", "endpoint", "status"},
	nil,
)

type APIEndpoint string

const (
	APIEndpointRequestJob APIEndpoint = "request_job"
	APIEndpointUpdateJob  APIEndpoint = "update_job"
	APIEndpointPatchTrace APIEndpoint = "patch_trace"
)

type apiRequestStatusPermutation struct {
	runnerID string
	endpoint APIEndpoint
	status   int
}

type APIRequestStatusesMap struct {
	internal map[apiRequestStatusPermutation]int
	lock     sync.RWMutex
}

func (arspm *APIRequestStatusesMap) Append(runnerID string, endpoint APIEndpoint, status int) {
	arspm.lock.Lock()
	defer arspm.lock.Unlock()

	permutation := apiRequestStatusPermutation{runnerID: runnerID, endpoint: endpoint, status: status}

	if _, ok := arspm.internal[permutation]; !ok {
		arspm.internal[permutation] = 0
	}

	arspm.internal[permutation]++
}

// Describe implements prometheus.Collector.
func (arspm *APIRequestStatusesMap) Describe(ch chan<- *prometheus.Desc) {
	ch <- apiRequestStatuses
}

// Collect implements prometheus.Collector.
func (arspm *APIRequestStatusesMap) Collect(ch chan<- prometheus.Metric) {
	arspm.lock.RLock()
	defer arspm.lock.RUnlock()

	for permutation, count := range arspm.internal {
		ch <- prometheus.MustNewConstMetric(
			apiRequestStatuses,
			prometheus.CounterValue,
			float64(count),
			permutation.runnerID,
			string(permutation.endpoint),
			strconv.Itoa(permutation.status),
		)
	}
}

func NewAPIRequestStatusesMap() *APIRequestStatusesMap {
	return &APIRequestStatusesMap{
		internal: make(map[apiRequestStatusPermutation]int),
	}
}

type GitLabClient struct {
	clients map[string]*client
	lock    sync.Mutex

	requestsStatusesMap *APIRequestStatusesMap
}

func (n *GitLabClient) getClient(credentials requestCredentials) (c *client, err error) {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.clients == nil {
		n.clients = make(map[string]*client)
	}
	key := fmt.Sprintf("%s_%s_%s_%s", credentials.GetURL(), credentials.GetToken(), credentials.GetTLSCAFile(), credentials.GetTLSCertFile())
	c = n.clients[key]
	if c == nil {
		c, err = newClient(credentials)
		if err != nil {
			return
		}
		n.clients[key] = c
	}

	return
}

func (n *GitLabClient) getLastUpdate(credentials requestCredentials) (lu string) {
	cli, err := n.getClient(credentials)
	if err != nil {
		return ""
	}
	return cli.getLastUpdate()
}

func (n *GitLabClient) getRunnerVersion(config common.RunnerConfig) common.VersionInfo {
	info := common.VersionInfo{
		Name:         common.NAME,
		Version:      common.VERSION,
		Revision:     common.REVISION,
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
		Executor:     config.Executor,
		Shell:        config.Shell,
	}

	if executorProvider := common.GetExecutorProvider(config.Executor); executorProvider != nil {
		executorProvider.GetFeatures(&info.Features)

		if info.Shell == "" {
			info.Shell = executorProvider.GetDefaultShell()
		}
	}

	if shell := common.GetShell(info.Shell); shell != nil {
		shell.GetFeatures(&info.Features)
	}

	return info
}

func (n *GitLabClient) doRaw(credentials requestCredentials, method, uri string, request io.Reader, requestType string, headers http.Header) (*response.Response, error) {
	c, err := n.getClient(credentials)
	if err != nil {
		return nil, err
	}

	return c.do(uri, method, request, requestType, headers)
}

func (n *GitLabClient) doJSON(credentials requestCredentials, method, uri string, expectedStatusCode int, request interface{}, result interface{}) *response.Response {
	c, err := n.getClient(credentials)
	if err != nil {
		return response.NewSimple(clientError, err.Error())
	}

	return c.doJSON(uri, method, expectedStatusCode, request, result)
}

func (n *GitLabClient) getResponseTLSData(credentials requestCredentials, response *response.Response) (ResponseTLSData, error) {
	c, err := n.getClient(credentials)
	if err != nil {
		return ResponseTLSData{}, fmt.Errorf("couldn't get client: %w", err)
	}

	return c.getResponseTLSData(response.TLS())
}

func (n *GitLabClient) RegisterRunner(runner common.RunnerCredentials, parameters common.RegisterRunnerParameters) *common.RegisterRunnerResponse {
	// TODO: pass executor
	request := common.RegisterRunnerRequest{
		RegisterRunnerParameters: parameters,
		Token:                    runner.Token,
		Info:                     n.getRunnerVersion(common.RunnerConfig{}),
	}

	var result common.RegisterRunnerResponse
	httpResponse := n.doJSON(&runner, http.MethodPost, "runners", http.StatusCreated, &request, &result)

	responseHandler := response.NewHandler(runner.Log(), "Registering runner...")
	defer responseHandler.Flush()

	responseHandler.SetResponse(httpResponse)
	responseHandler.WhenCodeIs(http.StatusCreated).
		LogResultAs("succeeded").
		WithHandlerFn(response.IdentityHandlerFn(&result))
	responseHandler.WhenCodeIs(http.StatusForbidden).
		LogResultAs("forbidden (check registration token)")
	responseHandler.WhenCodeIs(clientError).
		LogResultAs("error")
	responseHandler.InDefaultCase().
		LogResultAs("failed")

	r, ok := responseHandler.Handle().(*common.RegisterRunnerResponse)
	if !ok {
		return nil
	}

	return r
}

func (n *GitLabClient) VerifyRunner(runner common.RunnerCredentials) bool {
	request := common.VerifyRunnerRequest{
		Token: runner.Token,
	}

	httpResponse := n.doJSON(&runner, http.MethodPost, "runners/verify", http.StatusOK, &request, nil)

	responseHandler := response.NewHandler(runner.Log(), "Verifying runner...")
	defer responseHandler.Flush()

	responseHandler.SetResponse(httpResponse)
	responseHandler.WhenCodeIs(http.StatusOK).
		LogResultAs("is alive").
		WithHandlerFn(response.IdentityHandlerFn(true))
	responseHandler.WhenCodeIs(http.StatusForbidden).
		LogResultAs("is removed")
	responseHandler.WhenCodeIs(clientError).
		LogResultAs("error").
		WithHandlerFn(response.IdentityHandlerFn(true))
	responseHandler.InDefaultCase().
		LogResultAs("failed").
		WithHandlerFn(response.IdentityHandlerFn(true))

	r, ok := responseHandler.Handle().(bool)
	if !ok {
		return false
	}

	return r
}

func (n *GitLabClient) UnregisterRunner(runner common.RunnerCredentials) bool {
	request := common.UnregisterRunnerRequest{
		Token: runner.Token,
	}

	httpResponse := n.doJSON(&runner, http.MethodDelete, "runners", http.StatusNoContent, &request, nil)

	responseHandler := response.NewHandler(runner.Log(), "Unregistering runner from GitLab")
	defer responseHandler.Flush()

	responseHandler.SetResponse(httpResponse)
	responseHandler.WhenCodeIs(http.StatusNoContent).
		LogResultAs("succeeded").
		WithHandlerFn(response.IdentityHandlerFn(true))
	responseHandler.WhenCodeIs(http.StatusForbidden).
		LogResultAs("forbidden")
	responseHandler.WhenCodeIs(clientError).
		LogResultAs("error")
	responseHandler.InDefaultCase().
		LogResultAs("failed")

	r, ok := responseHandler.Handle().(bool)
	if !ok {
		return false
	}

	return r
}

func addTLSData(response *common.JobResponse, tlsData ResponseTLSData) {
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

func (n *GitLabClient) RequestJob(config common.RunnerConfig, sessionInfo *common.SessionInfo) (*common.JobResponse, bool) {
	request := common.JobRequest{
		Info:       n.getRunnerVersion(config),
		Token:      config.Token,
		LastUpdate: n.getLastUpdate(&config.RunnerCredentials),
		Session:    sessionInfo,
	}

	var result common.JobResponse
	httpResponse := n.doJSON(&config.RunnerCredentials, http.MethodPost, "jobs/request", http.StatusCreated, &request, &result)

	n.requestsStatusesMap.Append(config.RunnerCredentials.ShortDescription(), APIEndpointRequestJob, httpResponse.StatusCode())

	type handlerResult struct {
		jobResponse *common.JobResponse
		healthy     bool
	}

	newHandlerResult := func(jobResponse *common.JobResponse, healthy bool) *handlerResult {
		return &handlerResult{
			jobResponse: jobResponse,
			healthy:     healthy,
		}
	}

	newHandlerResultHandlerFn := func(jobResponse *common.JobResponse, healthy bool) response.HandlerFn {
		return func(_ logrus.FieldLogger) interface{} {
			return newHandlerResult(jobResponse, healthy)
		}
	}

	responseHandler := response.NewHandler(config.Log(), "Checking for jobs...")
	defer responseHandler.Flush()

	responseHandler.SetResponse(httpResponse)
	responseHandler.WhenCodeIs(http.StatusCreated).
		LogResultAs("received").
		WithLogFields(logrus.Fields{
			"job":      result.ID,
			"repo_url": result.RepoCleanURL(),
		}).
		WithHandlerFn(func(log logrus.FieldLogger) interface{} {
			tlsData, err := n.getResponseTLSData(&config.RunnerCredentials, httpResponse)
			if err != nil {
				log.WithError(err).
					Errorln("Error on fetching TLS Data from API response...")
			}
			addTLSData(&result, tlsData)

			return newHandlerResult(&result, true)
		})
	responseHandler.WhenCodeIs(http.StatusForbidden).
		LogResultAs("forbidden").
		WithHandlerFn(newHandlerResultHandlerFn(nil, false))
	responseHandler.WhenCodeIs(http.StatusNoContent).
		LogResultAs("nothing").
		WithLogLevel(logrus.DebugLevel).
		WithHandlerFn(newHandlerResultHandlerFn(nil, true))
	responseHandler.WhenCodeIs(clientError).
		LogResultAs("error").
		WithHandlerFn(newHandlerResultHandlerFn(nil, false))
	responseHandler.InDefaultCase().
		LogResultAs("failed").
		WithHandlerFn(newHandlerResultHandlerFn(nil, true))

	responseHandlerResult := responseHandler.Handle()
	r, ok := responseHandlerResult.(*handlerResult)
	if !ok {
		return nil, false
	}

	return r.jobResponse, r.healthy
}

func (n *GitLabClient) UpdateJob(config common.RunnerConfig, jobCredentials *common.JobCredentials, jobInfo common.UpdateJobInfo) common.UpdateState {
	request := common.UpdateJobRequest{
		Info:          n.getRunnerVersion(config),
		Token:         jobCredentials.Token,
		State:         jobInfo.State,
		FailureReason: jobInfo.FailureReason,
	}

	httpResponse := n.doJSON(&config.RunnerCredentials, http.MethodPut, fmt.Sprintf("jobs/%d", jobInfo.ID), http.StatusOK, &request, nil)

	n.requestsStatusesMap.Append(config.RunnerCredentials.ShortDescription(), APIEndpointUpdateJob, httpResponse.StatusCode())

	remoteJobStateResponse := NewRemoteJobStateResponse(httpResponse)
	log := config.Log().WithFields(logrus.Fields{
		"job":       jobInfo.ID,
		"jobStatus": remoteJobStateResponse.RemoteState,
	})

	responseHandler := response.NewHandler(log, "Submitting job to coordinator...")
	defer responseHandler.Flush()

	responseHandler.SetResponse(httpResponse)

	if remoteJobStateResponse.IsAborted() {
		responseHandler.Log(logrus.WarnLevel, "aborted")
		return common.UpdateAbort
	}

	abortHandlerFn := response.IdentityHandlerFn(common.UpdateAbort)

	responseHandler.WhenCodeIs(http.StatusOK).
		LogResultAs("ok").
		WithLogLevel(logrus.DebugLevel).
		WithHandlerFn(response.IdentityHandlerFn(common.UpdateSucceeded))
	responseHandler.WhenCodeIs(http.StatusNotFound).
		LogResultAs("aborted").
		WithLogLevel(logrus.WarnLevel).
		WithHandlerFn(abortHandlerFn)
	responseHandler.WhenCodeIs(http.StatusForbidden).
		LogResultAs("forbidden").
		WithHandlerFn(abortHandlerFn)
	responseHandler.WhenCodeIs(clientError).
		LogResultAs("error").
		WithHandlerFn(abortHandlerFn)
	responseHandler.InDefaultCase().
		LogResultAs("failed").
		WithHandlerFn(response.IdentityHandlerFn(common.UpdateFailed))

	r, ok := responseHandler.Handle().(common.UpdateState)
	if !ok {
		return common.UpdateFailed
	}

	return r
}

func (n *GitLabClient) PatchTrace(config common.RunnerConfig, jobCredentials *common.JobCredentials, content []byte, startOffset int) common.PatchTraceResult {
	id := jobCredentials.ID

	baseLog := config.Log().WithField("job", id)
	responseHandler := response.NewHandler(baseLog, "Appending trace to coordinator...")
	defer responseHandler.Flush()

	if len(content) == 0 {
		responseHandler.Log(logrus.DebugLevel, "skipped due to empty patch")
		return common.NewPatchTraceResult(startOffset, common.UpdateSucceeded, 0)
	}

	endOffset := startOffset + len(content)
	contentRange := fmt.Sprintf("%d-%d", startOffset, endOffset-1)

	headers := make(http.Header)
	headers.Set("Content-Range", contentRange)
	headers.Set("JOB-TOKEN", jobCredentials.Token)

	uri := fmt.Sprintf("jobs/%d/trace", id)
	request := bytes.NewReader(content)

	httpResponse, err := n.doRaw(&config.RunnerCredentials, http.MethodPatch, uri, request, "text/plain", headers)
	responseHandler.SetResponse(httpResponse)

	if err != nil {
		responseHandler.AddLogError(err).Log(logrus.ErrorLevel, "error")
		return common.NewPatchTraceResult(startOffset, common.UpdateFailed, 0)
	}

	n.requestsStatusesMap.Append(config.RunnerCredentials.ShortDescription(), APIEndpointPatchTrace, httpResponse.StatusCode())

	tracePatchResponse := NewTracePatchResponse(httpResponse, baseLog)
	responseHandler.AddLogFields(logrus.Fields{
		"sent-log":        contentRange,
		"job-log":         tracePatchResponse.RemoteRange,
		"job-status":      tracePatchResponse.RemoteState,
		"update-interval": tracePatchResponse.RemoteTraceUpdateInterval,
	})

	result := common.PatchTraceResult{
		SentOffset:        startOffset,
		NewUpdateInterval: tracePatchResponse.RemoteTraceUpdateInterval,
	}

	if tracePatchResponse.IsAborted() {
		responseHandler.Log(logrus.WarnLevel, "aborted")
		result.State = common.UpdateAbort
		return result
	}

	responseHandler.WhenCodeIs(http.StatusAccepted).
		LogResultAs("ok").
		WithLogLevel(logrus.DebugLevel).
		WithHandlerFn(func(_ logrus.FieldLogger) interface{} {
			result.SentOffset = endOffset
			result.State = common.UpdateSucceeded
			return result
		})
	responseHandler.WhenCodeIs(http.StatusNotFound).
		LogResultAs("not-found").
		WithHandlerFn(func(_ logrus.FieldLogger) interface{} {
			result.State = common.UpdateNotFound
			return result
		})
	responseHandler.WhenCodeIs(http.StatusRequestedRangeNotSatisfiable).
		LogResultAs("range mismatch").
		WithLogLevel(logrus.WarnLevel).
		WithHandlerFn(func(_ logrus.FieldLogger) interface{} {
			result.SentOffset = tracePatchResponse.NewOffset()
			result.State = common.UpdateRangeMismatch
			return result
		})
	responseHandler.WhenCodeIs(clientError).
		LogResultAs("error").
		WithHandlerFn(func(_ logrus.FieldLogger) interface{} {
			result.State = common.UpdateAbort
			return result
		})
	responseHandler.InDefaultCase().
		LogResultAs("failed").
		WithHandlerFn(func(_ logrus.FieldLogger) interface{} {
			result.State = common.UpdateFailed
			return result
		})

	r, ok := responseHandler.Handle().(common.PatchTraceResult)
	if !ok {
		result.State = common.UpdateFailed
		return result
	}

	return r
}

func (n *GitLabClient) createArtifactsForm(mpw *multipart.Writer, reader io.Reader, baseName string) error {
	wr, err := mpw.CreateFormFile("file", baseName)
	if err != nil {
		return err
	}

	_, err = io.Copy(wr, reader)
	if err != nil {
		return err
	}

	return nil
}

func uploadRawArtifactsQuery(options common.ArtifactsOptions) url.Values {
	q := url.Values{}

	if options.ExpireIn != "" {
		q.Set("expire_in", options.ExpireIn)
	}

	if options.Format != "" {
		q.Set("artifact_format", string(options.Format))
	}

	if options.Type != "" {
		q.Set("artifact_type", options.Type)
	}

	return q
}

func (n *GitLabClient) UploadRawArtifacts(config common.JobCredentials, reader io.Reader, options common.ArtifactsOptions) common.UploadState {
	pr, pw := io.Pipe()
	defer pr.Close()

	mpw := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer mpw.Close()
		err := n.createArtifactsForm(mpw, reader, options.BaseName)
		if err != nil {
			pw.CloseWithError(err)
		}
	}()

	query := uploadRawArtifactsQuery(options)

	headers := make(http.Header)
	headers.Set("JOB-TOKEN", config.Token)
	httpResponse, err := n.doRaw(&config, http.MethodPost, fmt.Sprintf("jobs/%d/artifacts?%s", config.ID, query.Encode()), pr, mpw.FormDataContentType(), headers)

	log := logrus.WithFields(logrus.Fields{
		"id":    config.ID,
		"token": helpers.ShortenToken(config.Token),
	})

	messagePrefix := "Uploading artifacts to coordinator..."
	if options.Type != "" {
		messagePrefix = fmt.Sprintf("Uploading artifacts as %q to coordinator...", options.Type)
	}

	responseHandler := response.NewHandler(log, messagePrefix)
	defer responseHandler.Flush()

	responseHandler.SetResponse(httpResponse)

	if err != nil {
		responseHandler.AddLogError(err).Log(logrus.ErrorLevel, "error")
		return common.UploadFailed
	}

	responseHandler.WhenCodeIs(http.StatusCreated).
		LogResultAs("ok").
		WithHandlerFn(response.IdentityHandlerFn(common.UploadSucceeded))
	responseHandler.WhenCodeIs(http.StatusForbidden).
		LogResultAs("forbidden").
		WithHandlerFn(response.IdentityHandlerFn(common.UploadForbidden))
	responseHandler.WhenCodeIs(http.StatusRequestEntityTooLarge).
		LogResultAs("too large archive").
		WithLogLevel(logrus.WarnLevel).
		WithHandlerFn(response.IdentityHandlerFn(common.UploadTooLarge))
	responseHandler.WhenCodeIs(http.StatusServiceUnavailable).
		LogResultAs("service unavailable").
		WithHandlerFn(response.IdentityHandlerFn(common.UploadServiceUnavailable))
	responseHandler.InDefaultCase().
		LogResultAs("failed").
		WithHandlerFn(response.IdentityHandlerFn(common.UploadFailed))

	r, ok := responseHandler.Handle().(common.UploadState)
	if !ok {
		return common.UploadFailed
	}

	return r
}

func (n *GitLabClient) DownloadArtifacts(config common.JobCredentials, artifactsFile string, directDownload *bool) common.DownloadState {
	query := url.Values{}
	if directDownload != nil {
		query.Set("direct_download", strconv.FormatBool(*directDownload))
	}

	headers := make(http.Header)
	headers.Set("JOB-TOKEN", config.Token)
	uri := fmt.Sprintf("jobs/%d/artifacts?%s", config.ID, query.Encode())

	httpResponse, err := n.doRaw(&config, http.MethodGet, uri, nil, "", headers)

	log := logrus.WithFields(logrus.Fields{
		"id":    config.ID,
		"token": helpers.ShortenToken(config.Token),
	})

	responseHandler := response.NewHandler(log, "Downloading artifacts from coordinator...")
	defer responseHandler.Flush()

	responseHandler.SetResponse(httpResponse)

	if err != nil {
		responseHandler.AddLogError(err).Log(logrus.ErrorLevel, "error")
		return common.DownloadFailed
	}

	responseHandler.WhenCodeIs(http.StatusOK).
		LogResultAs("ok").
		WithHandlerFn(func(log logrus.FieldLogger) interface{} {
			fileLogger := log.WithField("targetFile", artifactsFile)

			f, err := os.OpenFile(artifactsFile, os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				fileLogger.WithError(err).
					Error("Failed to create artifact file")

				return common.DownloadFailed
			}

			defer func() {
				err := f.Close()
				if err != nil {
					fileLogger.WithError(err).
						Error("Failed to close the artifact file")
				}
			}()

			err = httpResponse.FlushBodyTo(f)
			if err != nil {
				fileLogger.WithError(err).
					Error("Failed to save artifact file")

				removeErr := os.Remove(artifactsFile)
				if removeErr != nil {
					fileLogger.WithError(removeErr).
						Error("Failed to remove artifact file")
				}

				return common.DownloadFailed
			}

			return common.DownloadSucceeded
		})
	responseHandler.WhenCodeIs(http.StatusForbidden).
		LogResultAs("forbidden").
		WithHandlerFn(response.IdentityHandlerFn(common.DownloadForbidden))
	responseHandler.WhenCodeIs(http.StatusNotFound).
		LogResultAs("not found").
		WithHandlerFn(response.IdentityHandlerFn(common.DownloadNotFound))
	responseHandler.InDefaultCase().
		LogResultAs("failed").
		WithHandlerFn(response.IdentityHandlerFn(common.DownloadFailed))

	r, ok := responseHandler.Handle().(common.DownloadState)
	if !ok {
		return common.DownloadSucceeded
	}

	return r
}

func (n *GitLabClient) ProcessJob(config common.RunnerConfig, jobCredentials *common.JobCredentials) (common.JobTrace, error) {
	trace, err := newJobTrace(n, config, jobCredentials)
	if err != nil {
		return nil, err
	}

	trace.start()

	return trace, nil
}

func NewGitLabClientWithRequestStatusesMap(rsMap *APIRequestStatusesMap) *GitLabClient {
	return &GitLabClient{
		requestsStatusesMap: rsMap,
	}
}

func NewGitLabClient() *GitLabClient {
	return NewGitLabClientWithRequestStatusesMap(NewAPIRequestStatusesMap())
}
