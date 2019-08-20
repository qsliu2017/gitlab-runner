package network

import (
	"context"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/mock"
)

type MockPrometheusApi struct {
	mock.Mock
}

func (mpi *MockPrometheusApi) AlertManagers(ctx context.Context) (v1.AlertManagersResult, error) {
	return v1.AlertManagersResult{}, nil
}

func (mpi *MockPrometheusApi) CleanTombstones(ctx context.Context) error {
	return nil
}

func (mpi *MockPrometheusApi) Config(ctx context.Context) (v1.ConfigResult, error) {
	return v1.ConfigResult{}, nil
}

func (mpi *MockPrometheusApi) DeleteSeries(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) error {
	return nil
}

func (mpi *MockPrometheusApi) Flags(ctx context.Context) (v1.FlagsResult, error) {
	return nil, nil
}

func (mpi *MockPrometheusApi) LabelValues(ctx context.Context, label string) (model.LabelValues, error) {
	return nil, nil
}

func (mpi *MockPrometheusApi) Query(ctx context.Context, query string, ts time.Time) (model.Value, error) {
	return nil, nil
}

func (mpi *MockPrometheusApi) QueryRange(ctx context.Context, query string, r v1.Range) (model.Value, error) {
	metric := model.Metric{"test": "test"}
	var ts model.Time = 32432
	var sv model.SampleValue = 435345
	samplePair := model.SamplePair{Timestamp: ts, Value: sv}
	sps := []model.SamplePair{samplePair}
	sampleStream := model.SampleStream{Metric: metric, Values: sps}
	var matrix model.Matrix = []*model.SampleStream{&sampleStream}
	return matrix, nil
}

func (mpi *MockPrometheusApi) Series(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) ([]model.LabelSet, error) {
	return nil, nil
}

func (mpi *MockPrometheusApi) Snapshot(ctx context.Context, skipHead bool) (v1.SnapshotResult, error) {
	return v1.SnapshotResult{}, nil
}

func (mpi *MockPrometheusApi) Targets(ctx context.Context) (v1.TargetsResult, error) {
	return v1.TargetsResult{}, nil
}
