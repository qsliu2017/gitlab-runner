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

func generateReaderMatcher(metricsJson string) interface{} {
	return mock.MatchedBy(func(reader *io.SectionReader) bool {
		buf := new(bytes.Buffer)
		buf.ReadFrom(reader)
		s := buf.String()
		return s == metricsJson
	})
}

func TestQueryAndUploadMetrics(t *testing.T) {
	mockPrometheusApi := new(MockPrometheusApi)
	mockNetwork := new(common.MockNetwork)
	ctx, _ := context.WithCancel(context.Background())

	config := common.QueryMetricsConfig{
		QueryInterval: "10s",
		MetricQueries: []string{"metric1{{selector}}", "metric2{{selector}}"},
	}

	m, err := NewPrometheusQueryer(config, "instance", mockNetwork)
	require.NoError(t, err)

	metrics, err := m.Query(ctx, mockPrometheusApi, "test", time.Now(), time.Now())
	require.NoError(t, err)

	// make sure same length of metrics returned
	assert.Len(t, metrics, len(config.MetricQueries))

	metricsBytes, err := json.Marshal(metrics)
	require.NoError(t, err)

	metricsJson := string(metricsBytes)
	readerMatcher := generateReaderMatcher(metricsJson)
	mockNetwork.On("UploadRawArtifacts", *metricsJobCredentials, readerMatcher, metricsArtifactOptions).
		Return(common.UploadSucceeded).Once()

}
