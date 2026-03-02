package agent_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pithecene-io/bonsai/internal/agent"
)

func TestModel_IsHaiku(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"haiku", true},
		{"Haiku", true},
		{"claude-3-5-haiku-latest", true},
		{"sonnet", false},
		{"codex", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := agent.Model(tt.model).IsHaiku(); got != tt.want {
			t.Errorf("Model(%q).IsHaiku() = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestModel_IsCodex(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"codex", true},
		{"codex-mini", true},
		{"Codex-4o", true},
		{"haiku", false},
		{"sonnet", false},
		{"mycodex", false}, // prefix, not substring
		{"", false},
	}
	for _, tt := range tests {
		if got := agent.Model(tt.model).IsCodex(); got != tt.want {
			t.Errorf("Model(%q).IsCodex() = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestModel_IsLite(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"haiku", true},
		{"codex", true},
		{"codex-mini", true},
		{"sonnet", false},
		{"opus", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := agent.Model(tt.model).IsLite(); got != tt.want {
			t.Errorf("Model(%q).IsLite() = %v, want %v", tt.model, got, tt.want)
		}
	}
}

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
		EvaluateResponse: "ok",
	}

	_, _ = mock.Evaluate(t.Context(), "sys", "user", agent.Model("haiku"))

	if mock.CallCount() != 1 {
		t.Fatalf("CallCount = %d, want 1", mock.CallCount())
	}
	if got := mock.EvaluateCalls[0].Model; got != "haiku" {
		t.Errorf("Model = %q, want haiku", got)
	}
}

func TestMockAgent_FuncOverridesResponse(t *testing.T) {
	mock := &agent.MockAgent{
		NameVal:                "test",
		EvaluateResponse: "should not appear",
		EvaluateFunc: func(_ context.Context, _, _ string, model agent.Model) (string, error) {
			return "model=" + string(model), nil
		},
	}

	got, err := mock.Evaluate(t.Context(), "sys", "user", agent.Model("opus"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got != "model=opus" {
		t.Errorf("got %q, want model=opus", got)
	}
}

// TestClaude_Evaluate_ModelArg verifies the --model flag is passed to
// the subprocess. Uses a shell script stub that echoes its arguments.
func TestClaude_Evaluate_ModelArg(t *testing.T) {
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
	out, err := c.Evaluate(t.Context(), "test-system", "test-user", agent.Model("haiku"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
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
	// Haiku should get --effort low for latency
	if !strings.Contains(args, "--effort") || !strings.Contains(args, "low") {
		t.Errorf("haiku args missing --effort low:\n%s", args)
	}
}

// TestClaude_Evaluate_NoEffortForSonnet verifies --effort is NOT
// passed for non-haiku models.
func TestClaude_Evaluate_NoEffortForSonnet(t *testing.T) {
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
	_, err := c.Evaluate(t.Context(), "test-system", "test-user", agent.Model("sonnet"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	argsData, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := string(argsData)

	if strings.Contains(args, "--effort") {
		t.Errorf("sonnet args should NOT contain --effort:\n%s", args)
	}
}

func TestModel_IsClaude(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"haiku", true},
		{"sonnet", true},
		{"opus", true},
		{"claude-3-5-haiku-latest", true},
		{"claude-sonnet-4-6", true},
		{"Claude-Opus-4-6", true},
		{"codex", false},
		{"codex-mini", false},
		{"gpt-4", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := agent.Model(tt.model).IsClaude(); got != tt.want {
			t.Errorf("Model(%q).IsClaude() = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestModel_Tier(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{"haiku", "haiku"},
		{"claude-3-5-haiku-latest", "haiku"},
		{"sonnet", "sonnet"},
		{"claude-sonnet-4-6", "sonnet"},
		{"opus", "opus"},
		{"claude-opus-4-6", "opus"},
		{"codex", "codex"},
		{"gpt-4", "gpt-4"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := agent.Model(tt.model).Tier(); got != tt.want {
			t.Errorf("Model(%q).Tier() = %q, want %q", tt.model, got, tt.want)
		}
	}
}

func TestRouter_Implements(_ *testing.T) {
	// Compile-time interface check.
	var _ agent.Agent = (*agent.Router)(nil)
}

func TestRouter_RoutesToClaude(t *testing.T) {
	// Ensure no credentials cause Anthropic routing.
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	fakeBin := filepath.Join(dir, "fake-claude")

	script := `#!/bin/sh
printf '%s\n' "$@" > "` + argsFile + `"
cat
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	r := agent.NewRouter(fakeBin, "nonexistent-codex")
	out, err := r.Evaluate(t.Context(), "sys", "user-input", agent.Model("haiku"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !strings.Contains(out, "user-input") {
		t.Errorf("output = %q, expected user-input", out)
	}

	argsData, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	if !strings.Contains(string(argsData), "--model") || !strings.Contains(string(argsData), "haiku") {
		t.Errorf("expected --model haiku in claude args:\n%s", argsData)
	}
}

func TestRouter_RoutesToCodex(t *testing.T) {
	// Ensure no ambient API key interferes with routing.
	t.Setenv("ANTHROPIC_API_KEY", "")

	dir := t.TempDir()
	markerFile := filepath.Join(dir, "codex-called")
	fakeCodex := filepath.Join(dir, "fake-codex")

	script := `#!/bin/sh
echo "codex-was-called" > "` + markerFile + `"
cat
`
	if err := os.WriteFile(fakeCodex, []byte(script), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	r := agent.NewRouter("nonexistent-claude", fakeCodex)
	out, err := r.Evaluate(t.Context(), "sys", "user-input", agent.Model("codex"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !strings.Contains(out, "user-input") {
		t.Errorf("output = %q, expected user-input", out)
	}

	// Verify codex was actually called
	if _, err := os.Stat(markerFile); err != nil {
		t.Errorf("codex marker file not created — codex was not called")
	}
}

func TestRouter_RoutesToCodexVariant(t *testing.T) {
	// Ensure no ambient API key interferes with routing.
	t.Setenv("ANTHROPIC_API_KEY", "")

	dir := t.TempDir()
	markerFile := filepath.Join(dir, "codex-called")
	fakeCodex := filepath.Join(dir, "fake-codex")

	script := `#!/bin/sh
echo "codex-was-called" > "` + markerFile + `"
cat
`
	if err := os.WriteFile(fakeCodex, []byte(script), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	r := agent.NewRouter("nonexistent-claude", fakeCodex)
	out, err := r.Evaluate(t.Context(), "sys", "user-input", agent.Model("codex-mini"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !strings.Contains(out, "user-input") {
		t.Errorf("output = %q, expected user-input", out)
	}

	// Verify codex was actually called (not claude)
	if _, err := os.Stat(markerFile); err != nil {
		t.Errorf("codex marker file not created — codex-mini was not routed to codex backend")
	}
}

func TestRouter_RoutesToAnthropicDirect(t *testing.T) {
	// When Anthropic is non-nil and model is a claude-family model,
	// the router should use the Anthropic backend.
	r := agent.NewRouter("nonexistent-claude", "nonexistent-codex", agent.WithAPIKey("test-key"))
	if r.Anthropic == nil {
		t.Fatal("expected Anthropic backend to be non-nil with explicit key")
	}
	if r.Anthropic.Name() != "anthropic" {
		t.Errorf("Anthropic.Name() = %q, want %q", r.Anthropic.Name(), "anthropic")
	}
}

func TestRouter_FallsBackToClaudeWhenNoKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	// Isolate HOME so OAuth credential lookup also finds nothing.
	t.Setenv("HOME", t.TempDir())
	r := agent.NewRouter("nonexistent-claude", "nonexistent-codex")
	if r.Anthropic != nil {
		t.Error("expected Anthropic backend to be nil when no credentials are available")
	}
}

// TestRouter_FallsBackToClaudeOnAnthropicError verifies that when the
// Anthropic direct API fails (e.g. bad key, outage), the router falls
// back to Claude CLI instead of hard-failing.
//
// Uses MockAgent for the Anthropic slot — no network calls.
func TestRouter_FallsBackToClaudeOnAnthropicError(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	fakeBin := filepath.Join(dir, "fake-claude")

	script := `#!/bin/sh
printf '%s\n' "$@" > "` + argsFile + `"
cat
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Construct router manually: Anthropic slot is a mock that always
	// returns an error, simulating auth failure / outage.
	t.Setenv("ANTHROPIC_API_KEY", "")
	r := &agent.Router{
		Claude: agent.NewClaude(fakeBin),
		Codex:  agent.NewCodex("nonexistent-codex"),
		Anthropic: &agent.MockAgent{
			NameVal:           "anthropic",
			EvaluateErr: errors.New("401 authentication_error"),
		},
	}

	out, err := r.Evaluate(t.Context(), "sys", "user-input", agent.Model("haiku"))
	if err != nil {
		t.Fatalf("Evaluate should succeed via Claude CLI fallback: %v", err)
	}
	if !strings.Contains(out, "user-input") {
		t.Errorf("output = %q, expected user-input from Claude CLI fallback", out)
	}

	// Verify Claude CLI was actually called after Anthropic failure.
	argsData, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	if !strings.Contains(string(argsData), "--model") || !strings.Contains(string(argsData), "haiku") {
		t.Errorf("expected --model haiku in Claude CLI fallback args:\n%s", argsData)
	}

	// Verify Anthropic was attempted first.
	mock := r.Anthropic.(*agent.MockAgent)
	if mock.CallCount() != 1 {
		t.Errorf("Anthropic CallCount = %d, want 1", mock.CallCount())
	}
}

// TestClaude_Evaluate_NoModelWhenEmpty verifies --model is omitted
// when model is empty.
// TestRouter_NoFallbackOnContextCancel verifies that when the context is
// already canceled, the router does NOT fall back to Claude CLI — a canceled
// context means the caller is done.
func TestRouter_NoFallbackOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	markerFile := filepath.Join(dir, "claude-called")
	fakeBin := filepath.Join(dir, "fake-claude")

	script := `#!/bin/sh
echo "claude-was-called" > "` + markerFile + `"
cat
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // cancel before calling Evaluate

	r := &agent.Router{
		Claude: agent.NewClaude(fakeBin),
		Codex:  agent.NewCodex("nonexistent-codex"),
		Anthropic: &agent.MockAgent{
			NameVal:           "anthropic",
			EvaluateErr: context.Canceled,
		},
	}

	_, err := r.Evaluate(ctx, "sys", "user-input", agent.Model("haiku"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled in error chain, got: %v", err)
	}

	// Verify Claude CLI was NOT called.
	if _, statErr := os.Stat(markerFile); statErr == nil {
		t.Error("Claude CLI marker file was created — fallback should be skipped on context cancellation")
	}

	// Verify Anthropic was attempted exactly once.
	mock := r.Anthropic.(*agent.MockAgent)
	if mock.CallCount() != 1 {
		t.Errorf("Anthropic CallCount = %d, want 1", mock.CallCount())
	}
}

// TestRouter_FallbackPreservesBothErrors verifies that when both the
// Anthropic direct API and the Claude CLI fallback fail, the returned
// error contains both root causes.
func TestRouter_FallbackPreservesBothErrors(t *testing.T) {
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "fake-claude")

	// Fake Claude binary that always exits 1 with known stderr.
	script := `#!/bin/sh
echo "claude-cli: connection refused" >&2
exit 1
`
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	r := &agent.Router{
		Claude: agent.NewClaude(fakeBin),
		Codex:  agent.NewCodex("nonexistent-codex"),
		Anthropic: &agent.MockAgent{
			NameVal:           "anthropic",
			EvaluateErr: errors.New("401 authentication_error"),
		},
	}

	_, err := r.Evaluate(t.Context(), "sys", "user-input", agent.Model("haiku"))
	if err == nil {
		t.Fatal("expected error when both backends fail, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "401 authentication_error") {
		t.Errorf("combined error should contain Anthropic error, got: %v", err)
	}
	if !strings.Contains(errStr, "connection refused") {
		t.Errorf("combined error should contain Claude CLI error, got: %v", err)
	}
}

func TestClaude_Evaluate_NoModelWhenEmpty(t *testing.T) {
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
	_, err := c.Evaluate(t.Context(), "test-system", "test-user", agent.Model(""))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
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
