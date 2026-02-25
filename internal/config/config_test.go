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

func TestModelForSkill(t *testing.T) {
	tests := []struct {
		name string
		m    config.ModelsConfig
		cost string
		want string
	}{
		{
			name: "cheap returns skills.cheap",
			m:    config.Default().Models,
			cost: "cheap",
			want: "haiku",
		},
		{
			name: "moderate returns skills.moderate",
			m:    config.Default().Models,
			cost: "moderate",
			want: "sonnet",
		},
		{
			name: "heavy returns skills.heavy",
			m:    config.Default().Models,
			cost: "heavy",
			want: "sonnet",
		},
		{
			name: "unknown cost returns empty",
			m:    config.Default().Models,
			cost: "exotic",
			want: "",
		},
		{
			name: "empty cost returns empty",
			m:    config.Default().Models,
			cost: "",
			want: "",
		},
		{
			name: "empty skills.cheap returns empty",
			m:    config.ModelsConfig{},
			cost: "cheap",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.m.ModelForSkill(tt.cost)
			if got != tt.want {
				t.Errorf("ModelForSkill(%q) = %q, want %q", tt.cost, got, tt.want)
			}
		})
	}
}

func TestModelForRole(t *testing.T) {
	tests := []struct {
		name string
		m    config.ModelsConfig
		role string
		want string
	}{
		{
			name: "implement",
			m:    config.ModelsConfig{Roles: config.RoleModels{Implement: "opus"}},
			role: "implement",
			want: "opus",
		},
		{
			name: "plan",
			m:    config.ModelsConfig{Roles: config.RoleModels{Plan: "opus"}},
			role: "plan",
			want: "opus",
		},
		{
			name: "review",
			m:    config.ModelsConfig{Roles: config.RoleModels{Review: "haiku"}},
			role: "review",
			want: "haiku",
		},
		{
			name: "patch",
			m:    config.ModelsConfig{Roles: config.RoleModels{Patch: "opus"}},
			role: "patch",
			want: "opus",
		},
		{
			name: "chat",
			m:    config.ModelsConfig{Roles: config.RoleModels{Chat: "haiku"}},
			role: "chat",
			want: "haiku",
		},
		{
			name: "unknown role returns empty",
			m:    config.ModelsConfig{},
			role: "unknown",
			want: "",
		},
		{
			name: "empty role string returns empty",
			m:    config.ModelsConfig{},
			role: "",
			want: "",
		},
		{
			name: "empty role field returns empty",
			m:    config.ModelsConfig{Roles: config.RoleModels{Implement: ""}},
			role: "implement",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.m.ModelForRole(tt.role)
			if got != tt.want {
				t.Errorf("ModelForRole(%q) = %q, want %q", tt.role, got, tt.want)
			}
		})
	}
}

func TestDefaultModelsConfig(t *testing.T) {
	cfg := config.Default()
	m := cfg.Models

	if m.Skills.Cheap != "haiku" {
		t.Errorf("Skills.Cheap = %q, want haiku", m.Skills.Cheap)
	}
	if m.Skills.Moderate != "sonnet" {
		t.Errorf("Skills.Moderate = %q, want sonnet", m.Skills.Moderate)
	}
	if m.Skills.Heavy != "sonnet" {
		t.Errorf("Skills.Heavy = %q, want sonnet", m.Skills.Heavy)
	}
	if m.Roles.Implement != "opus" {
		t.Errorf("Roles.Implement = %q, want opus", m.Roles.Implement)
	}
	if m.Roles.Plan != "opus" {
		t.Errorf("Roles.Plan = %q, want opus", m.Roles.Plan)
	}
	if m.Roles.Review != "codex" {
		t.Errorf("Roles.Review = %q, want codex", m.Roles.Review)
	}
	if m.Roles.Patch != "sonnet" {
		t.Errorf("Roles.Patch = %q, want sonnet", m.Roles.Patch)
	}
	if m.Roles.Chat != "sonnet" {
		t.Errorf("Roles.Chat = %q, want sonnet", m.Roles.Chat)
	}
}

func TestLoadRepoConfig_ModelsConfig(t *testing.T) {
	dir := t.TempDir()
	repoConfig := filepath.Join(dir, ".bonsai.yaml")
	yaml := `models:
  skills:
    cheap: haiku
    heavy: opus
  roles:
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

	if cfg.Models.Skills.Cheap != "haiku" {
		t.Errorf("Models.Skills.Cheap = %q, want haiku", cfg.Models.Skills.Cheap)
	}
	if cfg.Models.Skills.Heavy != "opus" {
		t.Errorf("Models.Skills.Heavy = %q, want opus", cfg.Models.Skills.Heavy)
	}
	if cfg.Models.Roles.Implement != "opus" {
		t.Errorf("Models.Roles.Implement = %q, want opus", cfg.Models.Roles.Implement)
	}
	if cfg.Models.Roles.Plan != "opus" {
		t.Errorf("Models.Roles.Plan = %q, want opus", cfg.Models.Roles.Plan)
	}
	// Unset fields should retain defaults
	if cfg.Models.Roles.Review != "codex" {
		t.Errorf("Models.Roles.Review = %q, want codex (default)", cfg.Models.Roles.Review)
	}
	if cfg.Models.Skills.Moderate != "sonnet" {
		t.Errorf("Models.Skills.Moderate = %q, want sonnet (default)", cfg.Models.Skills.Moderate)
	}
}

func TestLoadEnvOverride_ModelsConfig(t *testing.T) {
	t.Setenv("BONSAI_MODEL_SKILL_CHEAP", "haiku-3")
	t.Setenv("BONSAI_MODEL_SKILL_HEAVY", "opus-4")
	t.Setenv("BONSAI_MODEL_ROLE_IMPLEMENT", "opus")
	t.Setenv("BONSAI_MODEL_ROLE_PLAN", "opus")
	t.Setenv("BONSAI_MODEL_ROLE_REVIEW", "haiku")
	t.Setenv("BONSAI_MODEL_ROLE_PATCH", "opus")
	t.Setenv("BONSAI_MODEL_ROLE_CHAT", "haiku")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Models.Skills.Cheap != "haiku-3" {
		t.Errorf("Skills.Cheap = %q, want haiku-3", cfg.Models.Skills.Cheap)
	}
	if cfg.Models.Skills.Heavy != "opus-4" {
		t.Errorf("Skills.Heavy = %q, want opus-4", cfg.Models.Skills.Heavy)
	}
	if cfg.Models.Roles.Implement != "opus" {
		t.Errorf("Roles.Implement = %q, want opus", cfg.Models.Roles.Implement)
	}
	if cfg.Models.Roles.Plan != "opus" {
		t.Errorf("Roles.Plan = %q, want opus", cfg.Models.Roles.Plan)
	}
	if cfg.Models.Roles.Review != "haiku" {
		t.Errorf("Roles.Review = %q, want haiku", cfg.Models.Roles.Review)
	}
	if cfg.Models.Roles.Patch != "opus" {
		t.Errorf("Roles.Patch = %q, want opus", cfg.Models.Roles.Patch)
	}
	if cfg.Models.Roles.Chat != "haiku" {
		t.Errorf("Roles.Chat = %q, want haiku", cfg.Models.Roles.Chat)
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

func TestLoadRepoConfig_ConcurrencyZeroOverridesNonZero(t *testing.T) {
	// First layer: user config sets concurrency to 4
	userDir := t.TempDir()
	userCfg := filepath.Join(userDir, "bonsai", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(userCfg), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(userCfg, []byte("check:\n  concurrency: 4\n"), 0o644); err != nil {
		t.Fatalf("write user config: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", userDir)

	// Second layer: repo config sets concurrency to 0 (unlimited)
	repoDir := t.TempDir()
	repoConfig := filepath.Join(repoDir, ".bonsai.yaml")
	if err := os.WriteFile(repoConfig, []byte("check:\n  concurrency: 0\n"), 0o644); err != nil {
		t.Fatalf("write repo config: %v", err)
	}

	cfg, err := config.Load(repoDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Check.Concurrency == nil {
		t.Fatal("Check.Concurrency is nil, want 0 (unlimited)")
	}
	if *cfg.Check.Concurrency != 0 {
		t.Errorf("Check.Concurrency = %d, want 0 (unlimited overrides prior 4)", *cfg.Check.Concurrency)
	}
}

func TestLoadRepoConfig_LegacyAgentsCompat(t *testing.T) {
	dir := t.TempDir()
	repoConfig := filepath.Join(dir, ".bonsai.yaml")
	yaml := `agents:
  anthropic:
    api_key: sk-legacy-key
  models:
    check:
      cheap: haiku-legacy
      moderate: sonnet-legacy
      heavy: opus-legacy
    implement: opus-legacy
    plan: opus-legacy
    review: codex-legacy
    patch: sonnet-legacy
    chat: sonnet-legacy
`
	if err := os.WriteFile(repoConfig, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Providers.Anthropic.APIKey != "sk-legacy-key" {
		t.Errorf("Providers.Anthropic.APIKey = %q, want sk-legacy-key", cfg.Providers.Anthropic.APIKey)
	}
	if cfg.Models.Skills.Cheap != "haiku-legacy" {
		t.Errorf("Skills.Cheap = %q, want haiku-legacy", cfg.Models.Skills.Cheap)
	}
	if cfg.Models.Skills.Moderate != "sonnet-legacy" {
		t.Errorf("Skills.Moderate = %q, want sonnet-legacy", cfg.Models.Skills.Moderate)
	}
	if cfg.Models.Skills.Heavy != "opus-legacy" {
		t.Errorf("Skills.Heavy = %q, want opus-legacy", cfg.Models.Skills.Heavy)
	}
	if cfg.Models.Roles.Implement != "opus-legacy" {
		t.Errorf("Roles.Implement = %q, want opus-legacy", cfg.Models.Roles.Implement)
	}
	if cfg.Models.Roles.Plan != "opus-legacy" {
		t.Errorf("Roles.Plan = %q, want opus-legacy", cfg.Models.Roles.Plan)
	}
	if cfg.Models.Roles.Review != "codex-legacy" {
		t.Errorf("Roles.Review = %q, want codex-legacy", cfg.Models.Roles.Review)
	}
	if cfg.Models.Roles.Patch != "sonnet-legacy" {
		t.Errorf("Roles.Patch = %q, want sonnet-legacy", cfg.Models.Roles.Patch)
	}
	if cfg.Models.Roles.Chat != "sonnet-legacy" {
		t.Errorf("Roles.Chat = %q, want sonnet-legacy", cfg.Models.Roles.Chat)
	}
}

func TestLoadRepoConfig_NewPathWinsOverLegacy(t *testing.T) {
	dir := t.TempDir()
	repoConfig := filepath.Join(dir, ".bonsai.yaml")
	// File sets BOTH old and new paths — new must win.
	yaml := `providers:
  anthropic:
    api_key: sk-new-key
agents:
  anthropic:
    api_key: sk-legacy-key
models:
  skills:
    cheap: haiku-new
  roles:
    implement: opus-new
`
	if err := os.WriteFile(repoConfig, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Providers.Anthropic.APIKey != "sk-new-key" {
		t.Errorf("Providers.Anthropic.APIKey = %q, want sk-new-key (new wins)", cfg.Providers.Anthropic.APIKey)
	}
	if cfg.Models.Skills.Cheap != "haiku-new" {
		t.Errorf("Skills.Cheap = %q, want haiku-new (new wins)", cfg.Models.Skills.Cheap)
	}
	if cfg.Models.Roles.Implement != "opus-new" {
		t.Errorf("Roles.Implement = %q, want opus-new (new wins)", cfg.Models.Roles.Implement)
	}
}

func TestLoadEnvOverride_LegacyEnvCompat(t *testing.T) {
	t.Setenv("BONSAI_ANTHROPIC_API_KEY", "sk-env-legacy")
	t.Setenv("BONSAI_MODEL_CHECK_CHEAP", "haiku-env-legacy")
	t.Setenv("BONSAI_MODEL_CHECK_MODERATE", "sonnet-env-legacy")
	t.Setenv("BONSAI_MODEL_CHECK_HEAVY", "opus-env-legacy")
	t.Setenv("BONSAI_MODEL_IMPLEMENT", "opus-env-legacy")
	t.Setenv("BONSAI_MODEL_PLAN", "opus-env-legacy")
	t.Setenv("BONSAI_MODEL_REVIEW", "codex-env-legacy")
	t.Setenv("BONSAI_MODEL_PATCH", "sonnet-env-legacy")
	t.Setenv("BONSAI_MODEL_CHAT", "sonnet-env-legacy")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Providers.Anthropic.APIKey != "sk-env-legacy" {
		t.Errorf("Providers.Anthropic.APIKey = %q, want sk-env-legacy", cfg.Providers.Anthropic.APIKey)
	}
	if cfg.Models.Skills.Cheap != "haiku-env-legacy" {
		t.Errorf("Skills.Cheap = %q, want haiku-env-legacy", cfg.Models.Skills.Cheap)
	}
	if cfg.Models.Skills.Moderate != "sonnet-env-legacy" {
		t.Errorf("Skills.Moderate = %q, want sonnet-env-legacy", cfg.Models.Skills.Moderate)
	}
	if cfg.Models.Skills.Heavy != "opus-env-legacy" {
		t.Errorf("Skills.Heavy = %q, want opus-env-legacy", cfg.Models.Skills.Heavy)
	}
	if cfg.Models.Roles.Implement != "opus-env-legacy" {
		t.Errorf("Roles.Implement = %q, want opus-env-legacy", cfg.Models.Roles.Implement)
	}
	if cfg.Models.Roles.Plan != "opus-env-legacy" {
		t.Errorf("Roles.Plan = %q, want opus-env-legacy", cfg.Models.Roles.Plan)
	}
	if cfg.Models.Roles.Review != "codex-env-legacy" {
		t.Errorf("Roles.Review = %q, want codex-env-legacy", cfg.Models.Roles.Review)
	}
	if cfg.Models.Roles.Patch != "sonnet-env-legacy" {
		t.Errorf("Roles.Patch = %q, want sonnet-env-legacy", cfg.Models.Roles.Patch)
	}
	if cfg.Models.Roles.Chat != "sonnet-env-legacy" {
		t.Errorf("Roles.Chat = %q, want sonnet-env-legacy", cfg.Models.Roles.Chat)
	}
}

func TestLoadEnvOverride_NewEnvWinsOverLegacy(t *testing.T) {
	// Set both old and new env vars — new must win.
	t.Setenv("BONSAI_PROVIDER_ANTHROPIC_API_KEY", "sk-new-env")
	t.Setenv("BONSAI_ANTHROPIC_API_KEY", "sk-legacy-env")
	t.Setenv("BONSAI_MODEL_SKILL_CHEAP", "haiku-new-env")
	t.Setenv("BONSAI_MODEL_CHECK_CHEAP", "haiku-legacy-env")
	t.Setenv("BONSAI_MODEL_ROLE_IMPLEMENT", "opus-new-env")
	t.Setenv("BONSAI_MODEL_IMPLEMENT", "opus-legacy-env")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Providers.Anthropic.APIKey != "sk-new-env" {
		t.Errorf("Providers.Anthropic.APIKey = %q, want sk-new-env (new wins)", cfg.Providers.Anthropic.APIKey)
	}
	if cfg.Models.Skills.Cheap != "haiku-new-env" {
		t.Errorf("Skills.Cheap = %q, want haiku-new-env (new wins)", cfg.Models.Skills.Cheap)
	}
	if cfg.Models.Roles.Implement != "opus-new-env" {
		t.Errorf("Roles.Implement = %q, want opus-new-env (new wins)", cfg.Models.Roles.Implement)
	}
}
