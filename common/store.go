package common

import (
	"encoding/json"
	"fmt"
	"github.com/gofrs/flock"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type JobExecutionState struct {
	sync.Mutex

	Job              *JobResponse      `json:"job"`
	Resumes          int               `json:"retries"`
	State            BuildRuntimeState `json:"state"`
	Stage            BuildStage        `json:"stage"`
	HealthCheckAt    TimeRFC3339       `json:"health_check_at"`
	StartedAt        TimeRFC3339       `json:"started_at"`
	SentTrace        int               `json:"sent_trace"`
	ExecutorMetadata any               `json:"executor_metadata"`

	ResumedFromStage BuildStage
}

func NewJobExecutionState(job *JobResponse) *JobExecutionState {
	return &JobExecutionState{
		Job:           job,
		Resumes:       0,
		HealthCheckAt: NewTimeRFC3339(time.Now()),
		StartedAt:     NewTimeRFC3339(time.Now()),
	}
}

func (s *JobExecutionState) UpdateHealth() {
	s.HealthCheckAt = NewTimeRFC3339(time.Now())
}

func (s *JobExecutionState) IsResumed() bool {
	return s.Resumes > 0
}

func (s *JobExecutionState) SetExecutorMetadata(data any) {
	s.Lock()
	defer s.Unlock()
	s.ExecutorMetadata = data
}

func (s *JobExecutionState) UpdateExecutorMetadata(mutator func(data any)) {
	s.Lock()
	defer s.Unlock()
	mutator(s.ExecutorMetadata)
}

type StoreFilter func(job *JobExecutionState) bool

var findRunningFilter = func(execution *JobExecutionState) bool {
	return time.Since(execution.HealthCheckAt.Time) > 30*time.Second &&
		(execution.State == BuildRunStatePending || execution.State == BuildRunRuntimeRunning)
}

type JobStore interface {
	Save(job *JobExecutionState) error
	Load(jobID int64) (*JobExecutionState, error)
	Remove(jobID int64) error
	FindJobToResume() (*JobExecutionState, error)
}

type StoreFactory func(*RunnerConfig) JobStore

type MultiJobStore struct {
	factory StoreFactory
	stores  map[string]JobStore
}

func NewMultiJobStore(factory StoreFactory) *MultiJobStore {
	return &MultiJobStore{stores: map[string]JobStore{}, factory: factory}
}

func (m *MultiJobStore) Get(runner *RunnerConfig) JobStore {
	if m.stores[runner.UniqueID()] == nil {
		m.stores[runner.UniqueID()] = m.factory(runner)
	}

	return m.stores[runner.UniqueID()]
}

type FileJobStore struct {
	dir    string
	runner *RunnerConfig
}

func NewFileJobStore(dir string, runner *RunnerConfig) *FileJobStore {
	return &FileJobStore{dir, runner}
}

func (f *FileJobStore) FindJobToResume() (*JobExecutionState, error) {
	files, err := ioutil.ReadDir(f.dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), fmt.Sprintf("runner.%s-%s.json", f.runner.Name, f.runner.ShortDescription())) {
			continue
		}

		state, err := func() (*JobExecutionState, error) {
			path := filepath.Join(f.dir, file.Name())
			lock := flock.New(path)
			if err := lock.Lock(); err != nil {
				return nil, err
			}
			defer lock.Unlock()

			state, err := f.load(path)
			if err != nil {
				return nil, err
			}

			if findRunningFilter(state) {
				state.UpdateHealth()
				if err := f.saveNoLocK(state); err != nil {
					return nil, err
				}

				return state, nil
			}

			return nil, nil
		}()

		if err != nil {
			return nil, err
		}

		if state != nil {
			return state, nil
		}
	}

	return nil, nil
}

func (f *FileJobStore) statePath(jobID int64) string {
	return filepath.Join(f.dir, fmt.Sprintf("state.job.%d.runner.%s-%s.json", jobID, f.runner.Name, f.runner.ShortDescription()))
}

func (f *FileJobStore) saveNoLocK(state *JobExecutionState) error {
	file := f.statePath(state.Job.ID)
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(file, b, 0700)
}

func (f *FileJobStore) Save(state *JobExecutionState) error {
	file := f.statePath(state.Job.ID)
	lock := flock.New(file)
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Unlock()

	return f.saveNoLocK(state)
}

func (f *FileJobStore) load(file string) (*JobExecutionState, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var state *JobExecutionState
	if err := json.Unmarshal(b, &state); err != nil {
		return nil, err
	}

	return state, nil
}

func (f *FileJobStore) Load(jobID int64) (*JobExecutionState, error) {
	return f.load(f.statePath(jobID))
}

func (f *FileJobStore) Remove(jobID int64) error {
	file := f.statePath(jobID)
	lock := flock.New(file)
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Unlock()

	return os.Remove(file)
}
