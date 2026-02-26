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
// When model is non-empty, passes --model to the claude CLI.
func (c *Claude) Interactive(ctx context.Context, systemPrompt string, extraArgs []string) error {
	args := []string{"--system-prompt", systemPrompt}
	args = append(args, extraArgs...)

	cmd := exec.CommandContext(ctx, c.Bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// NonInteractive runs claude in non-interactive (print) mode.
// The --model flag is placed early in the args to ensure the CLI
// parses it before processing the system prompt.
func (c *Claude) NonInteractive(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
	var args []string

	// --model MUST come before other flags to ensure correct parsing
	if model != "" {
		args = append(args, "--model", model)
	}

	args = append(args,
		"-p",
		"--system-prompt", systemPrompt,
		"--tools", "",
		"--disable-slash-commands",
		"--no-session-persistence",
		"--output-format", "text",
	)

	// Use low effort for haiku to minimize latency on cheap evaluation.
	if Model(model).IsHaiku() {
		args = append(args, "--effort", "low")
	}

	cmd := exec.CommandContext(ctx, c.Bin, args...)
	cmd.Stdin = strings.NewReader(userPrompt)

	// Remove CLAUDECODE from env so nested invocations work.
	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	if os.Getenv("BONSAI_DEBUG") != "" {
		debugArgs := make([]string, len(args))
		copy(debugArgs, args)
		for i, a := range debugArgs {
			if a == "--system-prompt" && i+1 < len(debugArgs) {
				debugArgs[i+1] = fmt.Sprintf("[%d chars]", len(debugArgs[i+1]))
			}
		}
		fmt.Fprintf(os.Stderr, "[bonsai:debug] claude %s\n", strings.Join(debugArgs, " "))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude invocation failed: %w\nstderr: %s", err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}

// Autonomous runs claude in print mode with tools enabled.
// Unlike NonInteractive, tools are not disabled — the model can
// autonomously edit files and run commands. Output streams directly
// to stdout/stderr rather than being captured.
func (c *Claude) Autonomous(ctx context.Context, systemPrompt, userPrompt, model string) error {
	var args []string

	if model != "" {
		args = append(args, "--model", model)
	}

	args = append(args,
		"-p",
		"--system-prompt", systemPrompt,
		"--no-session-persistence",
		"--output-format", "text",
	)
	// Tools remain enabled — no --tools "" flag.

	cmd := exec.CommandContext(ctx, c.Bin, args...)
	cmd.Stdin = strings.NewReader(userPrompt)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	if os.Getenv("BONSAI_DEBUG") != "" {
		debugArgs := make([]string, len(args))
		copy(debugArgs, args)
		for i, a := range debugArgs {
			if a == "--system-prompt" && i+1 < len(debugArgs) {
				debugArgs[i+1] = fmt.Sprintf("[%d chars]", len(debugArgs[i+1]))
			}
		}
		fmt.Fprintf(os.Stderr, "[bonsai:debug] claude autonomous %s\n", strings.Join(debugArgs, " "))
	}

	return cmd.Run()
}

// filterEnv returns a copy of environ with the named variable removed.
func filterEnv(environ []string, name string) []string {
	prefix := name + "="
	out := make([]string, 0, len(environ))
	for _, e := range environ {
		if !strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	return out
}
