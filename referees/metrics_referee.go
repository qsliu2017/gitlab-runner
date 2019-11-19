package referees

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

type Metrics interface {
	MetricLabel() string
}

type MetricsRefereeConfig struct {
	PrometheusAddress string
	QueryInterval     string
	MetricQueries     []string
}

type MetricsReferee struct {
	prometheusAPI prometheusV1.API
	metricQueries []string
	queryInterval time.Duration
	labelName     string
	labelValue    string
	log           *logrus.Entry
}

func (mr *MetricsReferee) ArtifactBaseName() string {
	return "metrics_referee.json"
}

func (mr *MetricsReferee) ArtifactType() string {
	return "metrics_referee"
}

func (mr *MetricsReferee) ArtifactFormat() string {
	return "gzip"
}

func (mr *MetricsReferee) Execute(
	ctx context.Context,
	startTime time.Time,
	endTime time.Time,
) (*bytes.Reader, error) {
	// specify the range used for the PromQL query
	queryRange := prometheusV1.Range{
		Start: startTime.UTC(),
		End:   endTime.UTC(),
		Step:  mr.queryInterval,
	}

	metrics := make(map[string][]model.SamplePair)
	// use config file to pull metrics from prometheus range queries
	for _, metricQuery := range mr.metricQueries {
		// break up query into name:query
		components := strings.Split(metricQuery, ":")
		if len(components) != 2 {
			return nil, fmt.Errorf("prometheus_queryer: %s not in name:query format", metricQuery)
		}

		name := components[0]
		query := components[1]
		selector := fmt.Sprintf("%s=\"%s\"", mr.labelName, mr.labelValue)
		interval := fmt.Sprintf("%.0fs", mr.queryInterval.Seconds())
		query = strings.Replace(query, "{selector}", selector, -1)
		query = strings.Replace(query, "{interval}", interval, -1)

		// execute query over range
		result, _, err := mr.prometheusAPI.QueryRange(ctx, query, queryRange)
		if err != nil {
			return nil, err
		}

		// check for a result and pull first
		if result == nil || result.(model.Matrix).Len() == 0 {
			continue
		}

		// save first result set values at metric
		metrics[name] = (result.(model.Matrix)[0]).Values
	}

	// convert metrics sample pairs to JSON
	output, err := json.Marshal(metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to marshall metrics into json for artifact upload: %v", err)
	}

	return bytes.NewReader(output), nil
}

func NewPrometheusAPI(prometheusAddress string) (prometheusV1.API, error) {
	// create prometheus client from server address in config
	clientConfig := api.Config{Address: prometheusAddress}
	prometheusClient, err := api.NewClient(clientConfig)
	if err != nil {
		return nil, err
	}

	// create a prometheus api from the client config
	return prometheusV1.NewAPI(prometheusClient), nil
}

func NewMetricsReferee(log *logrus.Entry, labelName string, config MetricsRefereeConfig) (*MetricsReferee, error) {
	queryIntervalDuration, err := time.ParseDuration(config.QueryInterval)
	if err != nil {
		return nil, fmt.Errorf("unable to parse query interval from config: %v", err)
	}

	prometheusAPI, err := NewPrometheusAPI(config.PrometheusAddress)
	if err != nil {
		return nil, fmt.Errorf("setting prometheus API: %v", err)
	}

	return &MetricsReferee{
		prometheusAPI: prometheusAPI,
		queryInterval: queryIntervalDuration,
		metricQueries: config.MetricQueries,
		labelName:     labelName,
		log:           log,
	}, nil
}
