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
	runnerID        int
	opened          time.Time
	currentInterval *interval
	intervals       []*interval
	closed          time.Time
}

var runnerTimings = map[int]*timing{}
var mux = sync.Mutex{}

func Open(runnerID int) error {
	mux.Lock()
	defer mux.Unlock()
	if _, ok := runnerTimings[runnerID]; ok {
		return fmt.Errorf("timing for runner %v is already open", runnerID)
	}
	runnerTimings[runnerID] = &timing{
		runnerID: runnerID,
		opened:   time.Now(),
	}
	return nil
}

func Close(runnerID int) (string, error) {
	mux.Lock()
	defer mux.Unlock()
	t, ok := runnerTimings[runnerID]
	if !ok {
		return "", fmt.Errorf("timing for runner %v is not open", runnerID)
	}
	if t.currentInterval != nil {
		return "", fmt.Errorf("timing %q is still open for runner %v", t.currentInterval.name, runnerID)
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
	totalString := fmt.Sprintf("[RUNNER %v TOTAL %vms UNACCOUNTED %vms] ", t.runnerID, ms, ms-msAccounted)
	report := totalString + strings.Join(intervalStrings, " ")

	delete(runnerTimings, runnerID)
	return report, nil
}

// Begin does not support nested intervals.
func Begin(runnerID int, name string) error {
	mux.Lock()
	defer mux.Unlock()
	t, ok := runnerTimings[runnerID]
	if !ok {
		return fmt.Errorf("timing for runner %v is not open", runnerID)
	}
	if t.currentInterval != nil {
		return fmt.Errorf("interval %q already begun for runner %v", t.currentInterval.name, runnerID)
	}
	t.currentInterval = &interval{
		name:  name,
		begin: time.Now(),
	}
	return nil
}

func End(runnerID int, name string) error {
	mux.Lock()
	defer mux.Unlock()
	t, ok := runnerTimings[runnerID]
	if !ok {
		return fmt.Errorf("timing for runner %v is not open", runnerID)
	}
	if t.currentInterval == nil {
		return fmt.Errorf("interval %q has not begun for runner %v", t.currentInterval.name, runnerID)
	}
	t.currentInterval.end = time.Now()
	t.intervals = append(t.intervals, t.currentInterval)
	t.currentInterval = nil
	return nil
}
