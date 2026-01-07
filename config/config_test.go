package config

import "testing"

func TestConfig_GetAgentConfig_Missing(t *testing.T) {
	cfg := &Config{
		Agents: map[string]AgentConfig{
			"a": {Enabled: true, LLM: AgentLLMConfig{Provider: "openai", Model: "gpt-4"}},
		},
	}
	if _, err := cfg.GetAgentConfig("missing"); err == nil {
		t.Fatalf("expected error for missing agent config")
	}
}

func TestConfig_GetAgentConfig_OK(t *testing.T) {
	cfg := &Config{
		Agents: map[string]AgentConfig{
			"a": {Enabled: true, LLM: AgentLLMConfig{Provider: "openai", Model: "gpt-4"}},
		},
	}
	got, err := cfg.GetAgentConfig("a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.LLM.Provider != "openai" || got.LLM.Model != "gpt-4" {
		t.Fatalf("unexpected cfg: %#v", got)
	}
}
