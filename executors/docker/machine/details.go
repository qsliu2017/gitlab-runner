package machine

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

type machineDetails struct {
	Name       string       `json:"name"`
	Created    time.Time    `json:"created" yaml:"-"`
	Used       time.Time    `json:"used" yaml:"-"`
	UsedCount  int          `json:"used_count"`
	State      machineState `json:"state"`
	Reason     string       `json:"reason"`
	RetryCount int          `json:"retry_count"`
	LastSeen   time.Time    `json:"last_seen"`
	UsedBy     string       `json:"used_by"`
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
		time.Since(m.LastSeen) > machineDeadInterval
}

func (m *machineDetails) canBeUsed() bool {
	return m.State == machineStateAcquired
}

func (m *machineDetails) match(machineFilter string) bool {
	var query string
	if n, _ := fmt.Sscanf(m.Name, machineFilter, &query); n != 1 {
		return false
	}
	return true
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
	details.CreatedAgo = time.Since(m.Created)
	data := helpers.ToYAML(&details)
	_ = ioutil.WriteFile("machines/"+details.Details.Name+".yml", []byte(data), 0600)
}

func (m *machineDetails) logger() *logrus.Entry {
	return logrus.WithFields(logrus.Fields{
		"name":      m.Name,
		"lifetime":  time.Since(m.Created),
		"used":      time.Since(m.Used),
		"usedCount": m.UsedCount,
		"reason":    m.Reason,
	})
}

type machinesDetails map[string]*machineDetails
