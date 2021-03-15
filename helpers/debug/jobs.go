package debug

import (
	"net/http"

	"github.com/gorilla/mux"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func (s *Server) RegisterJobsEndpoint() *JobsEndpoint {
	router := s.router.PathPrefix("/jobs").Subrouter()

	return NewJobsEndpoint(router)
}

type ListJobsHandler interface {
	LegacyListJobsHandler(w http.ResponseWriter, r *http.Request)
	ListJobs() []*common.Build
}

type JobsEndpoint struct {
	router *mux.Router
}

func NewJobsEndpoint(router *mux.Router) *JobsEndpoint {
	return &JobsEndpoint{
		router: router,
	}
}

type jobEntry struct {
	URL           string                   `json:"url"`
	State         common.BuildRuntimeState `json:"state"`
	Stage         common.BuildStage        `json:"stage"`
	ExecutorStage common.ExecutorStage     `json:"executor_stage"`
	Duration      string                   `json:"duration"`
}

func (je *JobsEndpoint) RegisterJobsListEndpoint(h ListJobsHandler) {
	je.router.Path("/list").HandlerFunc(h.LegacyListJobsHandler)
	je.router.Path("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobs := h.ListJobs()
		jobEntries := make([]jobEntry, len(jobs))

		for idx, job := range jobs {
			jobEntries[idx] = jobEntry{
				URL:           job.JobURL(),
				State:         job.CurrentState(),
				Stage:         job.CurrentStage(),
				ExecutorStage: job.CurrentExecutorStage(),
				Duration:      job.Duration().String(),
			}
		}

		SendJSON(w, http.StatusOK, jobEntries)
	})
}
