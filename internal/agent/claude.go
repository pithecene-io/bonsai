package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Claude implements the Agent interface by shelling out to the claude CLI.
type Claude struct {
	// Bin is the path to the claude binary. Defaults to "claude".
	Bin string
}

// NewClaude creates a Claude agent with the given binary path.
func NewClaude(bin string) *Claude {
	if bin == "" {
		bin = "claude"
	}
	return &Claude{Bin: bin}
}

// Name returns "claude".
func (c *Claude) Name() string { return "claude" }

// Interactive starts an interactive claude session.
// Matches: claude --system-prompt "$SYSTEM_PROMPT" "$@"
func (c *Claude) Interactive(ctx context.Context, systemPrompt string, extraArgs []string) error {
	args := []string{"--system-prompt", systemPrompt}
	args = append(args, extraArgs...)

	cmd := exec.CommandContext(ctx, c.Bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// NonInteractive runs claude in non-interactive mode.
// Matches: echo "$USER_PROMPT" | CLAUDECODE= claude -p \
//
//	--system-prompt "$SYSTEM_PROMPT" \
//	--tools "" \
//	--disable-slash-commands \
//	--no-session-persistence \
//	--output-format text
func (c *Claude) NonInteractive(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	args := []string{
		"-p",
		"--system-prompt", systemPrompt,
		"--tools", "",
		"--disable-slash-commands",
		"--no-session-persistence",
		"--output-format", "text",
	}

	cmd := exec.CommandContext(ctx, c.Bin, args...)
	cmd.Stdin = strings.NewReader(userPrompt)
	cmd.Env = append(os.Environ(), "CLAUDECODE=")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude invocation failed: %w\nstderr: %s", err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}
