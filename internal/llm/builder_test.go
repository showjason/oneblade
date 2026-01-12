package llm

import (
	"context"
	"os"
	"testing"

	"github.com/oneblade/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAPIKey(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *config.AgentLLMConfig
		apiKey       string
		providerName string
		setupEnv     func()
		wantKey      string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "API key from config",
			cfg:          &config.AgentLLMConfig{APIKey: "config-key"},
			apiKey:       "OPENAI_API_KEY",
			providerName: "openai",
			wantKey:      "config-key",
			wantErr:      false,
		},
		{
			name:         "API key from env variable",
			cfg:          &config.AgentLLMConfig{APIKey: ""},
			apiKey:       "OPENAI_API_KEY",
			providerName: "openai",
			setupEnv: func() {
				os.Setenv("OPENAI_API_KEY", "env-key")
			},
			wantKey: "env-key",
			wantErr: false,
		},
		{
			name:         "API key from config with whitespace",
			cfg:          &config.AgentLLMConfig{APIKey: "  config-key  "},
			apiKey:       "OPENAI_API_KEY",
			providerName: "openai",
			wantKey:      "config-key",
			wantErr:      false,
		},
		{
			name:         "API key from second env variable",
			cfg:          &config.AgentLLMConfig{APIKey: ""},
			apiKey:       "FIRST_KEY",
			providerName: "provider",
			setupEnv: func() {
				os.Setenv("FIRST_KEY", "first-key")
			},
			wantKey:      "first-key",
			wantErr:      false,
		},
		{
			name:         "Config key takes priority over api key",
			cfg:          &config.AgentLLMConfig{APIKey: "config-key"},
			apiKey:       "OPENAI_API_KEY",
			providerName: "openai",
			setupEnv: func() {
				os.Setenv("OPENAI_API_KEY", "env-key")
			},
			wantKey: "config-key",
			wantErr: false,
		},
		{
			name:         "Missing API key",
			cfg:          &config.AgentLLMConfig{APIKey: ""},
			apiKey:       "OPENAI_API_KEY",
			providerName: "openai",
			wantErr:      true,
			errContains:  "api key not configured",
		},
		{
			name:         "Multiple env keys, first one set",
			cfg:          &config.AgentLLMConfig{APIKey: ""},
			apiKey:       "FIRST_KEY",
			providerName: "provider",
			setupEnv: func() {
				os.Setenv("FIRST_KEY", "first-key")
				os.Setenv("SECOND_KEY", "second-key")
			},
			wantKey: "first-key",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupEnv != nil {
				defer func() {
					os.Unsetenv(tt.apiKey)
				}()
				tt.setupEnv()
			}

			gotKey, err := resolveAPIKey(tt.cfg, tt.apiKey)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Empty(t, gotKey)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantKey, gotKey)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		setup   func()
		want    []string
		cleanup func()
	}{
		{
			name: "single key",
			key:  "TEST_KEY",
			setup: func() {
				os.Setenv("TEST_KEY", "test-value")
			},
			want: []string{"test-value"},
			cleanup: func() {
				os.Unsetenv("TEST_KEY")
			},
		},
		{
			name: "multiple keys",
			key:  "KEY1",
			setup: func() {
				os.Setenv("KEY1", "value1")
				os.Setenv("KEY2", "value2")
			},
			want: []string{"value1", "value2"},
			cleanup: func() {
				os.Unsetenv("KEY1")
				os.Unsetenv("KEY2")
			},
		},
		{
			name: "unset key returns empty",
			key:  "UNSET_KEY",
			want: []string{""},
		},
		{
			name: "mixed set and unset keys",
			key:  "SET_KEY",
			setup: func() {
				os.Setenv("SET_KEY", "value")
			},
			want: []string{"value", ""},
			cleanup: func() {
				os.Unsetenv("SET_KEY")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			if tt.cleanup != nil {
				defer tt.cleanup()
			}

			got := os.Getenv(tt.key)
			assert.Equal(t, tt.want[0], got)
		})
	}
}

func TestSetBaseURL(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.AgentLLMConfig
		defaultURL  string
		expectedURL string
	}{
		{
			name:        "use config base URL",
			cfg:         &config.AgentLLMConfig{BaseURL: "https://custom.url"},
			defaultURL:  "https://default.url",
			expectedURL: "https://custom.url",
		},
		{
			name:        "use default base URL when config is empty",
			cfg:         &config.AgentLLMConfig{BaseURL: ""},
			defaultURL:  "https://default.url",
			expectedURL: "https://default.url",
		},
		{
			name:        "use default base URL when config is whitespace",
			cfg:         &config.AgentLLMConfig{BaseURL: "   "},
			defaultURL:  "https://default.url",
			expectedURL: "https://default.url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveBaseURL(tt.cfg, tt.defaultURL)
			assert.Equal(t, tt.expectedURL, got)
		})
	}
}

func TestModelBuilder_OpenAIBuilder(t *testing.T) {
	builder := newOpenAIBuilder()

	t.Run("Name returns correct name", func(t *testing.T) {
		assert.Equal(t, "gpt-4", builder.GetModel(&config.AgentLLMConfig{Model: "gpt-4"}))
	})

	t.Run("GetBaseURL returns config URL when set", func(t *testing.T) {
		cfg := &config.AgentLLMConfig{BaseURL: "https://custom.url"}
		assert.Equal(t, "https://custom.url", builder.GetBaseURL(cfg))
	})

	t.Run("GetBaseURL returns default URL when config is empty", func(t *testing.T) {
		cfg := &config.AgentLLMConfig{BaseURL: ""}
		assert.Equal(t, "https://api.openai.com/v1", builder.GetBaseURL(cfg))
	})

	t.Run("Build with API key from config", func(t *testing.T) {
		maxTokens := 2048
		temperature := 0.7
		cfg := &config.AgentLLMConfig{
			APIKey:      "test-api-key",
			Model:       "gpt-4",
			BaseURL:     "https://api.openai.com/v1",
			MaxTokens:   &maxTokens,
			Temperature: &temperature,
		}

		model, err := builder.Build(context.Background(), cfg)
		assert.NoError(t, err)
		assert.NotNil(t, model)
	})

	t.Run("Build fails with missing API key", func(t *testing.T) {
		maxTokens := 2048
		temperature := 0.7
		cfg := &config.AgentLLMConfig{
			APIKey:      "",
			Model:       "gpt-4",
			MaxTokens:   &maxTokens,
			Temperature: &temperature,
		}

		os.Setenv("OPENAI_API_KEY", "")
		defer os.Unsetenv("OPENAI_API_KEY")

		_, err := builder.Build(context.Background(), cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "api key not configured")
	})
}

func TestModelBuilder_AnthropicBuilder(t *testing.T) {
	builder := newAnthropicBuilder()

	t.Run("Name returns correct name", func(t *testing.T) {
		assert.Equal(t, "claude-3", builder.GetModel(&config.AgentLLMConfig{Model: "claude-3"}))
	})

	t.Run("GetBaseURL returns config URL when set", func(t *testing.T) {
		cfg := &config.AgentLLMConfig{BaseURL: "https://custom.url"}
		assert.Equal(t, "https://custom.url", builder.GetBaseURL(cfg))
	})

	t.Run("GetBaseURL returns default URL when config is empty", func(t *testing.T) {
		cfg := &config.AgentLLMConfig{BaseURL: ""}
		assert.Equal(t, "https://api.anthropic.com", builder.GetBaseURL(cfg))
	})

	t.Run("Build with API key from config", func(t *testing.T) {
		maxTokens := 2048
		temperature := 0.7
		cfg := &config.AgentLLMConfig{
			APIKey:      "test-api-key",
			Model:       "claude-3",
			BaseURL:     "https://api.anthropic.com",
			MaxTokens:   &maxTokens,
			Temperature: &temperature,
		}

		model, err := builder.Build(context.Background(), cfg)
		assert.NoError(t, err)
		assert.NotNil(t, model)
	})

	t.Run("Build fails with missing API key", func(t *testing.T) {
		maxTokens := 2048
		temperature := 0.7
		cfg := &config.AgentLLMConfig{
			APIKey:      "",
			Model:       "claude-3",
			MaxTokens:   &maxTokens,
			Temperature: &temperature,
		}

		os.Setenv("ANTHROPIC_API_KEY", "")
		defer os.Unsetenv("ANTHROPIC_API_KEY")

		_, err := builder.Build(context.Background(), cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "api key not configured")
	})
}

func TestModelBuilder_GeminiBuilder(t *testing.T) {
	builder := newGeminiBuilder()

	t.Run("Name returns correct name", func(t *testing.T) {
		assert.Equal(t, "gemini-1.5-pro", builder.GetModel(&config.AgentLLMConfig{Model: "gemini-1.5-pro"}))
	})

	t.Run("GetBaseURL returns empty string", func(t *testing.T) {
		cfg := &config.AgentLLMConfig{BaseURL: "https://custom.url"}
		assert.Equal(t, "https://custom.url", builder.GetBaseURL(cfg))
	})

	t.Run("Build with API key from config", func(t *testing.T) {
		maxTokens := 2048
		temperature := 0.7
		cfg := &config.AgentLLMConfig{
			APIKey:      "test-api-key",
			Model:       "gemini-1.5-pro",
			MaxTokens:   &maxTokens,
			Temperature: &temperature,
		}

		model, err := builder.Build(context.Background(), cfg)
		assert.NoError(t, err)
		assert.NotNil(t, model)
	})

	t.Run("Build fails with missing API key", func(t *testing.T) {
		maxTokens := 2048
		temperature := 0.7
		cfg := &config.AgentLLMConfig{
			APIKey:      "",
			Model:       "gemini-1.5-pro",
			MaxTokens:   &maxTokens,
			Temperature: &temperature,
		}

		os.Setenv("GEMINI_API_KEY", "")
		os.Setenv("GOOGLE_API_KEY", "")
		defer func() {
			os.Unsetenv("GEMINI_API_KEY")
			os.Unsetenv("GOOGLE_API_KEY")
		}()

		_, err := builder.Build(context.Background(), cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "api key not configured")
	})

	t.Run("Build uses second env key if first is empty", func(t *testing.T) {
		maxTokens := 2048
		temperature := 0.7
		cfg := &config.AgentLLMConfig{
			APIKey:      "",
			Model:       "gemini-1.5-pro",
			MaxTokens:   &maxTokens,
			Temperature: &temperature,
		}

		os.Setenv("GEMINI_API_KEY", "")
		os.Setenv("GOOGLE_API_KEY", "google-api-key")
		defer func() {
			os.Unsetenv("GEMINI_API_KEY")
			os.Unsetenv("GOOGLE_API_KEY")
		}()

		model, err := builder.Build(context.Background(), cfg)
		assert.NoError(t, err)
		assert.NotNil(t, model)
	})
}

func TestFactory_Build_AllProviders(t *testing.T) {
	tests := []struct {
		name      string
		provider  string
		model     string
		setupEnv  func()
		cleanup   func()
		wantErr   bool
		errString string
	}{
		{
			name:     "OpenAI provider",
			provider: "openai",
			model:    "gpt-4",
			setupEnv: func() {
				os.Setenv("OPENAI_API_KEY", "test-key")
			},
			cleanup: func() {
				os.Unsetenv("OPENAI_API_KEY")
			},
			wantErr: false,
		},
		{
			name:     "Anthropic provider",
			provider: "anthropic",
			model:    "claude-3",
			setupEnv: func() {
				os.Setenv("ANTHROPIC_API_KEY", "test-key")
			},
			cleanup: func() {
				os.Unsetenv("ANTHROPIC_API_KEY")
			},
			wantErr: false,
		},
		{
			name:     "Gemini provider",
			provider: "gemini",
			model:    "gemini-1.5-pro",
			setupEnv: func() {
				os.Setenv("GEMINI_API_KEY", "test-key")
			},
			cleanup: func() {
				os.Unsetenv("GEMINI_API_KEY")
			},
			wantErr: false,
		},
		{
			name:      "Unsupported provider",
			provider:  "unsupported",
			model:     "model",
			wantErr:   true,
			errString: "oneof", // validator error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupEnv != nil {
				tt.setupEnv()
			}
			if tt.cleanup != nil {
				defer tt.cleanup()
			}

			f := NewFactory()
			maxTokens := 2048
			temperature := 0.7
			cfg := config.AgentLLMConfig{
				Provider:    tt.provider,
				Model:       tt.model,
				MaxTokens:   &maxTokens,
				Temperature: &temperature,
			}

			model, err := f.Build(context.Background(), cfg)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errString)
				assert.Nil(t, model)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, model)
			}
		})
	}
}

func TestFactory_Build_CaseInsensitiveProvider(t *testing.T) {
	tests := []struct {
		name        string
		provider    string
		shouldBuild bool
	}{
		{"OpenAI lowercase", "openai", true},
		{"OpenAI uppercase", "OPENAI", true},
		{"OpenAI mixed case", "OpenAI", true},
		{"Anthropic lowercase", "anthropic", true},
		{"Anthropic uppercase", "ANTHROPIC", true},
		{"Anthropic mixed case", "Anthropic", true},
		{"Gemini lowercase", "gemini", true},
		{"Gemini uppercase", "GEMINI", true},
		{"Gemini mixed case", "Gemini", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("TEST_API_KEY", "test-key")
			defer os.Unsetenv("TEST_API_KEY")

			f := NewFactory()
			maxTokens := 2048
			temperature := 0.7

			envKey := ""
			switch tt.provider {
			case "openai", "OPENAI", "OpenAI":
				envKey = "OPENAI_API_KEY"
			case "anthropic", "ANTHROPIC", "Anthropic":
				envKey = "ANTHROPIC_API_KEY"
			case "gemini", "GEMINI", "Gemini":
				envKey = "GEMINI_API_KEY"
			}
			os.Setenv(envKey, "test-key")
			defer os.Unsetenv(envKey)

			cfg := config.AgentLLMConfig{
				Provider:    tt.provider,
				Model:       "test-model",
				MaxTokens:   &maxTokens,
				Temperature: &temperature,
			}

			model, err := f.Build(context.Background(), cfg)

			if tt.shouldBuild {
				require.NoError(t, err)
				assert.NotNil(t, model)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	t.Run("sets default timeout when empty", func(t *testing.T) {
		cfg := &config.AgentLLMConfig{Timeout: ""}
		applyDefaults(cfg)
		assert.Equal(t, "60s", cfg.Timeout)
	})

	t.Run("preserves timeout when set", func(t *testing.T) {
		cfg := &config.AgentLLMConfig{Timeout: "30s"}
		applyDefaults(cfg)
		assert.Equal(t, "30s", cfg.Timeout)
	})

	t.Run("sets default MaxTokens when nil", func(t *testing.T) {
		cfg := &config.AgentLLMConfig{MaxTokens: nil}
		applyDefaults(cfg)
		require.NotNil(t, cfg.MaxTokens)
		assert.Equal(t, 2048, *cfg.MaxTokens)
	})

	t.Run("preserves MaxTokens when set", func(t *testing.T) {
		maxTokens := 4096
		cfg := &config.AgentLLMConfig{MaxTokens: &maxTokens}
		applyDefaults(cfg)
		assert.Equal(t, 4096, *cfg.MaxTokens)
	})

	t.Run("sets default Temperature when nil", func(t *testing.T) {
		cfg := &config.AgentLLMConfig{Temperature: nil}
		applyDefaults(cfg)
		require.NotNil(t, cfg.Temperature)
		assert.Equal(t, 0.7, *cfg.Temperature)
	})

	t.Run("preserves Temperature when set", func(t *testing.T) {
		temperature := 0.5
		cfg := &config.AgentLLMConfig{Temperature: &temperature}
		applyDefaults(cfg)
		assert.Equal(t, 0.5, *cfg.Temperature)
	})
}
