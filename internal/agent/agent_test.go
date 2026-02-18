package agent_test

import (
	"testing"

	"github.com/pithecene-io/bonsai/internal/agent"
)

func TestNewClaude_DefaultBin(t *testing.T) {
	c := agent.NewClaude("")
	if c.Bin != "claude" {
		t.Errorf("Bin = %q, want %q", c.Bin, "claude")
	}
}

func TestNewClaude_CustomBin(t *testing.T) {
	c := agent.NewClaude("/usr/local/bin/claude")
	if c.Bin != "/usr/local/bin/claude" {
		t.Errorf("Bin = %q, want %q", c.Bin, "/usr/local/bin/claude")
	}
}

func TestNewCodex_DefaultBin(t *testing.T) {
	c := agent.NewCodex("")
	if c.Bin != "codex" {
		t.Errorf("Bin = %q, want %q", c.Bin, "codex")
	}
}

func TestClaude_Name(t *testing.T) {
	c := agent.NewClaude("")
	if c.Name() != "claude" {
		t.Errorf("Name() = %q, want %q", c.Name(), "claude")
	}
}

func TestCodex_Name(t *testing.T) {
	c := agent.NewCodex("")
	if c.Name() != "codex" {
		t.Errorf("Name() = %q, want %q", c.Name(), "codex")
	}
}

func TestMockAgent_Implements(_ *testing.T) {
	// Compile-time interface check.
	var _ agent.Agent = (*agent.MockAgent)(nil)
}
