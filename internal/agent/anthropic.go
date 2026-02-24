package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// claudeCodeSystemPrefix is the system prompt prefix required by the
// Anthropic API when authenticating with a Claude CLI OAuth token.
const claudeCodeSystemPrefix = "You are Claude Code, Anthropic's official CLI for Claude."

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
	oauth  bool // true when using Claude CLI OAuth token
}

// NewAnthropic creates an Anthropic backend. Returns nil when no
// credentials are available, enabling graceful fallback in the Router.
//
// Credential resolution order:
//  1. Explicit API key (WithAPIKey option)
//  2. Claude CLI OAuth token (~/.claude/.credentials.json)
//  3. ANTHROPIC_API_KEY environment variable
//
// OAuth is preferred over the env var because Max/Pro subscribers
// get zero-overhead billing through their existing subscription,
// while ANTHROPIC_API_KEY requires separate prepaid credits.
//
// API keys use x-api-key header (billed to API credits).
// OAuth tokens use the Claude Code request shape (billed to Max/Pro).
func NewAnthropic(opts ...AnthropicOption) *Anthropic {
	var cfg anthropicConfig
	for _, o := range opts {
		o(&cfg)
	}

	// 1. Explicit API key (option).
	if cfg.apiKey != "" {
		client := anthropic.NewClient(option.WithAPIKey(cfg.apiKey))
		return &Anthropic{client: client}
	}

	// 2. Claude CLI OAuth token — match the Claude Code request shape
	//    so the API routes billing to the Max/Pro subscription.
	if token := readClaudeOAuthToken(); token != "" {
		client := anthropic.NewClient(
			option.WithAuthToken(token),
			// Suppress the env-based X-Api-Key header. The SDK reads
			// ANTHROPIC_API_KEY from the environment and sends it alongside
			// the Bearer token. The API sees the X-Api-Key first, checks
			// that account's credit balance, and rejects with "credit
			// balance is too low" — even though the Bearer token is valid
			// for Max/Pro subscription billing.
			// See: https://github.com/anthropics/claude-code/issues/18340
			option.WithAPIKey(""),
			option.WithHeader("anthropic-beta", "oauth-2025-04-20,interleaved-thinking-2025-05-14"),
			option.WithHeader("User-Agent", "claude-cli/2.1.52 (external, cli)"),
			option.WithHeader("x-app", "cli"),
			option.WithHeader("anthropic-dangerous-direct-browser-access", "true"),
		)
		return &Anthropic{client: client, oauth: true}
	}

	// 3. ANTHROPIC_API_KEY environment variable (billed to API credits).
	if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" {
		client := anthropic.NewClient(option.WithAPIKey(envKey))
		return &Anthropic{client: client}
	}

	return nil
}

// cliCredentials matches the relevant subset of ~/.claude/.credentials.json.
type cliCredentials struct {
	ClaudeAiOauth struct {
		AccessToken string `json:"accessToken"`
	} `json:"claudeAiOauth"`
}

// readClaudeOAuthToken reads the OAuth access token from the Claude CLI
// credentials file. Returns empty string on any error.
func readClaudeOAuthToken() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude", ".credentials.json"))
	if err != nil {
		return ""
	}
	var creds cliCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return ""
	}
	return creds.ClaudeAiOauth.AccessToken
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
		fmt.Fprintf(os.Stderr, "[bonsai:debug] anthropic model=%s resolved=%s maxTokens=%d oauth=%v\n",
			model, resolvedModel, profile.maxTokens, a.oauth)
	}

	// Build system prompt blocks. OAuth path requires the Claude Code
	// prefix as the first system block for billing validation.
	system := []anthropic.TextBlockParam{{Text: systemPrompt}}
	if a.oauth {
		system = []anthropic.TextBlockParam{
			{Text: claudeCodeSystemPrefix},
			{Text: systemPrompt},
		}
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(resolvedModel),
		MaxTokens: profile.maxTokens,
		System:    system,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	}

	// OAuth path: add metadata and ?beta=true query param.
	var reqOpts []option.RequestOption
	if a.oauth {
		params.Metadata = anthropic.MetadataParam{
			UserID: anthropic.String("bonsai"),
		}
		reqOpts = append(reqOpts, option.WithQuery("beta", "true"))
	}

	msg, err := a.client.Messages.New(ctx, params, reqOpts...)
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
