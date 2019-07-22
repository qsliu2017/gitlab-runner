package common

type Metrics struct{}

func (m *Metrics) RegisterCollector(collector Collector) {
}

func (m *Metrics) UnregisterCollector(collector Collector) {
}

func (m *Metrics) Start() {
}

func (m *Metrics) Finish() {
}

func (m *Metrics) uploadArtifact() {
}

func (m *Metrics) IsStarted() bool {
	return false
}
