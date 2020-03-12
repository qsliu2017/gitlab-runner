package machine

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Describe implements prometheus.Collector.
func (m *machineProvider) Describe(ch chan<- *prometheus.Desc) {
	m.totalActions.Describe(ch)
	m.creationHistogram.Describe(ch)

	ch <- m.currentStatesDesc
}

// Collect implements prometheus.Collector.
func (m *machineProvider) Collect(ch chan<- prometheus.Metric) {
	m.totalActions.Collect(ch)
	m.creationHistogram.Collect(ch)

	machinesCounter := m.collectDetails()

	ch <- prometheus.MustNewConstMetric(
		m.currentStatesDesc,
		prometheus.GaugeValue,
		float64(machinesCounter.Acquired),
		"acquired",
	)
	ch <- prometheus.MustNewConstMetric(
		m.currentStatesDesc,
		prometheus.GaugeValue,
		float64(machinesCounter.Creating),
		"creating",
	)
	ch <- prometheus.MustNewConstMetric(
		m.currentStatesDesc,
		prometheus.GaugeValue,
		float64(machinesCounter.Idle),
		"idle",
	)
	ch <- prometheus.MustNewConstMetric(
		m.currentStatesDesc,
		prometheus.GaugeValue,
		float64(machinesCounter.Used),
		"used",
	)
	ch <- prometheus.MustNewConstMetric(
		m.currentStatesDesc,
		prometheus.GaugeValue,
		float64(machinesCounter.Removing),
		"removing",
	)
	ch <- prometheus.MustNewConstMetric(
		m.currentStatesDesc,
		prometheus.GaugeValue,
		float64(machinesCounter.StuckOnRemoving),
		"stuck-on-removing",
	)
}

func (m *machineProvider) collectDetails() *machinesCounter {
	m.lock.RLock()
	defer m.lock.RUnlock()

	machinesCounter := new(machinesCounter)
	for _, machine := range m.machines {
		if !machine.isDead() {
			machinesCounter.Count(machine)
		}
	}

	return machinesCounter
}
