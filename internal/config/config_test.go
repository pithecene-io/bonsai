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
