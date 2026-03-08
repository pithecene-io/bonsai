package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Codex implements the Agent interface by shelling out to the codex CLI.
type Codex struct {
	// Bin is the path to the codex binary. Defaults to "codex".
	Bin string
}

// NewCodex creates a Codex agent with the given binary path.
func NewCodex(bin string) *Codex {
	if bin == "" {
		bin = "codex"
	}
	return &Codex{Bin: bin}
}

// Name returns "codex".
func (c *Codex) Name() string { return "codex" }

// Session starts an interactive codex session.
// Matches: codex "$PROMPT"
func (c *Codex) Session(ctx context.Context, systemPrompt string, extraArgs []string) error {
	args := []string{systemPrompt}
	args = append(args, extraArgs...)

	cmd := exec.CommandContext(ctx, c.Bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Remove CLAUDECODE from env so nested invocations work
	// (matches Claude.Session behavior).
	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	return cmd.Run()
}

// Evaluate runs codex in non-interactive mode via `codex exec`.
// The system prompt and user prompt are combined into a single prompt
// since codex doesn't have a separate system prompt concept.
// The tools parameter is accepted for interface compliance but has no
// effect — codex exec always uses --sandbox read-only.
func (c *Codex) Evaluate(ctx context.Context, systemPrompt, userPrompt string, model Model, _ ToolPolicy) (string, error) {
	// Combine system + user prompt (codex has no --system-prompt)
	combinedPrompt := systemPrompt + "\n\n" + userPrompt

	args := []string{
		"exec",
		"--ephemeral",
		"--sandbox", "read-only",
	}
	if model != "" && string(model) != "codex" {
		args = append(args, "-m", string(model))
	}
	// Prompt via stdin (use "-" placeholder if needed, but codex reads
	// stdin when no positional prompt is given)
	args = append(args, "-")

	cmd := exec.CommandContext(ctx, c.Bin, args...)
	cmd.Stdin = strings.NewReader(combinedPrompt)

	if os.Getenv("BONSAI_DEBUG") != "" {
		debugArgs := make([]string, len(args))
		copy(debugArgs, args)
		fmt.Fprintf(os.Stderr, "[bonsai:debug] codex %s\n", strings.Join(debugArgs, " "))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("codex invocation failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}

// Execute runs codex in non-interactive mode but streams stdout/stderr
// to the terminal instead of capturing output. Unlike Evaluate, no
// sandbox flag is passed — codex exec defaults to writable.
func (c *Codex) Execute(ctx context.Context, systemPrompt, userPrompt string, model Model) error {
	args := []string{"exec", "--ephemeral"}
	if model != "" && string(model) != "codex" {
		args = append(args, "-m", string(model))
	}
	args = append(args, "-")

	combined := systemPrompt + "\n\n" + userPrompt

	cmd := exec.CommandContext(ctx, c.Bin, args...)
	cmd.Stdin = strings.NewReader(combined)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
