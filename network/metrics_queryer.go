package network

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"

	"github.com/prometheus/common/model"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

var metricsArtifactOptions = common.ArtifactsOptions{
	BaseName: "monitor.log",
	Format:   "raw",
	Type:     "monitor",
	ExpireIn: "10000000",
}

type MetricsQueryer struct {
	metricQueries string[]
	queryInterval time.Duration
	network            common.Network
}

func (c *MetricsQueryer) Collect(
	ctx context.Context,
	prometheusAddress string,
	labelName string,
	lavelValue string,
	startTime time.Time,
	endTime time.Time,
) (map[string][]model.SamplePair, error) {

	clientConfig := api.Config{Address: prometheusAddress}
	prometheusClient, err := api.NewClient(clientConfig)
	if err != nil {
		mr.log().Info("Unable to create prometheus collector")
		return
	}

	prometheusApi := v1.NewAPI(prometheusClient)

	rng := v1.Range{
		Start: startTime,
		End:   endTime,
		Step:  c.collectionInterval,
	}

	metrics := make(map[string][]model.SamplePair)
	// use config file to pull metrics from prometheus range queries
	for metricQuery := range c.metricQueries {
		query := fmt.Sprintf("%s{%s=\"%s\"}", metricQuery, labelName, labelValue)
		result, err := prometheusApi.QueryRange(ctx, query, rng)
		if err != nil {
			fmt.Errorf("Unable to collect metrics for range")
			return nil, err
		}

		// check for a result
		if result == nil {
			continue
		}

		// pull first result
		if result.(model.Matrix).Len() == 0 {
			continue
		}

		// save first result set values at metric
		metrics[metricFullName] = (result.(model.Matrix)[0]).Values
	}

	return metrics, nil
}

func (c *MetricsQueryer) Upload(
	metrics map[string][]model.SamplePair,
	jobCredentials *common.JobCredentials,
) error {
	// convert metrics to JSON
	output, err := json.Marshal(metrics)
	if err != nil {
		fmt.Errorf("Failed to marshall metrics into json for upload")
		return err
	}

	// upload JSON to GitLab as monitor.log artifact
	reader := bytes.NewReader(output)
	c.network.UploadRawArtifacts(*jobCredentials, reader, metricsArtifactOptions)
	return nil
}

func mapCollectMetrics(collectMetrics []string) map[string]string {
	collectMetricsMap := make(map[string]string)
	for _, collectMetric := range collectMetrics {
		collectMetricParts := strings.Split(collectMetric, ":")
		collectMetricsMap[collectMetricParts[0]] = collectMetricParts[1]
	}
	return collectMetricsMap
}

func NewMetricQueryer(
	queryMetrics common.QueryMetricsConfig
	network common.Network,
) (*MetricsQueryer, error) {
	queryIntervalDuration, err := time.ParseDuration(queryInterval)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse query interval from config")
	}

	return &MetricsQueryer{
		metricQueries:  queryMetrics.MetricQueries,
		queryInterval: queryIntervalDuration
		network:       network,
	}, nil
}
