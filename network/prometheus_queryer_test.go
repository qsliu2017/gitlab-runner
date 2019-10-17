package network

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var (
	metricsJobCredentials = &common.JobCredentials{ID: -1}
)

func generateReaderMatcher(metricsJSON string) interface{} {
	return mock.MatchedBy(func(reader *io.SectionReader) bool {
		buf := new(bytes.Buffer)
		buf.ReadFrom(reader)
		s := buf.String()
		return s == metricsJSON
	})
}

func TestQueryAndUploadMetricsParseError(t *testing.T) {
	mockPrometheusAPI := new(MockPrometheusAPI)
	mockNetwork := new(common.MockNetwork)
	ctx, cancel := context.WithCancel(context.Background())

	config := common.MetricsQueryerConfig{
		QueryInterval: "10s",
		MetricQueries: []string{"name1=metric1{{selector}}", "name2=metric2{{selector}}"},
	}

	m, err := NewPrometheusQueryer(config, "instance", mockNetwork)
	require.NoError(t, err)

	_, err = m.Query(ctx, mockPrometheusAPI, "test", time.Now(), time.Now())
	require.Error(t, err)
	cancel()
}

func TestQueryAndUploadMetricsWorks(t *testing.T) {
	mockPrometheusAPI := new(MockPrometheusAPI)
	mockNetwork := new(common.MockNetwork)
	ctx, cancel := context.WithCancel(context.Background())

	config := common.MetricsQueryerConfig{
		QueryInterval: "10s",
		MetricQueries: []string{"name1:metric1{{selector}}", "name2:metric2{{selector}}"},
	}

	m, err := NewPrometheusQueryer(config, "instance", mockNetwork)
	require.NoError(t, err)

	metrics, err := m.Query(ctx, mockPrometheusAPI, "test", time.Now(), time.Now())
	require.NoError(t, err)

	// make sure same length of metrics returned
	assert.Len(t, metrics, len(config.MetricQueries))

	metricsBytes, err := json.Marshal(metrics)
	require.NoError(t, err)

	metricsJSON := string(metricsBytes)
	readerMatcher := generateReaderMatcher(metricsJSON)
	mockNetwork.On("UploadRawArtifacts", *metricsJobCredentials, readerMatcher, metricsArtifactOptions).
		Return(common.UploadSucceeded).Once()

	cancel()

}
