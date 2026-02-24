package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pithecene-io/bonsai/internal/config"
)

func TestDefault(t *testing.T) {
	cfg := config.Default()

	if cfg.Diff.HeavyDiffLines != 500 {
		t.Errorf("HeavyDiffLines = %d, want 500", cfg.Diff.HeavyDiffLines)
	}
	if cfg.Diff.HeavyFilesChanged != 15 {
		t.Errorf("HeavyFilesChanged = %d, want 15", cfg.Diff.HeavyFilesChanged)
	}
	if cfg.Diff.PatchMaxFiles != 3 {
		t.Errorf("PatchMaxFiles = %d, want 3", cfg.Diff.PatchMaxFiles)
	}
	if cfg.Gate.MaxIterations != 3 {
		t.Errorf("MaxIterations = %d, want 3", cfg.Gate.MaxIterations)
	}
	if cfg.Agents.Claude.Bin != "claude" {
		t.Errorf("Claude.Bin = %q, want claude", cfg.Agents.Claude.Bin)
	}
	if cfg.Agents.Codex.Bin != "codex" {
		t.Errorf("Codex.Bin = %q, want codex", cfg.Agents.Codex.Bin)
	}
	if cfg.Output.Dir != "ai/out" {
		t.Errorf("Output.Dir = %q, want ai/out", cfg.Output.Dir)
	}
}

func TestLoadNoFiles(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Should return defaults
	if cfg.Diff.HeavyDiffLines != 500 {
		t.Errorf("HeavyDiffLines = %d, want 500", cfg.Diff.HeavyDiffLines)
	}
}

func TestLoadRepoConfig(t *testing.T) {
	dir := t.TempDir()
	repoConfig := filepath.Join(dir, ".bonsai.yaml")
	if err := os.WriteFile(repoConfig, []byte("diff:\n  heavy_diff_lines: 1000\ngate:\n  max_iterations: 5\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Diff.HeavyDiffLines != 1000 {
		t.Errorf("HeavyDiffLines = %d, want 1000", cfg.Diff.HeavyDiffLines)
	}
	if cfg.Gate.MaxIterations != 5 {
		t.Errorf("MaxIterations = %d, want 5", cfg.Gate.MaxIterations)
	}
	// Other values should remain defaults
	if cfg.Diff.HeavyFilesChanged != 15 {
		t.Errorf("HeavyFilesChanged = %d, want 15 (default)", cfg.Diff.HeavyFilesChanged)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	t.Setenv("BONSAI_DIFF_HEAVY_LINES", "2000")
	t.Setenv("BONSAI_CLAUDE_BIN", "/custom/claude")
	t.Setenv("BONSAI_GATE_MAX_ITERATIONS", "7")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Diff.HeavyDiffLines != 2000 {
		t.Errorf("HeavyDiffLines = %d, want 2000", cfg.Diff.HeavyDiffLines)
	}
	if cfg.Agents.Claude.Bin != "/custom/claude" {
		t.Errorf("Claude.Bin = %q, want /custom/claude", cfg.Agents.Claude.Bin)
	}
	if cfg.Gate.MaxIterations != 7 {
		t.Errorf("MaxIterations = %d, want 7", cfg.Gate.MaxIterations)
	}
}

func TestLoadRepoConfig_SliceReplacesDefaults(t *testing.T) {
	dir := t.TempDir()
	repoConfig := filepath.Join(dir, ".bonsai.yaml")
	yaml := "routing:\n  public_surface_globs:\n    - \"api/\"\n"
	if err := os.WriteFile(repoConfig, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Slice override is a full replacement, not an extension.
	// Setting public_surface_globs to ["api/"] must discard the defaults
	// (sdk/, public/, cmd/, cli/).
	if len(cfg.Routing.PublicSurfaceGlobs) != 1 {
		t.Fatalf("PublicSurfaceGlobs len = %d, want 1 (full replacement)", len(cfg.Routing.PublicSurfaceGlobs))
	}
	if cfg.Routing.PublicSurfaceGlobs[0] != "api/" {
		t.Errorf("PublicSurfaceGlobs[0] = %q, want \"api/\"", cfg.Routing.PublicSurfaceGlobs[0])
	}

	// Other slices should retain defaults when not overridden.
	defaults := config.Default()
	if len(cfg.Routing.StructuralPatterns) != len(defaults.Routing.StructuralPatterns) {
		t.Errorf("StructuralPatterns len = %d, want %d (untouched defaults)",
			len(cfg.Routing.StructuralPatterns), len(defaults.Routing.StructuralPatterns))
	}
	if len(cfg.Routing.MergeBaseCandidates) != len(defaults.Routing.MergeBaseCandidates) {
		t.Errorf("MergeBaseCandidates len = %d, want %d (untouched defaults)",
			len(cfg.Routing.MergeBaseCandidates), len(defaults.Routing.MergeBaseCandidates))
	}
}

func TestLoadRepoConfig_AllSlicesReplace(t *testing.T) {
	dir := t.TempDir()
	repoConfig := filepath.Join(dir, ".bonsai.yaml")
	yaml := `routing:
  public_surface_globs:
    - "proto/"
  structural_patterns:
    - "core/"
  merge_base_candidates:
    - "develop"
`
	if err := os.WriteFile(repoConfig, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cfg.Routing.PublicSurfaceGlobs) != 1 || cfg.Routing.PublicSurfaceGlobs[0] != "proto/" {
		t.Errorf("PublicSurfaceGlobs = %v, want [proto/]", cfg.Routing.PublicSurfaceGlobs)
	}
	if len(cfg.Routing.StructuralPatterns) != 1 || cfg.Routing.StructuralPatterns[0] != "core/" {
		t.Errorf("StructuralPatterns = %v, want [core/]", cfg.Routing.StructuralPatterns)
	}
	if len(cfg.Routing.MergeBaseCandidates) != 1 || cfg.Routing.MergeBaseCandidates[0] != "develop" {
		t.Errorf("MergeBaseCandidates = %v, want [develop]", cfg.Routing.MergeBaseCandidates)
	}
}

func TestModelForCheck(t *testing.T) {
	tests := []struct {
		name string
		r    config.ModelRouting
		cost string
		want string
	}{
		{
			name: "cheap returns check.cheap",
			r:    config.Default().Agents.Models,
			cost: "cheap",
			want: "codex",
		},
		{
			name: "moderate returns check.moderate",
			r:    config.Default().Agents.Models,
			cost: "moderate",
			want: "sonnet",
		},
		{
			name: "heavy returns check.heavy",
			r:    config.Default().Agents.Models,
			cost: "heavy",
			want: "sonnet",
		},
		{
			name: "unknown cost falls back to default",
			r:    config.Default().Agents.Models,
			cost: "exotic",
			want: "sonnet",
		},
		{
			name: "empty cost falls back to default",
			r:    config.Default().Agents.Models,
			cost: "",
			want: "sonnet",
		},
		{
			name: "empty check.cheap falls back to default",
			r:    config.ModelRouting{Default: "fallback"},
			cost: "cheap",
			want: "fallback",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.ModelForCheck(tt.cost)
			if got != tt.want {
				t.Errorf("ModelForCheck(%q) = %q, want %q", tt.cost, got, tt.want)
			}
		})
	}
}

func TestModelForRole(t *testing.T) {
	tests := []struct {
		name string
		r    config.ModelRouting
		role string
		want string
	}{
		{
			name: "implement",
			r:    config.ModelRouting{Default: "sonnet", Implement: "opus"},
			role: "implement",
			want: "opus",
		},
		{
			name: "plan",
			r:    config.ModelRouting{Default: "sonnet", Plan: "opus"},
			role: "plan",
			want: "opus",
		},
		{
			name: "review",
			r:    config.ModelRouting{Default: "sonnet", Review: "haiku"},
			role: "review",
			want: "haiku",
		},
		{
			name: "patch",
			r:    config.ModelRouting{Default: "sonnet", Patch: "opus"},
			role: "patch",
			want: "opus",
		},
		{
			name: "chat",
			r:    config.ModelRouting{Default: "sonnet", Chat: "haiku"},
			role: "chat",
			want: "haiku",
		},
		{
			name: "unknown role falls back to default",
			r:    config.ModelRouting{Default: "sonnet"},
			role: "unknown",
			want: "sonnet",
		},
		{
			name: "empty role string falls back to default",
			r:    config.ModelRouting{Default: "sonnet"},
			role: "",
			want: "sonnet",
		},
		{
			name: "empty role field falls back to default",
			r:    config.ModelRouting{Default: "sonnet", Implement: ""},
			role: "implement",
			want: "sonnet",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.ModelForRole(tt.role)
			if got != tt.want {
				t.Errorf("ModelForRole(%q) = %q, want %q", tt.role, got, tt.want)
			}
		})
	}
}

func TestDefaultModelRouting(t *testing.T) {
	cfg := config.Default()
	m := cfg.Agents.Models

	if m.Default != "sonnet" {
		t.Errorf("Default = %q, want sonnet", m.Default)
	}
	if m.Check.Cheap != "codex" {
		t.Errorf("Check.Cheap = %q, want codex", m.Check.Cheap)
	}
	if m.Check.Moderate != "sonnet" {
		t.Errorf("Check.Moderate = %q, want sonnet", m.Check.Moderate)
	}
	if m.Check.Heavy != "sonnet" {
		t.Errorf("Check.Heavy = %q, want sonnet", m.Check.Heavy)
	}
	if m.Implement != "opus" {
		t.Errorf("Implement = %q, want opus", m.Implement)
	}
	if m.Plan != "opus" {
		t.Errorf("Plan = %q, want opus", m.Plan)
	}
	if m.Review != "codex" {
		t.Errorf("Review = %q, want codex", m.Review)
	}
	if m.Patch != "sonnet" {
		t.Errorf("Patch = %q, want sonnet", m.Patch)
	}
	if m.Chat != "sonnet" {
		t.Errorf("Chat = %q, want sonnet", m.Chat)
	}
}

func TestLoadRepoConfig_ModelRouting(t *testing.T) {
	dir := t.TempDir()
	repoConfig := filepath.Join(dir, ".bonsai.yaml")
	yaml := `agents:
  models:
    default: opus
    check:
      cheap: haiku
      heavy: opus
    implement: opus
    plan: opus
`
	if err := os.WriteFile(repoConfig, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Agents.Models.Default != "opus" {
		t.Errorf("Models.Default = %q, want opus", cfg.Agents.Models.Default)
	}
	if cfg.Agents.Models.Check.Cheap != "haiku" {
		t.Errorf("Models.Check.Cheap = %q, want haiku", cfg.Agents.Models.Check.Cheap)
	}
	if cfg.Agents.Models.Check.Heavy != "opus" {
		t.Errorf("Models.Check.Heavy = %q, want opus", cfg.Agents.Models.Check.Heavy)
	}
	if cfg.Agents.Models.Implement != "opus" {
		t.Errorf("Models.Implement = %q, want opus", cfg.Agents.Models.Implement)
	}
	if cfg.Agents.Models.Plan != "opus" {
		t.Errorf("Models.Plan = %q, want opus", cfg.Agents.Models.Plan)
	}
	// Unset fields should retain defaults
	if cfg.Agents.Models.Review != "codex" {
		t.Errorf("Models.Review = %q, want codex (default)", cfg.Agents.Models.Review)
	}
	if cfg.Agents.Models.Check.Moderate != "sonnet" {
		t.Errorf("Models.Check.Moderate = %q, want sonnet (default)", cfg.Agents.Models.Check.Moderate)
	}
}

func TestLoadEnvOverride_ModelRouting(t *testing.T) {
	t.Setenv("BONSAI_MODEL_DEFAULT", "opus")
	t.Setenv("BONSAI_MODEL_CHECK_CHEAP", "haiku-3")
	t.Setenv("BONSAI_MODEL_CHECK_HEAVY", "opus-4")
	t.Setenv("BONSAI_MODEL_IMPLEMENT", "opus")
	t.Setenv("BONSAI_MODEL_PLAN", "opus")
	t.Setenv("BONSAI_MODEL_REVIEW", "haiku")
	t.Setenv("BONSAI_MODEL_PATCH", "opus")
	t.Setenv("BONSAI_MODEL_CHAT", "haiku")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Agents.Models.Default != "opus" {
		t.Errorf("Default = %q, want opus", cfg.Agents.Models.Default)
	}
	if cfg.Agents.Models.Check.Cheap != "haiku-3" {
		t.Errorf("Check.Cheap = %q, want haiku-3", cfg.Agents.Models.Check.Cheap)
	}
	if cfg.Agents.Models.Check.Heavy != "opus-4" {
		t.Errorf("Check.Heavy = %q, want opus-4", cfg.Agents.Models.Check.Heavy)
	}
	if cfg.Agents.Models.Implement != "opus" {
		t.Errorf("Implement = %q, want opus", cfg.Agents.Models.Implement)
	}
	if cfg.Agents.Models.Plan != "opus" {
		t.Errorf("Plan = %q, want opus", cfg.Agents.Models.Plan)
	}
	if cfg.Agents.Models.Review != "haiku" {
		t.Errorf("Review = %q, want haiku", cfg.Agents.Models.Review)
	}
	if cfg.Agents.Models.Patch != "opus" {
		t.Errorf("Patch = %q, want opus", cfg.Agents.Models.Patch)
	}
	if cfg.Agents.Models.Chat != "haiku" {
		t.Errorf("Chat = %q, want haiku", cfg.Agents.Models.Chat)
	}
}

func TestLoadEnvOverridesRepoConfig(t *testing.T) {
	dir := t.TempDir()
	repoConfig := filepath.Join(dir, ".bonsai.yaml")
	if err := os.WriteFile(repoConfig, []byte("diff:\n  heavy_diff_lines: 1000\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	t.Setenv("BONSAI_DIFF_HEAVY_LINES", "2000")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Env should override repo config
	if cfg.Diff.HeavyDiffLines != 2000 {
		t.Errorf("HeavyDiffLines = %d, want 2000 (env override)", cfg.Diff.HeavyDiffLines)
	}
}
