//go:build integration

package agent_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/prompt"
	"github.com/pithecene-io/bonsai/internal/skill"
)

// benchmarkModel runs a single cheap skill invocation against the given
// model and reports wall time. Requires the claude/codex CLIs to be
// available.
//
// Run with:
//
//	go test -tags integration -run '^$' -bench BenchmarkModel -benchtime 1x ./internal/agent/
//
// Or for a specific model:
//
//	go test -tags integration -run '^$' -bench BenchmarkModel/haiku -benchtime 1x ./internal/agent/
func BenchmarkModel(b *testing.B) {
	repoRoot := detectRepoRoot(b)
	cfg := config.Default()
	resolver := assets.NewResolver(repoRoot)

	// Load a cheap skill for the benchmark
	def, err := skill.Load(resolver, "repo-convention-enforcer", "v1")
	if err != nil {
		b.Fatalf("load skill: %v", err)
	}

	builder := prompt.NewBuilder(resolver, repoRoot)
	systemPrompt, err := builder.BuildValidator(prompt.ValidatorOpts{
		SkillBody:    def.Body,
		OutputSchema: def.OutputSchema,
	})
	if err != nil {
		b.Fatalf("build prompt: %v", err)
	}

	userPrompt := "Evaluate the following repository.\n\nRepository tree:\ncmd/bonsai/main.go\ninternal/cli/app.go\ninternal/agent/agent.go\ngo.mod\nCLAUDE.md\nAGENTS.md\n\nRespond with JSON only. No other text."

	b.Logf("system prompt: %d chars (~%d tokens)", len(systemPrompt), len(systemPrompt)/4)
	b.Logf("user prompt: %d chars (~%d tokens)", len(userPrompt), len(userPrompt)/4)

	models := []struct {
		name   string
		agent  agent.Agent
		model  string
		budget time.Duration // per-model latency budget
	}{
		{"codex", agent.NewCodex(cfg.Agents.Codex.Bin), "codex", 30 * time.Second},
		{"sonnet", agent.NewClaude(cfg.Agents.Claude.Bin), "sonnet", 60 * time.Second},
		{"opus", agent.NewClaude(cfg.Agents.Claude.Bin), "opus", 90 * time.Second},
		{"haiku-cli", agent.NewClaude(cfg.Agents.Claude.Bin), "haiku", 60 * time.Second},
	}

	// Add haiku-direct entry when an API key is available.
	if a := agent.NewAnthropic(); a != nil {
		models = append(models, struct {
			name   string
			agent  agent.Agent
			model  string
			budget time.Duration
		}{"haiku-direct", a, "haiku", 5 * time.Second})
	}

	for _, m := range models {
		b.Run(m.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
				start := time.Now()
				out, err := m.agent.NonInteractive(ctx, systemPrompt, userPrompt, m.model)
				elapsed := time.Since(start)
				cancel()

				if err != nil {
					b.Logf("[%s] ERROR after %v: %v", m.name, elapsed, err)
					b.SkipNow()
				}

				b.Logf("[%s] %v — %d chars output", m.name, elapsed, len(out))

				if elapsed > m.budget {
					b.Errorf("[%s] EXCEEDED %v budget: %v", m.name, m.budget, elapsed)
				}
			}
		})
	}
}

// TestModelLatency is a non-benchmark test that runs each model once
// and reports timing. Use this for quick verification:
//
//	go test -tags integration -run TestModelLatency -v -timeout 300s ./internal/agent/
func TestModelLatency(t *testing.T) {
	repoRoot := detectRepoRoot(t)
	cfg := config.Default()
	resolver := assets.NewResolver(repoRoot)

	def, err := skill.Load(resolver, "repo-convention-enforcer", "v1")
	if err != nil {
		t.Fatalf("load skill: %v", err)
	}

	builder := prompt.NewBuilder(resolver, repoRoot)
	systemPrompt, err := builder.BuildValidator(prompt.ValidatorOpts{
		SkillBody:    def.Body,
		OutputSchema: def.OutputSchema,
	})
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}

	userPrompt := "Evaluate the following repository.\n\nRepository tree:\ncmd/bonsai/main.go\ninternal/cli/app.go\ninternal/agent/agent.go\ngo.mod\nCLAUDE.md\nAGENTS.md\n\nRespond with JSON only. No other text."

	t.Logf("system prompt: %d chars (~%d tokens)", len(systemPrompt), len(systemPrompt)/4)

	type result struct {
		model   string
		elapsed time.Duration
		budget  time.Duration
		output  string
		err     error
	}

	models := []struct {
		name   string
		agent  agent.Agent
		model  string
		budget time.Duration
	}{
		{"codex", agent.NewCodex(cfg.Agents.Codex.Bin), "codex", 30 * time.Second},
		{"sonnet", agent.NewClaude(cfg.Agents.Claude.Bin), "sonnet", 60 * time.Second},
		{"haiku-cli", agent.NewClaude(cfg.Agents.Claude.Bin), "haiku", 60 * time.Second},
	}

	// Add haiku-direct entry when an API key is available.
	if a := agent.NewAnthropic(); a != nil {
		models = append(models, struct {
			name   string
			agent  agent.Agent
			model  string
			budget time.Duration
		}{"haiku-direct", a, "haiku", 5 * time.Second})
	}

	var results []result
	for _, m := range models {
		t.Run(m.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			start := time.Now()
			out, err := m.agent.NonInteractive(ctx, systemPrompt, userPrompt, m.model)
			elapsed := time.Since(start)

			r := result{model: m.name, elapsed: elapsed, budget: m.budget, output: out, err: err}
			results = append(results, r)

			if err != nil {
				t.Logf("[%s] ERROR: %v (after %v)", m.name, err, elapsed)
				t.SkipNow()
			}

			t.Logf("[%s] %v — %d chars output", m.name, elapsed, len(out))

			// Soft latency target — API/network variance makes hard
			// assertions unreliable. Log for SLO tracking.
			if elapsed > m.budget {
				t.Logf("[%s] WARNING: exceeded %v target: %v", m.name, m.budget, elapsed)
			}
		})
	}

	// Print summary table
	t.Log("")
	t.Log("╔═══════════╦════════════╦════════╗")
	t.Log("║ Model     ║ Wall Time  ║ Status ║")
	t.Log("╠═══════════╬════════════╬════════╣")
	for _, r := range results {
		status := "✔ PASS"
		if r.err != nil {
			status = "✖ ERR "
		} else if r.elapsed > r.budget {
			status = "✖ SLOW"
		}
		t.Logf("║ %-9s ║ %9v ║ %s ║", r.model, r.elapsed.Round(time.Millisecond), status)
	}
	t.Log("╚═══════════╩════════════╩════════╝")
}

// TestAnthropic_NonInteractive_Haiku verifies the direct API backend
// can complete a cheap skill evaluation within the 5s latency budget.
//
// Run with:
//
//	go test -tags integration -run TestAnthropic_NonInteractive_Haiku -v -timeout 30s ./internal/agent/
func TestAnthropic_NonInteractive_Haiku(t *testing.T) {
	a := agent.NewAnthropic()
	if a == nil {
		t.Skip("ANTHROPIC_API_KEY not set — skipping direct API test")
	}

	repoRoot := detectRepoRoot(t)
	resolver := assets.NewResolver(repoRoot)

	def, err := skill.Load(resolver, "repo-convention-enforcer", "v1")
	if err != nil {
		t.Fatalf("load skill: %v", err)
	}

	builder := prompt.NewBuilder(resolver, repoRoot)
	systemPrompt, err := builder.BuildValidator(prompt.ValidatorOpts{
		SkillBody:    def.Body,
		OutputSchema: def.OutputSchema,
	})
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}

	userPrompt := "Evaluate the following repository.\n\nRepository tree:\ncmd/bonsai/main.go\ninternal/cli/app.go\ngo.mod\nCLAUDE.md\n\nRespond with JSON only. No other text."

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	out, err := a.NonInteractive(ctx, systemPrompt, userPrompt, "haiku")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("NonInteractive: %v (after %v)", err, elapsed)
	}

	t.Logf("[haiku-direct] %v — %d chars output", elapsed, len(out))

	if elapsed > 5*time.Second {
		t.Errorf("[haiku-direct] EXCEEDED 5s budget: %v", elapsed)
	}
}

// detectRepoRoot finds the bonsai repo root for benchmark tests.
func detectRepoRoot(tb testing.TB) string {
	tb.Helper()
	// Try common locations
	candidates := []string{
		".", "../..", "../../..",
	}
	for _, c := range candidates {
		if _, err := os.Stat(fmt.Sprintf("%s/CLAUDE.md", c)); err == nil {
			if _, err := os.Stat(fmt.Sprintf("%s/go.mod", c)); err == nil {
				return c
			}
		}
	}
	tb.Skip("could not detect bonsai repo root — run from repo directory")
	return ""
}
