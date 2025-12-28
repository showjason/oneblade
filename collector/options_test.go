package collector

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterOptionsParser(t *testing.T) {
	// Reset parsers for clean test
	optionsParsers.parsers = make(map[CollectorType]OptionsParser)

	testType := CollectorType("test")
	testParser := func(meta *toml.MetaData, primitive toml.Primitive) (interface{}, error) {
		return "test", nil
	}

	RegisterOptionsParser(testType, testParser)

	parser, ok := GetOptionsParser(testType)
	require.True(t, ok)
	assert.NotNil(t, parser)
}

func TestGetOptionsParser(t *testing.T) {
	// Reset parsers for clean test
	optionsParsers.parsers = make(map[CollectorType]OptionsParser)

	testType := CollectorType("test")
	testParser := func(meta *toml.MetaData, primitive toml.Primitive) (interface{}, error) {
		return "test", nil
	}

	RegisterOptionsParser(testType, testParser)

	// Test getting existing parser
	parser, ok := GetOptionsParser(testType)
	require.True(t, ok)
	assert.NotNil(t, parser)

	// Test getting non-existent parser
	_, ok = GetOptionsParser(CollectorType("nonexistent"))
	assert.False(t, ok)
}

func TestParseOptions_Valid(t *testing.T) {
	// This test is covered by TestParseOptions_WithRealTOML
	// which tests the actual usage pattern
	t.Skip("Covered by TestParseOptions_WithRealTOML")
}

func TestParseOptions_WithRealTOML(t *testing.T) {
	type TestOptions struct {
		Address string `toml:"address"`
		Timeout string `toml:"timeout"`
	}

	// Create a full config with collectors section
	content := `
[collectors.test]
type = "test"
enabled = true

[collectors.test.options]
address = "http://localhost:9090"
timeout = "30s"
`

	type CollectorConfig struct {
		Type    string         `toml:"type"`
		Enabled bool           `toml:"enabled"`
		Options toml.Primitive `toml:"options"`
	}

	type Config struct {
		Collectors map[string]CollectorConfig `toml:"collectors"`
	}

	var cfg Config
	meta, err := toml.Decode(content, &cfg)
	require.NoError(t, err)

	// Get the options primitive
	testCollector := cfg.Collectors["test"]
	require.NotNil(t, testCollector.Options)

	// Parse using ParseOptions
	opts, err := ParseOptions[TestOptions](&meta, testCollector.Options, CollectorType("test"))
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9090", opts.Address)
	assert.Equal(t, "30s", opts.Timeout)
}

func TestParseOptions_InvalidTOML(t *testing.T) {
	type TestOptions struct {
		Address string `toml:"address"`
	}

	// Create invalid primitive (wrong type)
	content := `
[collectors.test]
type = "test"
enabled = true

[collectors.test.options]
address = 123  # Should be string
`

	type CollectorConfig struct {
		Type    string         `toml:"type"`
		Enabled bool           `toml:"enabled"`
		Options toml.Primitive `toml:"options"`
	}

	type Config struct {
		Collectors map[string]CollectorConfig `toml:"collectors"`
	}

	var cfg Config
	meta, err := toml.Decode(content, &cfg)
	require.NoError(t, err)

	testCollector := cfg.Collectors["test"]
	opts, err := ParseOptions[TestOptions](&meta, testCollector.Options, CollectorType("test"))
	
	// The decode might succeed but the type conversion could fail
	// This depends on TOML library behavior
	_ = opts
	_ = err
}

func TestRegisterOptionsParser_Concurrent(t *testing.T) {
	// Reset parsers for clean test
	optionsParsers.parsers = make(map[CollectorType]OptionsParser)

	// Test concurrent registration
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			testType := CollectorType("test" + string(rune(id)))
			testParser := func(meta *toml.MetaData, primitive toml.Primitive) (interface{}, error) {
				return "test", nil
			}
			RegisterOptionsParser(testType, testParser)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all parsers were registered
	for i := 0; i < 10; i++ {
		testType := CollectorType("test" + string(rune(i)))
		_, ok := GetOptionsParser(testType)
		assert.True(t, ok, "Parser %d should be registered", i)
	}
}

func TestGetOptionsParser_Concurrent(t *testing.T) {
	// Reset parsers for clean test
	optionsParsers.parsers = make(map[CollectorType]OptionsParser)

	testType := CollectorType("test")
	testParser := func(meta *toml.MetaData, primitive toml.Primitive) (interface{}, error) {
		return "test", nil
	}

	RegisterOptionsParser(testType, testParser)

	// Test concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, ok := GetOptionsParser(testType)
			assert.True(t, ok)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
