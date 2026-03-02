package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load resolves configuration from the multi-source merge chain:
//
//  1. Embedded defaults (compiled into binary)
//  2. User config: ~/.config/bonsai/config.yaml
//  3. Repo config: <repoRoot>/.bonsai.yaml
//  4. Environment variables: BONSAI_* prefix
//
// CLI flags are applied by the caller after Load returns.
func Load(repoRoot string) (*Config, error) {
	cfg := Default()

	// 2. User config
	userPath := userConfigPath()
	if userPath != "" {
		if err := mergeFromFile(cfg, userPath); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	// 3. Repo config
	if repoRoot != "" {
		repoPath := filepath.Join(repoRoot, ".bonsai.yaml")
		if err := mergeFromFile(cfg, repoPath); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	// 4. Environment variables
	mergeFromEnv(cfg)

	return cfg, nil
}

func userConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "bonsai", "config.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "bonsai", "config.yaml")
}

func mergeFromFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Parse into a fresh config and merge non-zero values
	var overlay Config
	if err := yaml.Unmarshal(data, &overlay); err != nil {
		return err
	}

	mergeConfig(cfg, &overlay)
	return nil
}

// mergeFromEnv applies BONSAI_* environment variable overrides.
func mergeFromEnv(cfg *Config) {
	mergeIntEnvs(cfg)
	mergeStringEnvs(cfg)
	mergeMiscEnvs(cfg)
}

// mergeIntEnvs applies integer env overrides.
func mergeIntEnvs(cfg *Config) {
	intBindings := []struct {
		env string
		dst *int
	}{
		{"BONSAI_DIFF_HEAVY_LINES", &cfg.Diff.HeavyDiffLines},
		{"BONSAI_DIFF_HEAVY_FILES", &cfg.Diff.HeavyFilesChanged},
		{"BONSAI_DIFF_PATCH_MAX_FILES", &cfg.Diff.PatchMaxFiles},
		{"BONSAI_GATE_MAX_ITERATIONS", &cfg.Gate.MaxIterations},
		{"BONSAI_FIX_MAX_ITERATIONS", &cfg.Fix.MaxIterations},
	}
	for _, b := range intBindings {
		if v := os.Getenv(b.env); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				*b.dst = n
			}
		}
	}
}

// mergeStringEnvs applies string env overrides for new-path names.
func mergeStringEnvs(cfg *Config) {
	stringBindings := []struct {
		env string
		dst *string
	}{
		{"BONSAI_CLAUDE_BIN", &cfg.Agents.Claude.Bin},
		{"BONSAI_CODEX_BIN", &cfg.Agents.Codex.Bin},
		{"BONSAI_PROVIDER_ANTHROPIC_API_KEY", &cfg.Providers.Anthropic.APIKey},
		{"BONSAI_MODEL_SKILL_CHEAP", &cfg.Models.Skills.Cheap},
		{"BONSAI_MODEL_SKILL_MODERATE", &cfg.Models.Skills.Moderate},
		{"BONSAI_MODEL_SKILL_HEAVY", &cfg.Models.Skills.Heavy},
		{"BONSAI_MODEL_ROLE_IMPLEMENTER", &cfg.Models.Roles.Implementer},
		{"BONSAI_MODEL_ROLE_PLANNER", &cfg.Models.Roles.Planner},
		{"BONSAI_MODEL_ROLE_REVIEWER", &cfg.Models.Roles.Reviewer},
		{"BONSAI_MODEL_ROLE_PATCHER", &cfg.Models.Roles.Patcher},
		{"BONSAI_MODEL_ROLE_CHAT", &cfg.Models.Roles.Chat},
		{"BONSAI_OUTPUT_DIR", &cfg.Output.Dir},
	}
	for _, b := range stringBindings {
		if v := os.Getenv(b.env); v != "" {
			*b.dst = v
		}
	}
}

// mergeMiscEnvs applies env overrides with non-trivial logic.
func mergeMiscEnvs(cfg *Config) {
	if v := os.Getenv("BONSAI_CHECK_JOBS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Check.Concurrency = intPtr(n)
		}
	}
	if v := os.Getenv("BONSAI_SKILLS_EXTRA_DIRS"); v != "" {
		cfg.Skills.ExtraDirs = strings.Split(v, ":")
	}
}

// mergeConfig merges non-zero values from src into dst.
func mergeConfig(dst, src *Config) {
	mergeDiffConfig(dst, src)
	mergeRoutingConfig(dst, src)
	mergeScalarConfig(dst, src)
	mergeModelsConfig(&dst.Models, &src.Models)
}

// mergeDiffConfig merges diff threshold overrides.
func mergeDiffConfig(dst, src *Config) {
	if src.Diff.HeavyDiffLines > 0 {
		dst.Diff.HeavyDiffLines = src.Diff.HeavyDiffLines
	}
	if src.Diff.HeavyFilesChanged > 0 {
		dst.Diff.HeavyFilesChanged = src.Diff.HeavyFilesChanged
	}
	if src.Diff.PatchMaxFiles > 0 {
		dst.Diff.PatchMaxFiles = src.Diff.PatchMaxFiles
	}
}

// mergeRoutingConfig merges routing slice overrides (full replacement).
func mergeRoutingConfig(dst, src *Config) {
	if len(src.Routing.PublicSurfaceGlobs) > 0 {
		dst.Routing.PublicSurfaceGlobs = src.Routing.PublicSurfaceGlobs
	}
	if len(src.Routing.StructuralPatterns) > 0 {
		dst.Routing.StructuralPatterns = src.Routing.StructuralPatterns
	}
	if len(src.Routing.MergeBaseCandidates) > 0 {
		dst.Routing.MergeBaseCandidates = src.Routing.MergeBaseCandidates
	}
}

// mergeScalarConfig merges remaining scalar config fields.
func mergeScalarConfig(dst, src *Config) {
	if src.Gate.MaxIterations > 0 {
		dst.Gate.MaxIterations = src.Gate.MaxIterations
	}
	if src.Check.Concurrency != nil {
		dst.Check.Concurrency = src.Check.Concurrency
	}
	if src.Fix.MaxIterations > 0 {
		dst.Fix.MaxIterations = src.Fix.MaxIterations
	}
	if src.Providers.Anthropic.APIKey != "" {
		dst.Providers.Anthropic.APIKey = src.Providers.Anthropic.APIKey
	}
	if src.Agents.Claude.Bin != "" {
		dst.Agents.Claude.Bin = src.Agents.Claude.Bin
	}
	if src.Agents.Codex.Bin != "" {
		dst.Agents.Codex.Bin = src.Agents.Codex.Bin
	}
	if src.Output.Dir != "" {
		dst.Output.Dir = src.Output.Dir
	}
	if len(src.Skills.ExtraDirs) > 0 {
		dst.Skills.ExtraDirs = src.Skills.ExtraDirs
	}
}

// mergeModelsConfig merges non-empty model config fields from src into dst.
func mergeModelsConfig(dst, src *ModelsConfig) {
	skills := []struct {
		src string
		dst *string
	}{
		{src.Skills.Cheap, &dst.Skills.Cheap},
		{src.Skills.Moderate, &dst.Skills.Moderate},
		{src.Skills.Heavy, &dst.Skills.Heavy},
	}
	for _, s := range skills {
		if s.src != "" {
			*s.dst = s.src
		}
	}

	roles := []struct {
		src string
		dst *string
	}{
		{src.Roles.Implementer, &dst.Roles.Implementer},
		{src.Roles.Planner, &dst.Roles.Planner},
		{src.Roles.Reviewer, &dst.Roles.Reviewer},
		{src.Roles.Patcher, &dst.Roles.Patcher},
		{src.Roles.Chat, &dst.Roles.Chat},
	}
	for _, r := range roles {
		if r.src != "" {
			*r.dst = r.src
		}
	}
}
