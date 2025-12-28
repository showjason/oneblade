package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpenSearchCollectorFromOptions(t *testing.T) {
	opts := &OpenSearchOptions{
		Addresses: []string{"http://localhost:9200"},
		Username:  "admin",
		Password:  "admin",
		Index:     "logs-*",
	}

	collector, err := NewOpenSearchCollectorFromOptions(opts)
	require.NoError(t, err)
	assert.NotNil(t, collector)
	assert.Equal(t, []string{"http://localhost:9200"}, collector.addresses)
	assert.Equal(t, "admin", collector.username)
	assert.Equal(t, "admin", collector.password)
	assert.Equal(t, "logs-*", collector.index)
	assert.NotNil(t, collector.client)
}

func TestNewOpenSearchCollectorFromOptions_InvalidAddress(t *testing.T) {
	opts := &OpenSearchOptions{
		Addresses: []string{"://invalid"},
		Index:     "logs-*",
	}

	_, err := NewOpenSearchCollectorFromOptions(opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create opensearch client")
}

func TestOpenSearchCollector_Name(t *testing.T) {
	collector := &OpenSearchCollector{}
	assert.Equal(t, CollectorOpenSearch, collector.Name())
}

func TestOpenSearchCollector_Description(t *testing.T) {
	collector := &OpenSearchCollector{}
	desc := collector.Description()
	assert.Contains(t, desc, "OpenSearch")
	assert.Contains(t, desc, "DSL")
}

func TestOpenSearchCollector_AsTool(t *testing.T) {
	collector := &OpenSearchCollector{}
	tool, err := collector.AsTool()
	require.NoError(t, err)
	assert.NotNil(t, tool)
}

func TestOpenSearchCollector_Close(t *testing.T) {
	collector := &OpenSearchCollector{}
	err := collector.Close()
	assert.NoError(t, err)
}

// Note: Handle and Health methods require actual OpenSearch API calls
// These would be better tested with integration tests or a mock HTTP server
// For unit tests, we focus on the testable parts without external dependencies
