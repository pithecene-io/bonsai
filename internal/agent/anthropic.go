package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// modelProfile holds per-tier token limits for the Messages API.
type modelProfile struct {
	maxTokens int64
}

// modelProfiles maps tier names to their token limits.
var modelProfiles = map[string]modelProfile{
	"haiku":  {maxTokens: 4096},
	"sonnet": {maxTokens: 8192},
	"opus":   {maxTokens: 8192},
}

// modelAliases maps short tier names to full Anthropic model identifiers.
var modelAliases = map[string]string{
	"haiku":  string(anthropic.ModelClaudeHaiku4_5_20251001),
	"sonnet": string(anthropic.ModelClaudeSonnet4_6),
	"opus":   string(anthropic.ModelClaudeOpus4_6),
}

// AnthropicOption configures the Anthropic backend.
type AnthropicOption func(*anthropicConfig)

type anthropicConfig struct {
	apiKey string
}

// WithAPIKey sets an explicit API key, overriding ANTHROPIC_API_KEY.
func WithAPIKey(key string) AnthropicOption {
	return func(c *anthropicConfig) {
		c.apiKey = key
	}
}

// Anthropic implements Agent via the Anthropic Messages API.
type Anthropic struct {
	client anthropic.Client
}

// NewAnthropic creates an Anthropic backend. Returns nil when no API key
// is available (neither via option nor ANTHROPIC_API_KEY env), enabling
// graceful fallback in the Router.
func NewAnthropic(opts ...AnthropicOption) *Anthropic {
	var cfg anthropicConfig
	for _, o := range opts {
		o(&cfg)
	}

	// Resolve API key: explicit option > environment variable.
	apiKey := cfg.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &Anthropic{client: client}
}

// Name returns "anthropic".
func (a *Anthropic) Name() string { return "anthropic" }

// Interactive returns an error — the direct API backend cannot attach
// to a terminal for interactive sessions.
func (a *Anthropic) Interactive(_ context.Context, _ string, _ []string) error {
	return errors.New("anthropic direct API does not support interactive sessions")
}

// NonInteractive calls the Anthropic Messages API directly.
func (a *Anthropic) NonInteractive(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
	resolvedModel := resolveModel(model)
	profile := profileFor(Model(model).Tier())

	if os.Getenv("BONSAI_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[bonsai:debug] anthropic model=%s resolved=%s maxTokens=%d\n",
			model, resolvedModel, profile.maxTokens)
	}

	msg, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(resolvedModel),
		MaxTokens: profile.maxTokens,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("anthropic API call failed: %w", err)
	}

	return extractText(msg), nil
}

// resolveModel maps a short alias (e.g. "haiku") to the full Anthropic
// model identifier. Returns the input unchanged if no alias matches.
func resolveModel(name string) string {
	low := strings.ToLower(name)
	if full, ok := modelAliases[low]; ok {
		return full
	}
	return name
}

// profileFor returns the token profile for a given tier. Falls back to
// the sonnet profile for unknown tiers.
func profileFor(tier string) modelProfile {
	if p, ok := modelProfiles[tier]; ok {
		return p
	}
	return modelProfiles["sonnet"]
}

// extractText concatenates all text blocks from an Anthropic response.
func extractText(msg *anthropic.Message) string {
	var parts []string
	for _, block := range msg.Content {
		if block.Type == "text" {
			parts = append(parts, block.AsText().Text)
		}
	}
	return strings.Join(parts, "")
}
