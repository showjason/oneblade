package config

import "testing"

func TestLLMConfig_GetAgentStrict_Missing(t *testing.T) {
	cfg := &LLMConfig{
		Agents: map[string]AgentLLMConfig{
			"a": {Provider: "openai", Model: "gpt-4"},
		},
	}
	if _, err := cfg.GetAgentStrict("missing"); err == nil {
		t.Fatalf("expected error for missing agent config")
	}
}

func TestLLMConfig_GetAgentStrict_OK(t *testing.T) {
	cfg := &LLMConfig{
		Agents: map[string]AgentLLMConfig{
			"a": {Provider: "openai", Model: "gpt-4"},
		},
	}
	got, err := cfg.GetAgentStrict("a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Provider != "openai" || got.Model != "gpt-4" {
		t.Fatalf("unexpected cfg: %#v", got)
	}
}


