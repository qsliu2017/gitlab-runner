package referees

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPrometheusAPI(t *testing.T) {
	prometheusAPI, err := NewPrometheusAPI("http://localhost:9000")
	assert.NotNil(t, prometheusAPI)
	assert.NoError(t, err)
}

func TestNewMetricsReferee(t *testing.T) {
	log := logrus.WithField("builds", 1)
	cfg := MetricsRefereeConfig{
		PrometheusAddress: "http://127.0.0.1:9000",
		QueryInterval:     "10s",
		MetricQueries:     []string{"name1:metric1{{selector}}", "name2:metric2{{selector}}"},
	}

	mr, err := NewMetricsReferee(log, "instance", cfg)
	require.NotNil(t, mr)
	require.NoError(t, err)
}

func TestNewMetricsRefereeInvalidQueryInterval(t *testing.T) {
	log := logrus.WithField("builds", 1)
	cfg := MetricsRefereeConfig{
		PrometheusAddress: "http://127.0.0.1:9000",
		QueryInterval:     "10",
		MetricQueries:     []string{"name1:metric1{{selector}}", "name2:metric2{{selector}}"},
	}

	mr, err := NewMetricsReferee(log, "instance", cfg)
	assert.Nil(t, mr)
	assert.Error(t, err)
}

func TestMetricsRefereeInvalidQueryFormat(t *testing.T) {
	log := logrus.WithField("builds", 1)
	cfg := MetricsRefereeConfig{
		PrometheusAddress: "http://127.0.0.1:9000",
		QueryInterval:     "10s",
		MetricQueries:     []string{"name1=metric1{{selector}}", "name2=metric2{{selector}}"},
	}

	mr, err := NewMetricsReferee(log, "instance", cfg)
	require.NoError(t, err)

	_, err = mr.Execute(context.Background(), time.Now(), time.Now())
	assert.Error(t, err)
}

// FIXME
func TestMetricsRefereeExecute(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Assert that the query was parsed correctly
	}))

	log := logrus.WithField("builds", 1)
	cfg := MetricsRefereeConfig{
		PrometheusAddress: ts.URL,
		QueryInterval:     "10s",
		MetricQueries:     []string{"name1:metric1{{selector}}", "name2:metric2{{selector}}"},
	}

	mr, err := NewMetricsReferee(log, "instance", cfg)
	require.NoError(t, err)

	reader, err := mr.Execute(context.Background(), time.Now(), time.Now())
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	buf.ReadFrom(reader)

	var metrics interface{}
	err = json.Unmarshal(buf.Bytes(), &metrics)
	require.NoError(t, err)

	// confirm length of elements
	assert.Len(t, metrics, len(cfg.MetricQueries))
}
