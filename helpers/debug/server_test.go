package debug

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestRequests(t *testing.T) {
	m := http.NewServeMux()
	server := NewServer(m)

	upstream := httptest.NewServer(m)
	defer upstream.Close()

	url := upstream.URL

	ljh := new(MockListJobsHandler)
	defer ljh.AssertExpectations(t)

	jobs := server.RegisterJobsEndpoint()
	jobs.RegisterJobsListEndpoint(ljh)

	ljh.On("LegacyListJobsHandler", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		w, ok := args.Get(0).(http.ResponseWriter)
		if !ok {
			return
		}

		_, _ = fmt.Fprintf(w, "test-jobs-list")
	}).Once()

	ljh.On("ListJobs").Return([]*common.Build{
		{
			JobResponse: common.JobResponse{
				ID: 1,
				GitInfo: common.GitInfo{
					RepoURL: "https://gitlab.example.com/ns/project",
				},
			},
		},
	}).Once()

	runners := server.RegisterRunnersEndpoint()
	runners.SetRunners([]*common.RunnerConfig{
		{
			Name:  "test-1",
			Limit: 10,
			RunnerCredentials: common.RunnerCredentials{
				URL:   "https://gitlab-1.example.com/",
				Token: "TOKEN_1_is_very_long",
			},
			RunnerSettings: common.RunnerSettings{
				Executor: "shell",
			},
		},
		{
			Name:  "test-2",
			Limit: 20,
			RunnerCredentials: common.RunnerCredentials{
				URL:   "https://gitlab21.example.com/",
				Token: "TOKEN_2",
			},
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
			},
		},
	})

	r, err := http.Get(url + "/debug/jobs/")
	require.NoError(t, err)
	t.Log(r.StatusCode)
	b, err := ioutil.ReadAll(r.Body)
	require.NoError(t, err)
	t.Log(string(b))

	r, err = http.Get(url + "/debug/jobs/list")
	require.NoError(t, err)
	t.Log(r.StatusCode)
	b, err = ioutil.ReadAll(r.Body)
	require.NoError(t, err)
	t.Log(string(b))
	t.Log(r.Header)
}
