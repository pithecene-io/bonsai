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
			name: "implementer",
			m:    config.ModelsConfig{Roles: config.RoleModels{Implementer: "opus"}},
			role: "implementer",
			want: "opus",
		},
		{
			name: "planner",
			m:    config.ModelsConfig{Roles: config.RoleModels{Planner: "opus"}},
			role: "planner",
			want: "opus",
		},
		{
			name: "reviewer",
			m:    config.ModelsConfig{Roles: config.RoleModels{Reviewer: "haiku"}},
			role: "reviewer",
			want: "haiku",
		},
		{
			name: "patcher",
			m:    config.ModelsConfig{Roles: config.RoleModels{Patcher: "opus"}},
			role: "patcher",
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
			m:    config.ModelsConfig{Roles: config.RoleModels{Implementer: ""}},
			role: "implementer",
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
	if m.Roles.Implementer != "opus" {
		t.Errorf("Roles.Implementer = %q, want opus", m.Roles.Implementer)
	}
	if m.Roles.Planner != "opus" {
		t.Errorf("Roles.Planner = %q, want opus", m.Roles.Planner)
	}
	if m.Roles.Reviewer != "codex" {
		t.Errorf("Roles.Reviewer = %q, want codex", m.Roles.Reviewer)
	}
	if m.Roles.Patcher != "sonnet" {
		t.Errorf("Roles.Patcher = %q, want sonnet", m.Roles.Patcher)
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
    implementer: opus
    planner: opus
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
	if cfg.Models.Roles.Implementer != "opus" {
		t.Errorf("Models.Roles.Implementer = %q, want opus", cfg.Models.Roles.Implementer)
	}
	if cfg.Models.Roles.Planner != "opus" {
		t.Errorf("Models.Roles.Planner = %q, want opus", cfg.Models.Roles.Planner)
	}
	// Unset fields should retain defaults
	if cfg.Models.Roles.Reviewer != "codex" {
		t.Errorf("Models.Roles.Reviewer = %q, want codex (default)", cfg.Models.Roles.Reviewer)
	}
	if cfg.Models.Skills.Moderate != "sonnet" {
		t.Errorf("Models.Skills.Moderate = %q, want sonnet (default)", cfg.Models.Skills.Moderate)
	}
}

func TestLoadEnvOverride_ModelsConfig(t *testing.T) {
	t.Setenv("BONSAI_MODEL_SKILL_CHEAP", "haiku-3")
	t.Setenv("BONSAI_MODEL_SKILL_HEAVY", "opus-4")
	t.Setenv("BONSAI_MODEL_ROLE_IMPLEMENTER", "opus")
	t.Setenv("BONSAI_MODEL_ROLE_PLANNER", "opus")
	t.Setenv("BONSAI_MODEL_ROLE_REVIEWER", "haiku")
	t.Setenv("BONSAI_MODEL_ROLE_PATCHER", "opus")
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
	if cfg.Models.Roles.Implementer != "opus" {
		t.Errorf("Roles.Implementer = %q, want opus", cfg.Models.Roles.Implementer)
	}
	if cfg.Models.Roles.Planner != "opus" {
		t.Errorf("Roles.Planner = %q, want opus", cfg.Models.Roles.Planner)
	}
	if cfg.Models.Roles.Reviewer != "haiku" {
		t.Errorf("Roles.Reviewer = %q, want haiku", cfg.Models.Roles.Reviewer)
	}
	if cfg.Models.Roles.Patcher != "opus" {
		t.Errorf("Roles.Patcher = %q, want opus", cfg.Models.Roles.Patcher)
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

