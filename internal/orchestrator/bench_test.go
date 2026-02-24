//go:build integration

package orchestrator_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/orchestrator"
	"github.com/pithecene-io/bonsai/internal/registry"
)

// TestCheckLatency_SingleSkill runs a single cheap skill through the full
// orchestrator stack (config → model routing → agent router → subprocess)
// for each model and reports wall times.
//
// Run with:
//
//	go test -tags integration -run TestCheckLatency_SingleSkill -v -timeout 300s ./internal/orchestrator/
func TestCheckLatency_SingleSkill(t *testing.T) {
	repoRoot := detectRepoRoot(t)
	cfg, err := config.Load(repoRoot)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	resolver := assets.NewResolver(repoRoot)
	resolver.ExtraSkillDirs = cfg.Skills.ExtraDirs

	reg, err := registry.Load(resolver)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}

	// Find a cheap skill from the default bundle
	defaultSkills, err := reg.SkillsForBundle("default")
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}

	var cheapSkill registry.Skill
	for _, s := range defaultSkills {
		if s.Cost == "cheap" {
			cheapSkill = s
			break
		}
	}
	if cheapSkill.Name == "" {
		t.Fatal("no cheap skill found in default bundle")
	}
	t.Logf("Using skill: %s (cost: %s)", cheapSkill.Name, cheapSkill.Cost)

	models := []string{"haiku", "sonnet", "codex"}

	type result struct {
		model   string
		elapsed time.Duration
		status  string
		err     error
	}
	var results []result

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			router := agent.NewRouter(cfg.Agents.Claude.Bin, cfg.Agents.Codex.Bin)
			orch := orchestrator.New(router, resolver)

			// Override requires_diff so skill runs without --base
			skill := cheapSkill
			f := false
			skill.RequiresDiff = &f

			opts := orchestrator.RunOpts{
				Skills:              []registry.Skill{skill},
				Source:              "bench:" + model,
				RepoRoot:            repoRoot,
				Config:              cfg,
				DefaultRequiresDiff: false,
				Concurrency:         1,
				ModelOverride:       model,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			start := time.Now()
			report, err := orch.Run(ctx, opts, nil)
			elapsed := time.Since(start)

			r := result{model: model, elapsed: elapsed}

			if err != nil {
				r.err = err
				r.status = "ERROR"
				t.Logf("[%s] ERROR: %v (after %v)", model, err, elapsed)
			} else if report.Passed == 1 {
				r.status = "pass"
				t.Logf("[%s] %v — passed", model, elapsed)
			} else if report.Failed == 1 {
				r.status = "fail"
				t.Logf("[%s] %v — failed (skill reported findings)", model, elapsed)
			} else {
				r.status = fmt.Sprintf("p:%d f:%d s:%d", report.Passed, report.Failed, report.Skipped)
				t.Logf("[%s] %v — %s", model, elapsed, r.status)
			}

			results = append(results, r)

			// Haiku cheap skills MUST be under 10s
			if model == "haiku" && elapsed > 10*time.Second {
				t.Errorf("[haiku] EXCEEDED 10s budget: %v", elapsed)
			}
		})
	}

	// Summary
	t.Log("")
	t.Logf("╔═══════════╦════════════╦════════╗")
	t.Logf("║ Model     ║ Wall Time  ║ Status ║")
	t.Logf("╠═══════════╬════════════╬════════╣")
	for _, r := range results {
		budgetStatus := "✔"
		if r.model == "haiku" && r.elapsed > 10*time.Second {
			budgetStatus = "✖"
		}
		if r.err != nil {
			budgetStatus = "✖"
		}
		t.Logf("║ %-9s ║ %9v ║ %s %-4s ║",
			r.model, r.elapsed.Round(time.Millisecond), budgetStatus, r.status)
	}
	t.Logf("╚═══════════╩════════════╩════════╝")
}

// TestCheckLatency_ParallelBundle runs the full default bundle with
// parallelism and reports total wall time.
//
// Run with:
//
//	go test -tags integration -run TestCheckLatency_ParallelBundle -v -timeout 600s ./internal/orchestrator/
func TestCheckLatency_ParallelBundle(t *testing.T) {
	repoRoot := detectRepoRoot(t)
	cfg, err := config.Load(repoRoot)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	resolver := assets.NewResolver(repoRoot)
	resolver.ExtraSkillDirs = cfg.Skills.ExtraDirs

	reg, err := registry.Load(resolver)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}

	skills, err := reg.SkillsForBundle("default")
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}

	// Count by cost tier
	costs := map[string]int{}
	for _, s := range skills {
		costs[s.Cost]++
	}
	t.Logf("Bundle: %d skills (cheap:%d moderate:%d heavy:%d)",
		len(skills), costs["cheap"], costs["moderate"], costs["heavy"])

	router := agent.NewRouter(cfg.Agents.Claude.Bin, cfg.Agents.Codex.Bin)
	orch := orchestrator.New(router, resolver)

	opts := orchestrator.RunOpts{
		Skills:              skills,
		Source:              "bench:default-bundle",
		RepoRoot:            repoRoot,
		Config:              cfg,
		DefaultRequiresDiff: reg.Defaults.EffectiveRequiresDiff(),
		Concurrency:         cfg.Check.Concurrency,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	start := time.Now()
	report, err := orch.Run(ctx, opts, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run: %v (after %v)", err, elapsed)
	}

	t.Logf("")
	t.Logf("══════════════════════════════════════════")
	t.Logf("Total wall time:  %v", elapsed.Round(time.Millisecond))
	t.Logf("Concurrency:      %d", cfg.Check.Concurrency)
	t.Logf("Passed:           %d/%d", report.Passed, report.Total)
	t.Logf("Failed:           %d", report.Failed)
	t.Logf("Skipped:          %d", report.Skipped)
	t.Logf("══════════════════════════════════════════")

	// Per-skill timing
	t.Logf("")
	t.Logf("%-45s %10s %s", "Skill", "Time", "Status")
	t.Logf("%-45s %10s %s", strings.Repeat("─", 45), strings.Repeat("─", 10), strings.Repeat("─", 8))
	for _, r := range report.Results {
		ms := time.Duration(r.Elapsed) * time.Millisecond
		t.Logf("%-45s %9v  %s", r.Name, ms.Round(time.Millisecond), r.Status)
	}
}

func detectRepoRoot(tb testing.TB) string {
	tb.Helper()
	candidates := []string{".", "../..", "../../.."}
	for _, c := range candidates {
		if _, err := os.Stat(fmt.Sprintf("%s/CLAUDE.md", c)); err == nil {
			if _, err := os.Stat(fmt.Sprintf("%s/go.mod", c)); err == nil {
				return c
			}
		}
	}
	tb.Skip("could not detect bonsai repo root")
	return ""
}
