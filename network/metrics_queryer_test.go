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

func TestCollectAndUploadMetrics(t *testing.T) {
	mockPrometheusApi := new(MockPrometheusApi)
	mockNetwork := new(common.MockNetwork)
	ctx, _ := context.WithCancel(context.Background())

	collectMetrics := []string{"node_exporter:node"}
	collectionInterval := "10s"
	m, err := NewPrometheusMetricCollector(mockPrometheusApi, collectionInterval, collectMetrics, mockNetwork)
	require.NoError(t, err)

	metrics, err := m.Collect(ctx, "test", time.Now(), time.Now())
	require.NoError(t, err)

	// make sure same length of metrics returned
	assert.Len(t, metrics, len(NodeExporterMetrics))

	metricsBytes, err := json.Marshal(metrics)
	require.NoError(t, err)

	metricsJson := string(metricsBytes)
	readerMatcher := generateReaderMatcher(metricsJson)
	mockNetwork.On("UploadRawArtifacts", *metricsJobCredentials, readerMatcher, metricsArtifactOptions).
		Return(common.UploadSucceeded).Once()

}
