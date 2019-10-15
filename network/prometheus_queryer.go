package network

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var metricsArtifactOptions = common.ArtifactsOptions{
	BaseName: "monitor.log",
	Format:   "raw",
	Type:     "monitor",
	ExpireIn: "10000000",
}

type PrometheusQueryer struct {
	metricQueries []string
	queryInterval time.Duration
	network       common.Network
	labelName     string
	log           func() *logrus.Entry
}

func (mq *PrometheusQueryer) Query(
	ctx context.Context,
	prometheusAPI prometheusV1.API,
	labelValue string,
	startTime time.Time,
	endTime time.Time,
) (map[string][]model.SamplePair, error) {
	// specify the range used for the PromQL query
	queryRange := prometheusV1.Range{
		Start: startTime,
		End:   endTime,
		Step:  mq.queryInterval,
	}

	metrics := make(map[string][]model.SamplePair)
	// use config file to pull metrics from prometheus range queries
	for _, metricQuery := range mq.metricQueries {
		// break up query into name:query
		components := strings.Split(metricQuery, ":")
		if len(components) != 2 {
			return nil, fmt.Errorf("prometheus_queryer: %s not in name:query format", metricQuery)
		}

		name := components[0]
		query := components[1]
		selector := fmt.Sprintf("%s=\"%s\"", mq.labelName, labelValue)
		interval := fmt.Sprintf("%fs", mq.queryInterval.Seconds())
		query = strings.Replace(query, "{selector}", selector, -1)
		query = strings.Replace(query, "{interval}", interval, -1)

		// execute query over range
		result, _, err := prometheusAPI.QueryRange(ctx, query, queryRange)
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

	return metrics, nil
}

func (mq *PrometheusQueryer) Upload(
	metrics map[string][]model.SamplePair,
	jobCredentials *common.JobCredentials,
) error {
	// convert metrics sample pairs to JSON
	output, err := json.Marshal(metrics)
	if err != nil {
		fmt.Errorf("Failed to marshall metrics into json for artifact upload")
		return err
	}

	// upload JSON to GitLab as monitor.log artifact
	reader := bytes.NewReader(output)
	mq.network.UploadRawArtifacts(*jobCredentials, reader, metricsArtifactOptions)
	return nil
}

func NewPrometheusQueryer(
	metricQueryerConfig common.MetricsQueryerConfig,
	labelName string,
	network common.Network,
) (*PrometheusQueryer, error) {
	queryIntervalDuration, err := time.ParseDuration(metricQueryerConfig.QueryInterval)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse query interval from config")
	}

	return &PrometheusQueryer{
		metricQueries: metricQueryerConfig.MetricQueries,
		queryInterval: queryIntervalDuration,
		labelName:     labelName,
		network:       network,
	}, nil
}
