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
	t.Setenv("BONSAI_CLAUDE_BIN", "/usr/local/bin/claude")
	t.Setenv("BONSAI_GATE_MAX_ITERATIONS", "7")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Diff.HeavyDiffLines != 2000 {
		t.Errorf("HeavyDiffLines = %d, want 2000", cfg.Diff.HeavyDiffLines)
	}
	if cfg.Agents.Claude.Bin != "/usr/local/bin/claude" {
		t.Errorf("Claude.Bin = %q, want /usr/local/bin/claude", cfg.Agents.Claude.Bin)
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
