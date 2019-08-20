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

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

var NodeExporterMetrics = []string{
	"arp_entries",
	"context_switches_total",
	"cpu_guest_seconds_total",
	"cpu_seconds_total",
	"disk_io_now",
	"disk_io_time_seconds_total",
	"disk_io_time_weighted_seconds_total",
	"disk_read_bytes_total",
	"disk_read_time_seconds_total",
	"disk_reads_completed_total",
	"disk_reads_merged_total",
	"disk_write_time_seconds_total",
	"disk_writes_completed_total",
	"disk_writes_merged_total",
	"disk_written_bytes_total",
	"entropy_available_bits",
	"filefd_allocated",
	"filefd_maximum",
	"filesystem_avail_bytes",
	"filesystem_files",
	"filesystem_files_free",
	"filesystem_size_bytes",
	"memory_MemAvailable_bytes",
	"memory_MemFree_bytes",
	"memory_MemTotal_bytes",
	"memory_SwapCached_bytes",
	"memory_SwapFree_bytes",
	"memory_SwapTotal_bytes",
	"netstat_Tcp_ActiveOpens",
	"netstat_Tcp_InErrs",
	"netstat_Tcp_InSegs",
	"netstat_Tcp_OutSegs",
	"netstat_Tcp_PassiveOpens",
	"netstat_Udp_InDatagrams",
	"netstat_Udp_InErrors",
	"netstat_Udp_NoPorts",
	"netstat_Udp_OutDatagrams",
	"network_receive_bytes_total",
	"network_receive_drop_total",
	"network_receive_errs_total",
	"network_receive_packets_total",
	"network_transmit_bytes_total",
	"network_transmit_drop_total",
	"network_transmit_errs_total",
	"network_transmit_packets_total",
	"network_transmit_queue_length",
}

var MetricTypeMetrics = map[string][]string{
	"node_exporter": NodeExporterMetrics,
}

var MetricTypeLabels = map[string]string{
	"node_exporter": "instance",
}

var metricsArtifactOptions = common.ArtifactsOptions{
	BaseName: "monitor.log",
	Format:   "raw",
	Type:     "monitor",
	ExpireIn: "10000000",
}

type MetricsCollector struct {
	prometheusApi      v1.API
	prometheusAddress  string
	collectionInterval time.Duration
	collectMetrics     map[string]string
	network            common.Network
}

func (c *MetricsCollector) CollectAndUpload(
	ctx context.Context,
	labelValue string,
	jobData common.JobResponse,
	startTime time.Time,
	endTime time.Time,
) error {
	rng := v1.Range{
		Start: startTime,
		End:   endTime,
		Step:  c.collectionInterval,
	}

	metrics := make(map[string][]model.SamplePair)
	// use config file to pull metrics from prometheus range queries
	for metricType, metricJob := range c.collectMetrics {
		labelName := MetricTypeLabels[metricType]
		for _, metricName := range MetricTypeMetrics[metricType] {
			metricFullName := fmt.Sprintf("%s_%s", metricJob, metricName)
			query := fmt.Sprintf("%s{%s=\"%s\"}", metricFullName, labelName, labelValue)
			result, err := c.prometheusApi.QueryRange(ctx, query, rng)
			if err != nil {
				fmt.Errorf("Unable to collect metrics for range", err)
				return err
			}

			if result == nil {
				continue
			}

			if result.(model.Matrix).Len() == 0 {
				continue
			}

			// save first result set values at metric
			metrics[metricFullName] = (result.(model.Matrix)[0]).Values
		}
	}

	return nil

	output, err := json.Marshal(metrics)
	if err != nil {
		fmt.Errorf("Failed to marshall metrics into json for upload", err)
		return err
	}

	reader := bytes.NewReader(output)

	jobCredentials := &common.JobCredentials{
		ID:    jobData.ID,
		Token: jobData.Token,
	}

	// TODO FILL IN URL ^^

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

func NewMetricsCollector(
	serverAddress string,
	collectionInterval string,
	collectMetrics []string,
	network common.Network,
) (*MetricsCollector, error) {
	clientConfig := api.Config{
		Address: serverAddress,
	}

	prometheusClient, err := api.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("Unable to create prometheus collector", err)
	}

	prometheusApi := v1.NewAPI(prometheusClient)

	collectionIntervalDuration, err := time.ParseDuration(collectionInterval)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse step duration from config", err)
	}

	return &MetricsCollector{
		prometheusApi:      prometheusApi,
		collectionInterval: collectionIntervalDuration,
		collectMetrics:     mapCollectMetrics(collectMetrics),
		network:            network,
	}, nil
}
