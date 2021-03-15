package debug

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/mux"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func (s *Server) RegisterRunnersEndpoint() *RunnersEndpoint {
	router := s.router.PathPrefix("/runners").Subrouter()

	return NewRunnersEndpoint(router)
}

type RunnersEndpoint struct {
	router *mux.Router

	lock    sync.RWMutex
	runners map[string]*common.RunnerConfig
}

func NewRunnersEndpoint(router *mux.Router) *RunnersEndpoint {
	re := &RunnersEndpoint{
		router: router,
	}

	re.index(router)

	runnerRouter := router.PathPrefix("/{runner_id:[a-zA-Z0-9_]+}").Subrouter()
	re.config(runnerRouter.Path("/config"))
	re.executorProvider(runnerRouter.PathPrefix("/executor-provider"))

	return re
}

type runnerEntry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Executor string `json:"executor"`
	GitLab   string `json:"gitlab"`
	Limit    int    `json:"limit"`
}

func (re *RunnersEndpoint) index(router *mux.Router) {
	router.Path("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		re.lock.RLock()
		defer re.lock.RUnlock()

		runners := make([]runnerEntry, len(re.runners))

		index := 0
		for id, runner := range re.runners {
			runners[index] = runnerEntry{
				ID:       id,
				Name:     runner.Name,
				Executor: runner.Executor,
				GitLab:   runner.URL,
				Limit:    runner.Limit,
			}
			index++
		}

		SendJSON(w, http.StatusOK, runners)
	})
}

func (re *RunnersEndpoint) config(route *mux.Route) {
	route.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		runner := re.getRunner(w, r)
		if runner == nil {
			return
		}

		SendJSON(w, http.StatusOK, runner)
	})
}

func (re *RunnersEndpoint) getRunner(w http.ResponseWriter, r *http.Request) *common.RunnerConfig {
	vars := mux.Vars(r)

	runnerID, ok := vars["runner_id"]
	if !ok {
		SendJSON(w, http.StatusBadRequest, ErrorMsg{Error: "missing runner_id"})
		return nil
	}

	runner := re.findRunner(runnerID)
	if runner == nil {
		SendJSON(w, http.StatusNotFound, ErrorMsg{Error: fmt.Sprintf("no runner with id %q", runnerID)})
		return nil
	}

	return runner
}

func (re *RunnersEndpoint) findRunner(id string) *common.RunnerConfig {
	re.lock.RLock()
	defer re.lock.RUnlock()

	runner, ok := re.runners[id]
	if !ok {
		return nil
	}

	return runner
}

type ExecutorProviderDebugServer interface {
	ServeDebugHTTP(router *mux.Router)
}

func (re *RunnersEndpoint) executorProvider(route *mux.Route) {
	route.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		runner := re.getRunner(w, r)
		if runner == nil {
			return
		}

		provider := common.GetExecutorProvider(runner.Executor)
		if provider == nil {
			SendJSON(
				w,
				http.StatusInternalServerError,
				ErrorMsg{Error: fmt.Sprintf("couldn't find executor %q", runner.Executor)},
			)
			return
		}

		providerServer, ok := provider.(ExecutorProviderDebugServer)
		if !ok {
			SendJSON(
				w,
				http.StatusNotImplemented,
				ErrorMsg{Error: fmt.Sprintf("debug server for executor %q is not implemented", runner.Executor)},
			)
			return
		}

		router := route.Subrouter()
		providerServer.ServeDebugHTTP(router)
		router.ServeHTTP(w, r)
	})
}

func (re *RunnersEndpoint) SetRunners(runners []*common.RunnerConfig) {
	newRunners := make(map[string]*common.RunnerConfig, len(runners))

	for _, runner := range runners {
		newRunners[runner.ShortDescription()] = runner
	}

	re.lock.Lock()
	defer re.lock.Unlock()

	re.runners = newRunners
}
