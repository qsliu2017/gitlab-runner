package network

import (
	"bytes"
	"context"
	"sync"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/buffer"
)

type clientJobMetrics struct {
	client         common.Network
	config         common.RunnerConfig
	jobCredentials *common.JobCredentials
	id             int
	cancelFunc     context.CancelFunc

	buffer *buffer.Buffer

	lock     sync.RWMutex
	finished chan bool

	interval            int
	maxMetricsPatchSize int

	collectors map[string]*common.Collector
}

func (c *clientJobMetrics) RegisterCollector(name string, collector common.Collector) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.collectors[name] = &collector
}

func (c *clientJobMetrics) UnregisterCollector(name string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.collectors, name)
}

func (c *clientJobMetrics) readCollectors() {
	c.lock.Lock()
	defer c.lock.Unlock()
	for _, collector := range c.collectors {
		reader := (*collector).Collect()
		buf := new(bytes.Buffer)
		buf.ReadFrom(reader)
		c.buffer.Write(buf.Bytes())
	}
}

func (c *clientJobMetrics) start() {
	go c.poll()
}

func (c *clientJobMetrics) poll() {
	for {
		select {
		case <-c.finished:
			return
		case <-time.After(time.Duration(c.interval) * time.Second):
			go c.readCollectors()
		}
	}
}

func (c *clientJobMetrics) uploadArtifact() {
	// upload artifact
	options := common.ArtifactsOptions{
		BaseName: "monitor.log",
		Format:   "raw",
		Type:     "monitor",
		ExpireIn: "10000000",
	}
	reader, _ := c.buffer.Reader(0, c.buffer.Size())
	c.client.UploadRawArtifacts(*c.jobCredentials, reader, options)
	// close buffer
	c.buffer.Close()
}

func (c *clientJobMetrics) Stop() {
	// end the polling loop
	c.finished = make(chan bool)
	c.finished <- true
	// upload artifacts
	go c.uploadArtifact()
}

func newJobMetrics(client common.Network, config common.RunnerConfig, jobCredentials *common.JobCredentials) (*clientJobMetrics, error) {
	buffer, err := buffer.New("metrics")
	if err != nil {
		return nil, err
	}

	return &clientJobMetrics{
		client:              client,
		config:              config,
		buffer:              buffer,
		jobCredentials:      jobCredentials,
		id:                  jobCredentials.ID,
		maxMetricsPatchSize: common.DefaultMetricsPatchLimit,
		interval:            config.RunnerSettings.Metrics.Interval,
		collectors:          make(map[string]*common.Collector),
	}, nil
}
