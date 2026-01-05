package llm

import (
	"context"
	"testing"

	"github.com/oneblade/config"
)

func TestFactory_Build_ValidatorMissingProvider(t *testing.T) {
	f := NewFactory()
	_, err := f.Build(context.Background(), config.AgentLLMConfig{Model: "gpt-4"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestFactory_Build_OpenAI_MissingAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	f := NewFactory()
	_, err := f.Build(context.Background(), config.AgentLLMConfig{
		Provider: "openai",
		Model:    "gpt-4",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestFactory_Build_Gemini_MissingAPIKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")

	f := NewFactory()
	_, err := f.Build(context.Background(), config.AgentLLMConfig{
		Provider: "gemini",
		Model:    "gemini-1.5-pro",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}


