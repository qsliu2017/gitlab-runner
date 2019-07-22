package network

import (
	"bytes"
	"sync"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/buffer"
)

type clientJobMetrics struct {
	client         common.Network
	config         common.RunnerConfig
	jobCredentials *common.JobCredentials

	buffer *buffer.Buffer

	lock      sync.RWMutex
	finishing chan bool
	stopped   bool

	interval            time.Duration
	maxMetricsPatchSize int

	collectors map[common.Collector]common.Collector
}

var metricsArtifactOptions = common.ArtifactsOptions{
	BaseName: "monitor.log",
	Format:   "raw",
	Type:     "monitor",
	ExpireIn: "10000000",
}

func (c *clientJobMetrics) RegisterCollector(collector common.Collector) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.collectors[collector] = collector
}

func (c *clientJobMetrics) UnregisterCollector(collector common.Collector) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.collectors, collector)
}

func (c *clientJobMetrics) readCollectors() {
	c.lock.Lock()
	defer c.lock.Unlock()
	for _, collector := range c.collectors {
		reader := collector.Collect()
		buf := new(bytes.Buffer)
		buf.ReadFrom(reader)
		c.buffer.Write(buf.Bytes())
	}
}

func (c *clientJobMetrics) Start() {
	c.stopped = false
	c.finishing = make(chan bool)
	go c.poll()
}

func (c *clientJobMetrics) poll() {
	for {
		select {
		case <-c.finishing:
			return
		case <-time.After(c.interval):
			go c.readCollectors()
		}
	}
}

func (c *clientJobMetrics) Finish() {
	// mark stopped
	c.stopped = true
	// end the polling loop
	c.finishing <- true
	// upload artifact
	c.uploadArtifact()
}

func (c *clientJobMetrics) uploadArtifact() {
	size := c.buffer.Size()
	if size > 0 {
		// create a reader and upload the raw artifact to GitLab
		reader, _ := c.buffer.Reader(0, size)
		c.client.UploadRawArtifacts(*c.jobCredentials, reader, metricsArtifactOptions)
	}
	// close buffer
	c.buffer.Close()
}

func (c *clientJobMetrics) IsStarted() bool {
	return !c.stopped
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
		maxMetricsPatchSize: common.DefaultMetricsPatchLimit,
		interval:            time.Duration(config.RunnerSettings.Metrics.Interval) * time.Second,
		collectors:          make(map[common.Collector]common.Collector),
		stopped:             true,
	}, nil
}
