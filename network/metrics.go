package network

import (
	"context"
	"sync"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/trace"
)

type clientJobMetrics struct {
	client         common.Network
	config         common.RunnerConfig
	jobCredentials *common.JobCredentials
	id             int
	cancelFunc     context.CancelFunc

	lock          sync.RWMutex
	state         common.JobState
	failureReason common.JobFailureReason
	finished      chan bool

	sentMonitor int
	sentTime    time.Time
	sentState   common.JobState

	updateInterval      time.Duration
	forceSendInterval   time.Duration
	finishRetryInterval time.Duration

	failuresCollector common.FailuresCollector
}

func (c *clientJobMetrics) Success() {
	c.Fail(nil, common.NoneFailure)
}

func (c *clientJobMetrics) Fail(err error, failureReason common.JobFailureReason) {
	c.lock.Lock()

	if c.state != common.Running {
		c.lock.Unlock()
		return
	}

	if err == nil {
		c.state = common.Success
	} else {
		c.setFailure(failureReason)
	}

	c.lock.Unlock()
	c.finish()
}

func (c *clientJobMetrics) SetCancelFunc(cancelFunc context.CancelFunc) {
	c.cancelFunc = cancelFunc
}

func (c *clientJobMetrics) SetFailuresCollector(fc common.FailuresCollector) {
	c.failuresCollector = fc
}

func (c *clientJobMetrics) setFailure(reason common.JobFailureReason) {
	c.state = common.Failed
	c.failureReason = reason
	if c.failuresCollector != nil {
		c.failuresCollector.RecordFailure(reason, c.config.ShortDescription())
	}
}

func (c *clientJobMetrics) start() {
	c.finished = make(chan bool)
	c.state = common.Running
	c.sentState = common.Running
	c.setupLogLimit()
	go c.watch()
}

func (c *clientJobMetrics) finalTraceUpdate() {
	for c.anyTraceToSend() {
		switch c.sendPatch() {
		case common.UpdateSucceeded:
			// we continue sending till we succeed
			continue
		case common.UpdateAbort:
			return
		case common.UpdateNotFound:
			return
		case common.UpdateRangeMismatch:
			time.Sleep(c.finishRetryInterval)
		case common.UpdateFailed:
			time.Sleep(c.finishRetryInterval)
		}
	}
}

func (c *clientJobMetrics) finalStatusUpdate() {
	for {
		switch c.sendUpdate(true) {
		case common.UpdateSucceeded:
			return
		case common.UpdateAbort:
			return
		case common.UpdateNotFound:
			return
		case common.UpdateRangeMismatch:
			return
		case common.UpdateFailed:
			time.Sleep(c.finishRetryInterval)
		}
	}
}

func (c *clientJobMetrics) finish() {
	c.finished <- true
	c.finalTraceUpdate()
	c.finalStatusUpdate()
}

func (c *clientJobMetrics) incrementalUpdate() common.UpdateState {
	state := c.sendPatch()
	if state != common.UpdateSucceeded {
		return state
	}

	return c.sendUpdate(false)
}

func (c *clientJobMetrics) sendPatch() common.UpdateState {
	c.lock.RLock()
	content, err := c.buffer.Bytes(c.sentTrace, c.maxTracePatchSize)
	sentTrace := c.sentTrace
	c.lock.RUnlock()

	if err != nil {
		return common.UpdateFailed
	}

	if len(content) == 0 {
		return common.UpdateSucceeded
	}

	sentOffset, state := c.client.PatchTrace(
		c.config, c.jobCredentials, content, sentTrace)

	if state == common.UpdateSucceeded || state == common.UpdateRangeMismatch {
		c.lock.Lock()
		c.sentTime = time.Now()
		c.sentTrace = sentOffset
		c.lock.Unlock()
	}

	return state
}

func (c *clientJobMetrics) sendUpdate(force bool) common.UpdateState {
	c.lock.RLock()
	state := c.state
	shouldUpdateState := c.state != c.sentState
	shouldRefresh := time.Since(c.sentTime) > c.forceSendInterval
	c.lock.RUnlock()

	if !force && !shouldUpdateState && !shouldRefresh {
		return common.UpdateSucceeded
	}

	jobInfo := common.UpdateJobInfo{
		ID:            c.id,
		State:         state,
		FailureReason: c.failureReason,
	}

	status := c.client.UpdateJob(c.config, c.jobCredentials, jobInfo)

	if status == common.UpdateSucceeded {
		c.lock.Lock()
		c.sentTime = time.Now()
		c.sentState = state
		c.lock.Unlock()
	}

	return status
}

func (c *clientJobMetrics) abort() bool {
	if c.cancelFunc != nil {
		c.cancelFunc()
		c.cancelFunc = nil
		return true
	}
	return false
}

func (c *clientJobMetrics) watch() {
	for {
		select {
		case <-time.After(c.updateInterval):
			state := c.incrementalUpdate()
			if state == common.UpdateAbort && c.abort() {
				<-c.finished
				return
			}
			break

		case <-c.finished:
			return
		}
	}
}

func newJobTrace(client common.Network, config common.RunnerConfig, jobCredentials *common.JobCredentials) (*clientJobTrace, error) {
	buffer, err := trace.New()
	if err != nil {
		return nil, err
	}

	return &clientJobTrace{
		client:              client,
		config:              config,
		buffer:              buffer,
		jobCredentials:      jobCredentials,
		id:                  jobCredentials.ID,
		maxTracePatchSize:   common.DefaultTracePatchLimit,
		updateInterval:      common.UpdateInterval,
		forceSendInterval:   common.ForceTraceSentInterval,
		finishRetryInterval: common.UpdateRetryInterval,
	}, nil
}
