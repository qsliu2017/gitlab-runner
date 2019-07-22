package network

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var (
	metricsJobConfig = common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Metrics: &common.MetricsConfig{
				Interval: 1,
			},
		},
	}
	metricsJobCredentials = &common.JobCredentials{ID: -1}
)

func TestRegisterUnregisterCollector(t *testing.T) {
	mockNetwork := new(common.MockNetwork)
	mockCollector := new(common.MockCollector)
	m, err := newJobMetrics(mockNetwork, metricsJobConfig, metricsJobCredentials)
	require.NoError(t, err)
	m.RegisterCollector(mockCollector)
	assert.Contains(t, m.collectors, mockCollector)
	m.UnregisterCollector(mockCollector)
	assert.NotContains(t, m.collectors, mockCollector)
}

func generateReaderMatcher(example string) interface{} {
	return mock.MatchedBy(func(reader *io.SectionReader) bool {
		buf := new(bytes.Buffer)
		buf.ReadFrom(reader)
		s := buf.String()
		return s == example
	})
}

func TestCollectMetrics(t *testing.T) {

	mockNetwork := new(common.MockNetwork)
	mockCollector := common.NewMockCollector("TEST")
	readerMatcher := generateReaderMatcher("TEST")

	// expect to receive just one metrics upload
	mockNetwork.On("UploadRawArtifacts", *metricsJobCredentials, readerMatcher, metricsArtifactOptions).
		Return(common.UploadSucceeded).Once()

	m, err := newJobMetrics(mockNetwork, metricsJobConfig, metricsJobCredentials)
	require.NoError(t, err)
	assert.False(t, m.IsStarted())
	m.RegisterCollector(mockCollector)
	m.Start()
	assert.True(t, m.IsStarted())
	time.Sleep(1500 * time.Millisecond)
	m.UnregisterCollector(mockCollector)
	m.Finish()
	assert.False(t, m.IsStarted())

}

func TestCollectNoMetrics(t *testing.T) {
	mockNetwork := new(common.MockNetwork)
	mockCollector := new(common.MockCollector)
	m, err := newJobMetrics(mockNetwork, metricsJobConfig, metricsJobCredentials)
	require.NoError(t, err)
	assert.False(t, m.IsStarted())
	m.RegisterCollector(mockCollector)
	m.Start()
	assert.True(t, m.IsStarted())
	m.UnregisterCollector(mockCollector)
	m.Finish()
	assert.False(t, m.IsStarted())
}
