package skill

import (
	"context"
	"fmt"
	"strings"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/prompt"
)

// RunOpts holds options for running a skill.
type RunOpts struct {
	RepoTree    string // Repository tree listing
	DiffPayload string // Diff content (from --base)
	BaseRef     string // Base ref for diff context
}

// Runner invokes skills via an AI agent.
type Runner struct {
	agent   agent.Agent
	builder *prompt.Builder
}

// NewRunner creates a skill runner.
func NewRunner(a agent.Agent, b *prompt.Builder) *Runner {
	return &Runner{agent: a, builder: b}
}

// Run invokes a skill and returns validated output.
func (r *Runner) Run(ctx context.Context, def *Definition, opts RunOpts) (*Output, error) {
	// Build system prompt (validator pattern)
	systemPrompt, err := r.builder.BuildValidator(prompt.ValidatorOpts{
		SkillBody:    def.Body,
		OutputSchema: def.OutputSchema,
	})
	if err != nil {
		return nil, fmt.Errorf("build system prompt: %w", err)
	}

	// Build user prompt
	userPrompt := buildUserPrompt(opts)

	// Invoke agent non-interactively
	response, err := r.agent.NonInteractive(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("agent invocation: %w", err)
	}

	// Validate response
	output, err := ParseOutput(response)
	if err != nil {
		return nil, fmt.Errorf("validate output: %w", err)
	}

	return output, nil
}

// buildUserPrompt constructs the user prompt matching ai-skill.sh behavior.
func buildUserPrompt(opts RunOpts) string {
	var parts []string

	parts = append(parts, "Evaluate the following repository.")
	parts = append(parts, "")
	parts = append(parts, "Repository tree:")
	parts = append(parts, opts.RepoTree)

	if opts.DiffPayload != "" {
		parts = append(parts, "")
		parts = append(parts, fmt.Sprintf("Diff (base: %s):", opts.BaseRef))
		parts = append(parts, opts.DiffPayload)
	}

	parts = append(parts, "")
	parts = append(parts, "Respond with JSON only. No other text.")

	return strings.Join(parts, "\n")
}
