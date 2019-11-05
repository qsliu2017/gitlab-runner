package referees

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsRefereeParseError(t *testing.T) {
	mockPrometheusAPI := new(MockPrometheusAPI)
	ctx, cancel := context.WithCancel(context.Background())

	queryInterval := "10s"
	metricQueries := []string{"name1=metric1{{selector}}", "name2=metric2{{selector}}"}
	log := logrus.WithField("builds", 1)

	mr, err := NewMetricsReferee(mockPrometheusAPI, queryInterval, metricQueries, "instance", log)
	require.NoError(t, err)

	_, err = mr.Execute(ctx, "test", time.Now(), time.Now())
	require.Error(t, err)
	cancel()
}

func TestMetricsRefereeExecute(t *testing.T) {
	mockPrometheusAPI := new(MockPrometheusAPI)
	ctx, cancel := context.WithCancel(context.Background())

	queryInterval := "10s"
	metricQueries := []string{"name1:metric1{{selector}}", "name2:metric2{{selector}}"}
	log := logrus.WithField("builds", 1)

	m, err := NewMetricsReferee(mockPrometheusAPI, queryInterval, metricQueries, "instance", log)
	require.NoError(t, err)

	reader, err := m.Execute(ctx, "test", time.Now(), time.Now())
	require.NoError(t, err)

	// convert reader result to golang maps
	buf := new(bytes.Buffer)
	buf.ReadFrom(reader)
	var metrics interface{}
	err = json.Unmarshal(buf.Bytes(), &metrics)
	require.NoError(t, err)

	// confirm length of elements
	assert.Len(t, metrics, len(metricQueries))
	cancel()
}
