package machine

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/machine/utils"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

type machineDetailsMap map[string]*machineDetails

type machineDetails struct {
	Name string

	CreatedAt  time.Time `yaml:"-"`
	UsedAt     time.Time `yaml:"-"`
	LastSeenAt time.Time

	UsedCount     int
	State         state
	RemovalReason string
	RetryCount    int
}

func newMachineDetails(name string) *machineDetails {
	return &machineDetails{
		Name:       name,
		CreatedAt:  time.Now(),
		UsedAt:     time.Now(),
		LastSeenAt: time.Now(),
		UsedCount:  1, // any machine that we find we mark as already used
		State:      machineStateIdle,
	}
}

func (m *machineDetails) acquire() {
	m.State = machineStateAcquired
}

func (m *machineDetails) create() {
	m.State = machineStateCreating
	m.UsedCount = 0
	m.RetryCount = 0
	m.LastSeenAt = time.Now()
}

func (m *machineDetails) remove(reason ...interface{}) {
	m.State = machineStateRemoving
	m.RetryCount = 0
	m.UsedAt = time.Now()
	m.RemovalReason = fmt.Sprint(reason...)
}

func (m *machineDetails) use() {
	m.State = machineStateUsed
	m.UsedCount++
	m.UsedAt = time.Now()
}

func (m *machineDetails) isPersistedOnDisk() bool {
	// Machines in creating phase might or might not be persisted on disk
	// this is due to async nature of machine creation process
	// where to `docker-machine create` is the one that is creating relevant files
	// and it is being executed with undefined delay
	return m.State != machineStateCreating
}

func (m *machineDetails) isUsed() bool {
	return m.State != machineStateIdle
}

func (m *machineDetails) isStuckOnRemove() bool {
	return m.State == machineStateRemoving && m.RetryCount >= removeRetryTries
}

func (m *machineDetails) isDead() bool {
	return m.State == machineStateIdle &&
		time.Since(m.LastSeenAt) > machineDeadInterval
}

func (m *machineDetails) canBeUsed() bool {
	return m.State == machineStateAcquired
}

func (m *machineDetails) match(machineNameTemplate string) bool {
	return utils.MatchesMachineNameTemplate(m.Name, machineNameTemplate)
}

func (m *machineDetails) logger() logrus.FieldLogger {
	return m.withFields(logrus.StandardLogger())
}

func (m *machineDetails) withFields(log logrus.FieldLogger) logrus.FieldLogger {
	return log.WithFields(logrus.Fields{
		"name":       m.Name,
		"lifetime":   m.Age(),
		"used":       m.UnusedFor(),
		"usedCount":  m.UsedCount,
		"reason":     m.RemovalReason,
		"retryCount": m.RetryCount,
	})
}

func (m *machineDetails) Age() time.Duration {
	return time.Since(m.CreatedAt)
}

func (m *machineDetails) UnusedFor() time.Duration {
	return time.Since(m.UsedAt)
}

func (m *machineDetails) writeDebugInformation() {
	if logrus.GetLevel() < logrus.DebugLevel {
		return
	}

	var details struct {
		Details    machineDetails
		Time       string
		CreatedAgo time.Duration
	}

	details.Details = *m
	details.Time = time.Now().String()
	details.CreatedAgo = m.Age()

	data := helpers.ToYAML(&details)

	_ = ioutil.WriteFile("machines/"+details.Details.Name+".yml", []byte(data), 0600)
}
