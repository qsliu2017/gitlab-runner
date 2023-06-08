package timing

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type interval struct {
	name  string
	begin time.Time
	end   time.Time
}

type timing struct {
	jobID           int64
	opened          time.Time
	currentInterval *interval
	intervals       []*interval
	closed          time.Time
}

var runnerTimings = map[int64]*timing{}
var mux = sync.Mutex{}

func Open(jobID int64) error {
	mux.Lock()
	defer mux.Unlock()
	if _, ok := runnerTimings[jobID]; ok {
		return fmt.Errorf("timing for runner %v is already open", jobID)
	}
	runnerTimings[jobID] = &timing{
		jobID:  jobID,
		opened: time.Now(),
	}
	return nil
}

func Close(jobID int64) (string, error) {
	mux.Lock()
	defer mux.Unlock()
	t, ok := runnerTimings[jobID]
	if !ok {
		return "", fmt.Errorf("timing for runner %v is not open", jobID)
	}
	if t.currentInterval != nil {
		return "", fmt.Errorf("timing %q is still open for runner %v", t.currentInterval.name, jobID)
	}
	t.closed = time.Now()

	intervalStrings := make([]string, len(t.intervals))
	var msAccounted int64
	for i, in := range t.intervals {
		ms := in.end.Sub(in.begin).Milliseconds()
		msAccounted += ms
		intervalStrings[i] = fmt.Sprintf("[%q %vms]", in.name, ms)
	}
	ms := t.closed.Sub(t.opened).Milliseconds()
	totalString := fmt.Sprintf("[JOB %v TOTAL %vms UNACCOUNTED %vms] ", t.jobID, ms, ms-msAccounted)
	report := totalString + strings.Join(intervalStrings, " ")

	delete(runnerTimings, jobID)
	return report, nil
}

// Begin does not support nested intervals.
func Begin(jobID int64, name string) error {
	mux.Lock()
	defer mux.Unlock()
	t, ok := runnerTimings[jobID]
	if !ok {
		return fmt.Errorf("timing for runner %v is not open", jobID)
	}
	if t.currentInterval != nil {
		return fmt.Errorf("interval %q already begun for runner %v", t.currentInterval.name, jobID)
	}
	t.currentInterval = &interval{
		name:  name,
		begin: time.Now(),
	}
	return nil
}

func End(jobID int64, name string) error {
	mux.Lock()
	defer mux.Unlock()
	t, ok := runnerTimings[jobID]
	if !ok {
		return fmt.Errorf("timing for runner %v is not open", jobID)
	}
	if t.currentInterval == nil {
		return fmt.Errorf("interval %q has not begun for runner %v", t.currentInterval.name, jobID)
	}
	t.currentInterval.end = time.Now()
	t.intervals = append(t.intervals, t.currentInterval)
	t.currentInterval = nil
	return nil
}
