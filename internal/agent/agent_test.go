package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestMockAgent_RecordsModel(t *testing.T) {
	mock := &agent.MockAgent{
		NameVal:                "test",
		NonInteractiveResponse: "ok",
	}

	_, _ = mock.NonInteractive(context.Background(), "sys", "user", "haiku")

	if mock.CallCount() != 1 {
		t.Fatalf("CallCount = %d, want 1", mock.CallCount())
	}
	if got := mock.NonInteractiveCalls[0].Model; got != "haiku" {
		t.Errorf("Model = %q, want haiku", got)
	}
}

func TestMockAgent_FuncOverridesResponse(t *testing.T) {
	mock := &agent.MockAgent{
		NameVal:                "test",
		NonInteractiveResponse: "should not appear",
		NonInteractiveFunc: func(_ context.Context, _, _, model string) (string, error) {
			return "model=" + model, nil
		},
	}

	got, err := mock.NonInteractive(context.Background(), "sys", "user", "opus")
	if err != nil {
		t.Fatalf("NonInteractive: %v", err)
	}
	if got != "model=opus" {
		t.Errorf("got %q, want model=opus", got)
	}
}

// TestClaude_NonInteractive_ModelArg verifies the --model flag is passed to
// the subprocess. Uses a shell script stub that echoes its arguments.
func TestClaude_NonInteractive_ModelArg(t *testing.T) {
	// Create a fake "claude" binary that dumps its args to a file
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	fakeBin := filepath.Join(dir, "fake-claude")

	// The fake binary writes all args and echoes stdin
	script := `#!/bin/sh
printf '%s\n' "$@" > "` + argsFile + `"
cat
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	c := agent.NewClaude(fakeBin)
	out, err := c.NonInteractive(context.Background(), "test-system", "test-user", "haiku")
	if err != nil {
		t.Fatalf("NonInteractive: %v", err)
	}

	// The fake binary echoes stdin, so output should contain the user prompt
	if !strings.Contains(out, "test-user") {
		t.Errorf("output = %q, expected to contain test-user", out)
	}

	// Read the captured args
	argsData, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := string(argsData)

	// Verify --model haiku appears in args
	if !strings.Contains(args, "--model") {
		t.Errorf("args missing --model flag:\n%s", args)
	}
	if !strings.Contains(args, "haiku") {
		t.Errorf("args missing model value 'haiku':\n%s", args)
	}
}

// TestClaude_NonInteractive_NoModelWhenEmpty verifies --model is omitted
// when model is empty.
func TestClaude_NonInteractive_NoModelWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	fakeBin := filepath.Join(dir, "fake-claude")

	script := `#!/bin/sh
printf '%s\n' "$@" > "` + argsFile + `"
cat
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	c := agent.NewClaude(fakeBin)
	_, err := c.NonInteractive(context.Background(), "test-system", "test-user", "")
	if err != nil {
		t.Fatalf("NonInteractive: %v", err)
	}

	argsData, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := string(argsData)

	if strings.Contains(args, "--model") {
		t.Errorf("args should not contain --model when model is empty:\n%s", args)
	}
}
