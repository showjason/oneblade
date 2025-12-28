package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPagerDutyCollectorFromOptions(t *testing.T) {
	opts := &PagerDutyOptions{
		APIKey: "test-api-key",
	}

	collector := NewPagerDutyCollectorFromOptions(opts)
	require.NotNil(t, collector)
	assert.Equal(t, "test-api-key", collector.apiKey)
	assert.NotNil(t, collector.client)
}

func TestPagerDutyCollector_Name(t *testing.T) {
	collector := &PagerDutyCollector{}
	assert.Equal(t, CollectorPagerDuty, collector.Name())
}

func TestPagerDutyCollector_Description(t *testing.T) {
	collector := &PagerDutyCollector{}
	desc := collector.Description()
	assert.Contains(t, desc, "PagerDuty")
	assert.Contains(t, desc, "incidents")
}

func TestPagerDutyCollector_AsTool(t *testing.T) {
	collector := &PagerDutyCollector{}
	tool, err := collector.AsTool()
	require.NoError(t, err)
	assert.NotNil(t, tool)
}

func TestPagerDutyCollector_Close(t *testing.T) {
	collector := &PagerDutyCollector{}
	err := collector.Close()
	assert.NoError(t, err)
}

// Note: Handle and Health methods require actual PagerDuty API calls
// These would be better tested with integration tests or a mock HTTP server
// For unit tests, we focus on the testable parts without external dependencies
