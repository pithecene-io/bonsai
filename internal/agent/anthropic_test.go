package agent_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pithecene-io/bonsai/internal/agent"
)

func TestAnthropic_Implements(_ *testing.T) {
	// Compile-time interface check.
	var _ agent.Agent = (*agent.Anthropic)(nil)
}

func TestAnthropic_Name(t *testing.T) {
	// Use a dummy key so NewAnthropic returns non-nil.
	a := agent.NewAnthropic(agent.WithAPIKey("test-key"))
	if a == nil {
		t.Fatal("NewAnthropic returned nil with explicit key")
	}
	if got := a.Name(); got != "anthropic" {
		t.Errorf("Name() = %q, want %q", got, "anthropic")
	}
}

func TestAnthropic_SessionReturnsError(t *testing.T) {
	a := agent.NewAnthropic(agent.WithAPIKey("test-key"))
	if a == nil {
		t.Fatal("NewAnthropic returned nil with explicit key")
	}
	err := a.Session(t.Context(), "sys", nil)
	if err == nil {
		t.Error("Session should return an error for direct API backend")
	}
}

func TestNewAnthropic_NilWithoutKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	// Point HOME to an empty dir so the OAuth credential file lookup
	// also finds nothing — makes the test hermetic regardless of host.
	t.Setenv("HOME", t.TempDir())
	a := agent.NewAnthropic()
	if a != nil {
		t.Error("NewAnthropic should return nil when no credentials are available")
	}
}

func TestNewAnthropic_UsesEnvKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-env-key")
	a := agent.NewAnthropic()
	if a == nil {
		t.Error("NewAnthropic should return non-nil when ANTHROPIC_API_KEY is set")
	}
}

func TestNewAnthropic_ExplicitKeyOverridesEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("HOME", t.TempDir())
	a := agent.NewAnthropic(agent.WithAPIKey("explicit-key"))
	if a == nil {
		t.Error("NewAnthropic should return non-nil with explicit key even when env is empty")
	}
}

func TestNewAnthropic_UsesClaudeOAuthToken(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")

	// Create a fake ~/.claude/.credentials.json
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	claudeDir := filepath.Join(fakeHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	creds := `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-fake-token"}}`
	if err := os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), []byte(creds), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	a := agent.NewAnthropic()
	if a == nil {
		t.Error("NewAnthropic should return non-nil when Claude CLI OAuth token is available")
	}
}

// writeOAuthCredentials creates a fake ~/.claude/.credentials.json under dir.
func writeOAuthCredentials(t *testing.T, dir string) {
	t.Helper()
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	creds := `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-fake-token"}}`
	if err := os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), []byte(creds), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// TestNewAnthropic_CredentialPrecedence verifies the 3-tier credential
// resolution order: explicit key → OAuth → env var.
//
// For the "explicit wins over all" case, both explicit and env keys are
// non-OAuth, so IsOAuth alone cannot distinguish them.  We additionally
// point a WithBaseURL at an httptest server and assert which X-Api-Key
// actually arrives on the wire.
func TestNewAnthropic_CredentialPrecedence(t *testing.T) {
	tests := []struct {
		name       string
		explicit   string
		oauth      bool
		envKey     string
		wantNil    bool
		wantOAuth  bool
		wantAPIKey string // if non-empty, verified via httptest
	}{
		{
			name:       "explicit wins over all",
			explicit:   "sk-explicit",
			oauth:      true,
			envKey:     "sk-env",
			wantOAuth:  false,
			wantAPIKey: "sk-explicit",
		},
		{
			name:      "OAuth wins over env",
			oauth:     true,
			envKey:    "sk-env",
			wantOAuth: true,
		},
		{
			name:       "env used when alone",
			envKey:     "sk-env",
			wantOAuth:  false,
			wantAPIKey: "sk-env",
		},
		{
			name:    "nil when nothing",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeHome := t.TempDir()
			t.Setenv("HOME", fakeHome)

			if tt.oauth {
				writeOAuthCredentials(t, fakeHome)
			}
			if tt.envKey != "" {
				t.Setenv("ANTHROPIC_API_KEY", tt.envKey)
			} else {
				t.Setenv("ANTHROPIC_API_KEY", "")
			}

			// Capture the key that actually hits the wire.
			var gotKey string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotKey = r.Header.Get("X-Api-Key")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(anthropicStubResponse()))
			}))
			defer srv.Close()

			var opts []agent.AnthropicOption
			opts = append(opts, agent.WithBaseURL(srv.URL))
			if tt.explicit != "" {
				opts = append(opts, agent.WithAPIKey(tt.explicit))
			}

			a := agent.NewAnthropic(opts...)
			if tt.wantNil {
				if a != nil {
					t.Fatal("expected nil, got non-nil")
				}
				return
			}
			if a == nil {
				t.Fatal("expected non-nil, got nil")
			}
			if got := a.IsOAuth(); got != tt.wantOAuth {
				t.Errorf("IsOAuth() = %v, want %v", got, tt.wantOAuth)
			}

			// When wantAPIKey is set, make a real request and verify
			// the key that arrives on the wire.
			if tt.wantAPIKey != "" {
				_, err := a.Evaluate(t.Context(), "sys", "user", agent.Model("haiku"))
				if err != nil {
					t.Fatalf("Evaluate: %v", err)
				}
				if gotKey != tt.wantAPIKey {
					t.Errorf("X-Api-Key on wire = %q, want %q", gotKey, tt.wantAPIKey)
				}
			}
		})
	}
}

// anthropicStubResponse returns a minimal valid Messages API JSON response.
func anthropicStubResponse() string {
	return `{
		"id": "msg_test",
		"type": "message",
		"role": "assistant",
		"model": "claude-haiku-4-5-20251001",
		"content": [{"type": "text", "text": "ok"}],
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 10, "output_tokens": 1}
	}`
}

// TestAnthropic_RequestShape_APIKey verifies the HTTP request shape when
// using an explicit API key (non-OAuth path).
func TestAnthropic_RequestShape_APIKey(t *testing.T) {
	var captured *http.Request
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Clone(r.Context())
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(anthropicStubResponse()))
	}))
	defer srv.Close()

	// Isolate from host credentials.
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("HOME", t.TempDir())

	a := agent.NewAnthropic(
		agent.WithAPIKey("sk-test-key-123"),
		agent.WithBaseURL(srv.URL),
	)
	if a == nil {
		t.Fatal("expected non-nil Anthropic")
	}
	if a.IsOAuth() {
		t.Fatal("expected non-OAuth path")
	}

	_, err := a.Evaluate(t.Context(), "test-system", "test-user", agent.Model("haiku"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if captured == nil {
		t.Fatal("server never received a request")
	}

	// Path must be /v1/messages.
	if captured.URL.Path != "/v1/messages" {
		t.Errorf("path = %q, want /v1/messages", captured.URL.Path)
	}

	// X-Api-Key must be present.
	if got := captured.Header.Get("X-Api-Key"); got != "sk-test-key-123" {
		t.Errorf("X-Api-Key = %q, want sk-test-key-123", got)
	}

	// No OAuth-specific headers.
	if got := captured.Header.Get("Authorization"); got != "" {
		t.Errorf("Authorization should be absent for API-key path, got %q", got)
	}
	if got := captured.Header.Get("anthropic-dangerous-direct-browser-access"); got != "" {
		t.Errorf("anthropic-dangerous-direct-browser-access should be absent, got %q", got)
	}

	// No ?beta=true query param.
	if captured.URL.Query().Get("beta") != "" {
		t.Errorf("?beta query param should be absent for API-key path")
	}

	// Body: verify model resolves correctly.
	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if model, ok := body["model"].(string); !ok || !strings.Contains(model, "haiku") {
		t.Errorf("body model = %v, want haiku variant", body["model"])
	}
}

// TestAnthropic_RequestShape_OAuth verifies the HTTP request shape when
// using OAuth credentials (Claude Code billing path).
func TestAnthropic_RequestShape_OAuth(t *testing.T) {
	var captured *http.Request
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Clone(r.Context())
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(anthropicStubResponse()))
	}))
	defer srv.Close()

	// Set up OAuth credentials.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv("ANTHROPIC_API_KEY", "")
	writeOAuthCredentials(t, fakeHome)

	a := agent.NewAnthropic(agent.WithBaseURL(srv.URL))
	if a == nil {
		t.Fatal("expected non-nil Anthropic with OAuth credentials")
	}
	if !a.IsOAuth() {
		t.Fatal("expected OAuth path")
	}

	_, err := a.Evaluate(t.Context(), "test-system", "test-user", agent.Model("haiku"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	if captured == nil {
		t.Fatal("server never received a request")
	}

	// Authorization: Bearer must be present.
	auth := captured.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		t.Errorf("Authorization = %q, want Bearer prefix", auth)
	}

	// X-Api-Key must not carry a real key — this is the billing-critical
	// contract from option.WithAPIKey("").  The SDK sends the header with
	// an empty value (present but empty); what matters is that no actual
	// API key leaks through, which would cause the API to check that
	// account's credit balance instead of routing to the Max/Pro
	// subscription.  Use the map form to inspect every value.
	if vals, exists := captured.Header["X-Api-Key"]; exists {
		for _, v := range vals {
			if v != "" {
				t.Errorf("X-Api-Key should be empty on OAuth path, got %q", v)
			}
		}
	}

	// OAuth-specific headers.
	if got := captured.Header.Get("anthropic-beta"); !strings.Contains(got, "oauth-2025-04-20") {
		t.Errorf("anthropic-beta = %q, want to contain oauth-2025-04-20", got)
	}
	if got := captured.Header.Get("User-Agent"); !strings.Contains(got, "claude-cli") {
		t.Errorf("User-Agent = %q, want to contain claude-cli", got)
	}
	if got := captured.Header.Get("x-app"); got != "cli" {
		t.Errorf("x-app = %q, want cli", got)
	}
	if got := captured.Header.Get("anthropic-dangerous-direct-browser-access"); got != "true" {
		t.Errorf("anthropic-dangerous-direct-browser-access = %q, want true", got)
	}

	// ?beta=true query param.
	if captured.URL.Query().Get("beta") != "true" {
		t.Errorf("?beta query param = %q, want true", captured.URL.Query().Get("beta"))
	}

	// Body: system prompt array starts with Claude Code prefix.
	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	system, ok := body["system"].([]any)
	if !ok || len(system) < 2 {
		t.Fatalf("system = %v, want array with ≥2 blocks", body["system"])
	}
	firstBlock, _ := system[0].(map[string]any)
	if text, _ := firstBlock["text"].(string); !strings.Contains(text, "Claude Code") {
		t.Errorf("first system block = %q, want Claude Code prefix", text)
	}

	// Metadata: user_id = bonsai.
	metadata, _ := body["metadata"].(map[string]any)
	if uid, _ := metadata["user_id"].(string); uid != "bonsai" {
		t.Errorf("metadata.user_id = %q, want bonsai", uid)
	}
}
