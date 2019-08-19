package network

import (
	"context"
	"fmt"
	"time"

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

var MetricTypeMetrics = map[string](*[]string){
	"node": &NodeExporterMetrics,
}

var MetricTypeLabels = map[string]string{
	"node": "instance",
}

type MetricsCollector struct {
	prometheusApi      v1.API
	prometheusAddress  string
	collectionInterval time.Duration
	metricTypes        []string
}

func (c *MetricsCollector) Collect(
	ctx context.Context,
	labelValue string,
	startTime time.Time,
	endTime time.Time,
) error {
	rng := v1.Range{
		Start: startTime,
		End:   endTime,
		Step:  c.collectionInterval,
	}

	for _, metricType := range c.metricTypes {
		for _, metric := range *MetricTypeMetrics[metricType] {
			query := fmt.Sprintf("%s{%s=%s}", metric, MetricTypeLabels[metricType], labelValue)
			value, err := c.prometheusApi.QueryRange(ctx, query, rng)
			if err != nil {
				fmt.Errorf("Unable to collect metrics for range", err)
			}
			fmt.Printf("%+v\n", value)
		}
	}

	return nil
}

func NewMetricsCollector(
	serverAddress string,
	collectionInterval string,
	metricTypes []string,
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
		metricTypes:        metricTypes,
	}, nil
}
