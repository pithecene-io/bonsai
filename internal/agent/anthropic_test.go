package agent_test

import (
	"testing"

	"github.com/pithecene-io/bonsai/internal/agent"
)

func TestAnthropic_Implements(_ *testing.T) {
	// Compile-time interface check.
	var _ agent.Agent = (*agent.Anthropic)(nil)
}

func TestAnthropic_Name(t *testing.T) {
	// Use a dummy key so NewAnthropic returns non-nil.
	a := agent.NewAnthropic(agent.WithAPIKey("test-key"))
	if a == nil {
		t.Fatal("NewAnthropic returned nil with explicit key")
	}
	if got := a.Name(); got != "anthropic" {
		t.Errorf("Name() = %q, want %q", got, "anthropic")
	}
}

func TestAnthropic_InteractiveReturnsError(t *testing.T) {
	a := agent.NewAnthropic(agent.WithAPIKey("test-key"))
	if a == nil {
		t.Fatal("NewAnthropic returned nil with explicit key")
	}
	err := a.Interactive(t.Context(), "sys", nil)
	if err == nil {
		t.Error("Interactive should return an error for direct API backend")
	}
}

func TestNewAnthropic_NilWithoutKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	a := agent.NewAnthropic()
	if a != nil {
		t.Error("NewAnthropic should return nil when no API key is available")
	}
}

func TestNewAnthropic_UsesEnvKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-env-key")
	a := agent.NewAnthropic()
	if a == nil {
		t.Error("NewAnthropic should return non-nil when ANTHROPIC_API_KEY is set")
	}
}

func TestNewAnthropic_ExplicitKeyOverridesEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	a := agent.NewAnthropic(agent.WithAPIKey("explicit-key"))
	if a == nil {
		t.Error("NewAnthropic should return non-nil with explicit key even when env is empty")
	}
}
