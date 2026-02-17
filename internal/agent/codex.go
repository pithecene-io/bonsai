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

// Interactive starts an interactive codex session.
// Matches: codex "$PROMPT" (prompt as first positional argument)
func (c *Codex) Interactive(ctx context.Context, systemPrompt string, extraArgs []string) error {
	args := []string{systemPrompt}
	args = append(args, extraArgs...)

	cmd := exec.CommandContext(ctx, c.Bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// NonInteractive runs codex non-interactively. Codex uses the same
// flag conventions as claude for non-interactive mode.
func (c *Codex) NonInteractive(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	args := []string{
		"-p",
		"--system-prompt", systemPrompt,
	}

	cmd := exec.CommandContext(ctx, c.Bin, args...)
	cmd.Stdin = strings.NewReader(userPrompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("codex invocation failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}
