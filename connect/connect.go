package connect

import (
	"sync"
	"time"

	"gitlab.com/gitlab-org/fleeting/taskscaler"
)

type Job struct {
	Acquisition taskscaler.Acquisition
	HoldUntil   time.Time
}

var jobsMux = sync.Mutex{}
var jobs = map[int64]*Job{}

func GetJob(jobID int64) *Job {
	jobsMux.Lock()
	defer jobsMux.Unlock()
	j, ok := jobs[jobID]
	if !ok {
		j = &Job{}
		jobs[jobID] = j
	}
	return j
}

func DeleteJob(jobID int64) {
	jobsMux.Lock()
	defer jobsMux.Unlock()
	_, ok := jobs[jobID]
	if ok {
		delete(jobs, jobID)
	}
}

func StillHolding(jobID int64) bool {
	jobsMux.Lock()
	defer jobsMux.Unlock()
	if j, ok := jobs[jobID]; ok {
		return !time.Now().After(j.HoldUntil)
	}
	return false
}
