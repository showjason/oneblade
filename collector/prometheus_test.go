package collector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oneblade/utils"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPrometheusAPI is a mock implementation of v1.API
type mockPrometheusAPI struct {
	queryRangeFunc func(ctx context.Context, query string, r v1.Range) (model.Value, v1.Warnings, error)
	configFunc     func(ctx context.Context) (v1.ConfigResult, error)
}

func (m *mockPrometheusAPI) QueryRange(ctx context.Context, query string, r v1.Range, opts ...v1.Option) (model.Value, v1.Warnings, error) {
	if m.queryRangeFunc != nil {
		return m.queryRangeFunc(ctx, query, r)
	}
	return nil, nil, nil
}

func (m *mockPrometheusAPI) Config(ctx context.Context) (v1.ConfigResult, error) {
	if m.configFunc != nil {
		return m.configFunc(ctx)
	}
	return v1.ConfigResult{}, nil
}

// Implement other required methods with no-ops
func (m *mockPrometheusAPI) Alerts(ctx context.Context) (v1.AlertsResult, error) {
	return v1.AlertsResult{}, nil
}

func (m *mockPrometheusAPI) AlertManagers(ctx context.Context) (v1.AlertManagersResult, error) {
	return v1.AlertManagersResult{}, nil
}

func (m *mockPrometheusAPI) CleanTombstones(ctx context.Context) error {
	return nil
}

func (m *mockPrometheusAPI) DeleteSeries(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) error {
	return nil
}

func (m *mockPrometheusAPI) Flags(ctx context.Context) (v1.FlagsResult, error) {
	return v1.FlagsResult{}, nil
}

func (m *mockPrometheusAPI) LabelNames(ctx context.Context, matches []string, startTime time.Time, endTime time.Time, opts ...v1.Option) ([]string, v1.Warnings, error) {
	return nil, nil, nil
}

func (m *mockPrometheusAPI) LabelValues(ctx context.Context, label string, matches []string, startTime time.Time, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error) {
	return model.LabelValues{}, nil, nil
}

func (m *mockPrometheusAPI) Query(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {
	return nil, nil, nil
}

func (m *mockPrometheusAPI) QueryExemplars(ctx context.Context, query string, startTime time.Time, endTime time.Time) ([]v1.ExemplarQueryResult, error) {
	return nil, nil
}

func (m *mockPrometheusAPI) Buildinfo(ctx context.Context) (v1.BuildinfoResult, error) {
	return v1.BuildinfoResult{}, nil
}

func (m *mockPrometheusAPI) Runtimeinfo(ctx context.Context) (v1.RuntimeinfoResult, error) {
	return v1.RuntimeinfoResult{}, nil
}

func (m *mockPrometheusAPI) Series(ctx context.Context, matches []string, startTime time.Time, endTime time.Time, opts ...v1.Option) ([]model.LabelSet, v1.Warnings, error) {
	return nil, nil, nil
}

func (m *mockPrometheusAPI) Snapshot(ctx context.Context, skipHead bool) (v1.SnapshotResult, error) {
	return v1.SnapshotResult{}, nil
}

func (m *mockPrometheusAPI) Rules(ctx context.Context) (v1.RulesResult, error) {
	return v1.RulesResult{}, nil
}

func (m *mockPrometheusAPI) Targets(ctx context.Context) (v1.TargetsResult, error) {
	return v1.TargetsResult{}, nil
}

func (m *mockPrometheusAPI) TargetsMetadata(ctx context.Context, matchTarget string, metric string, limit string) ([]v1.MetricMetadata, error) {
	return nil, nil
}

func (m *mockPrometheusAPI) Metadata(ctx context.Context, metric string, limit string) (map[string][]v1.Metadata, error) {
	return nil, nil
}

func (m *mockPrometheusAPI) TSDB(ctx context.Context, opts ...v1.Option) (v1.TSDBResult, error) {
	return v1.TSDBResult{}, nil
}

func (m *mockPrometheusAPI) WalReplay(ctx context.Context) (v1.WalReplayStatus, error) {
	return v1.WalReplayStatus{}, nil
}

func TestNewPrometheusCollectorFromOptions(t *testing.T) {
	opts := &PrometheusOptions{
		Address: "http://localhost:9090",
		Timeout: utils.Duration{Duration: 30 * time.Second},
	}

	collector, err := NewPrometheusCollectorFromOptions(opts)
	require.NoError(t, err)
	assert.NotNil(t, collector)
	assert.Equal(t, "http://localhost:9090", collector.address)
	assert.Equal(t, 30*time.Second, collector.timeout)
}

func TestNewPrometheusCollectorFromOptions_InvalidAddress(t *testing.T) {
	opts := &PrometheusOptions{
		Address: "://invalid",
		Timeout: utils.Duration{Duration: 30 * time.Second},
	}

	_, err := NewPrometheusCollectorFromOptions(opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create prometheus client")
}

func TestPrometheusCollector_Name(t *testing.T) {
	collector := &PrometheusCollector{}
	assert.Equal(t, CollectorPrometheus, collector.Name())
}

func TestPrometheusCollector_Description(t *testing.T) {
	collector := &PrometheusCollector{}
	desc := collector.Description()
	assert.Contains(t, desc, "Prometheus")
	assert.Contains(t, desc, "PromQL")
}

func TestPrometheusCollector_Handle_ValidQuery(t *testing.T) {
		mockAPI := &mockPrometheusAPI{
		queryRangeFunc: func(ctx context.Context, query string, r v1.Range) (model.Value, v1.Warnings, error) {
			s := model.String{Value: "test result"}
			return &s, []string{"warning1"}, nil
		},
	}

	collector := &PrometheusCollector{
		address: "http://localhost:9090",
		timeout: 30 * time.Second,
		api:     mockAPI,
	}

	input := PrometheusQueryInput{
		PromQL:    "up",
		StartTime: "2024-01-01T00:00:00Z",
		EndTime:   "2024-01-01T01:00:00Z",
		Step:      "1m",
	}

	output, err := collector.Handle(context.Background(), input)
	require.NoError(t, err)
	// output.Data is a model.Value (model.String in this case)
	assert.NotNil(t, output.Data)
	assert.Equal(t, []string{"warning1"}, output.Warnings)
}

func TestPrometheusCollector_Handle_InvalidStartTime(t *testing.T) {
	collector := &PrometheusCollector{}

	input := PrometheusQueryInput{
		PromQL:    "up",
		StartTime: "invalid-time",
		EndTime:   "2024-01-01T01:00:00Z",
	}

	_, err := collector.Handle(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse start_time")
}

func TestPrometheusCollector_Handle_InvalidEndTime(t *testing.T) {
	collector := &PrometheusCollector{}

	input := PrometheusQueryInput{
		PromQL:    "up",
		StartTime: "2024-01-01T00:00:00Z",
		EndTime:   "invalid-time",
	}

	_, err := collector.Handle(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse end_time")
}

func TestPrometheusCollector_Handle_InvalidStep(t *testing.T) {
	collector := &PrometheusCollector{}

	input := PrometheusQueryInput{
		PromQL:    "up",
		StartTime: "2024-01-01T00:00:00Z",
		EndTime:   "2024-01-01T01:00:00Z",
		Step:      "invalid",
	}

	_, err := collector.Handle(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse step")
}

func TestPrometheusCollector_Handle_DefaultStep(t *testing.T) {
		mockAPI := &mockPrometheusAPI{
		queryRangeFunc: func(ctx context.Context, query string, r v1.Range) (model.Value, v1.Warnings, error) {
			// Verify step is 1 minute (default)
			assert.Equal(t, time.Minute, r.Step)
			s := model.String{Value: "result"}
			return &s, nil, nil
		},
	}

	collector := &PrometheusCollector{
		address: "http://localhost:9090",
		timeout: 30 * time.Second,
		api:     mockAPI,
	}

	input := PrometheusQueryInput{
		PromQL:    "up",
		StartTime: "2024-01-01T00:00:00Z",
		EndTime:   "2024-01-01T01:00:00Z",
		// Step not provided, should default to 1m
	}

	_, err := collector.Handle(context.Background(), input)
	require.NoError(t, err)
}

func TestPrometheusCollector_Handle_APIError(t *testing.T) {
	mockAPI := &mockPrometheusAPI{
		queryRangeFunc: func(ctx context.Context, query string, r v1.Range) (model.Value, v1.Warnings, error) {
			return nil, nil, errors.New("api error")
		},
	}

	collector := &PrometheusCollector{
		address: "http://localhost:9090",
		timeout: 30 * time.Second,
		api:     mockAPI,
	}

	input := PrometheusQueryInput{
		PromQL:    "up",
		StartTime: "2024-01-01T00:00:00Z",
		EndTime:   "2024-01-01T01:00:00Z",
	}

	_, err := collector.Handle(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prometheus query")
}

func TestPrometheusCollector_AsTool(t *testing.T) {
	collector := &PrometheusCollector{}
	tool, err := collector.AsTool()
	require.NoError(t, err)
	assert.NotNil(t, tool)
}

func TestPrometheusCollector_Health_Success(t *testing.T) {
	mockAPI := &mockPrometheusAPI{
		configFunc: func(ctx context.Context) (v1.ConfigResult, error) {
			return v1.ConfigResult{}, nil
		},
	}

	collector := &PrometheusCollector{
		api: mockAPI,
	}

	err := collector.Health(context.Background())
	assert.NoError(t, err)
}

func TestPrometheusCollector_Health_Error(t *testing.T) {
	mockAPI := &mockPrometheusAPI{
		configFunc: func(ctx context.Context) (v1.ConfigResult, error) {
			return v1.ConfigResult{}, errors.New("health check failed")
		},
	}

	collector := &PrometheusCollector{
		api: mockAPI,
	}

	err := collector.Health(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}

func TestPrometheusCollector_Close(t *testing.T) {
	collector := &PrometheusCollector{}
	err := collector.Close()
	assert.NoError(t, err)
}
