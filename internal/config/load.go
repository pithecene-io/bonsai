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

// mergeConfig merges non-zero values from src into dst.
func mergeConfig(dst, src *Config) {
	// Diff
	if src.Diff.HeavyDiffLines > 0 {
		dst.Diff.HeavyDiffLines = src.Diff.HeavyDiffLines
	}
	if src.Diff.HeavyFilesChanged > 0 {
		dst.Diff.HeavyFilesChanged = src.Diff.HeavyFilesChanged
	}
	if src.Diff.PatchMaxFiles > 0 {
		dst.Diff.PatchMaxFiles = src.Diff.PatchMaxFiles
	}

	// Routing
	if len(src.Routing.PublicSurfaceGlobs) > 0 {
		dst.Routing.PublicSurfaceGlobs = src.Routing.PublicSurfaceGlobs
	}
	if len(src.Routing.StructuralPatterns) > 0 {
		dst.Routing.StructuralPatterns = src.Routing.StructuralPatterns
	}
	if len(src.Routing.MergeBaseCandidates) > 0 {
		dst.Routing.MergeBaseCandidates = src.Routing.MergeBaseCandidates
	}

	// Gate
	if src.Gate.MaxIterations > 0 {
		dst.Gate.MaxIterations = src.Gate.MaxIterations
	}

	// Check
	if src.Check.Concurrency != nil {
		dst.Check.Concurrency = src.Check.Concurrency
	}

	// Providers
	if src.Providers.Anthropic.APIKey != "" {
		dst.Providers.Anthropic.APIKey = src.Providers.Anthropic.APIKey
	}

	// Agents
	if src.Agents.Claude.Bin != "" {
		dst.Agents.Claude.Bin = src.Agents.Claude.Bin
	}
	if src.Agents.Codex.Bin != "" {
		dst.Agents.Codex.Bin = src.Agents.Codex.Bin
	}

	// Models
	mergeModelsConfig(&dst.Models, &src.Models)

	// Output
	if src.Output.Dir != "" {
		dst.Output.Dir = src.Output.Dir
	}

	// Skills
	if len(src.Skills.ExtraDirs) > 0 {
		dst.Skills.ExtraDirs = src.Skills.ExtraDirs
	}
}

// mergeModelsConfig merges non-empty model config fields from src into dst.
func mergeModelsConfig(dst, src *ModelsConfig) {
	// Skills
	if src.Skills.Cheap != "" {
		dst.Skills.Cheap = src.Skills.Cheap
	}
	if src.Skills.Moderate != "" {
		dst.Skills.Moderate = src.Skills.Moderate
	}
	if src.Skills.Heavy != "" {
		dst.Skills.Heavy = src.Skills.Heavy
	}

	// Roles
	if src.Roles.Implement != "" {
		dst.Roles.Implement = src.Roles.Implement
	}
	if src.Roles.Plan != "" {
		dst.Roles.Plan = src.Roles.Plan
	}
	if src.Roles.Review != "" {
		dst.Roles.Review = src.Roles.Review
	}
	if src.Roles.Patch != "" {
		dst.Roles.Patch = src.Roles.Patch
	}
	if src.Roles.Chat != "" {
		dst.Roles.Chat = src.Roles.Chat
	}
}

// mergeFromEnv applies BONSAI_* environment variable overrides.
func mergeFromEnv(cfg *Config) {
	if v := os.Getenv("BONSAI_DIFF_HEAVY_LINES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Diff.HeavyDiffLines = n
		}
	}
	if v := os.Getenv("BONSAI_DIFF_HEAVY_FILES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Diff.HeavyFilesChanged = n
		}
	}
	if v := os.Getenv("BONSAI_DIFF_PATCH_MAX_FILES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Diff.PatchMaxFiles = n
		}
	}
	if v := os.Getenv("BONSAI_GATE_MAX_ITERATIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Gate.MaxIterations = n
		}
	}
	if v := os.Getenv("BONSAI_CLAUDE_BIN"); v != "" {
		cfg.Agents.Claude.Bin = v
	}
	if v := os.Getenv("BONSAI_CODEX_BIN"); v != "" {
		cfg.Agents.Codex.Bin = v
	}
	if v := os.Getenv("BONSAI_PROVIDER_ANTHROPIC_API_KEY"); v != "" {
		cfg.Providers.Anthropic.APIKey = v
	}
	if v := os.Getenv("BONSAI_MODEL_SKILL_CHEAP"); v != "" {
		cfg.Models.Skills.Cheap = v
	}
	if v := os.Getenv("BONSAI_MODEL_SKILL_MODERATE"); v != "" {
		cfg.Models.Skills.Moderate = v
	}
	if v := os.Getenv("BONSAI_MODEL_SKILL_HEAVY"); v != "" {
		cfg.Models.Skills.Heavy = v
	}
	if v := os.Getenv("BONSAI_MODEL_ROLE_IMPLEMENT"); v != "" {
		cfg.Models.Roles.Implement = v
	}
	if v := os.Getenv("BONSAI_MODEL_ROLE_PLAN"); v != "" {
		cfg.Models.Roles.Plan = v
	}
	if v := os.Getenv("BONSAI_MODEL_ROLE_REVIEW"); v != "" {
		cfg.Models.Roles.Review = v
	}
	if v := os.Getenv("BONSAI_MODEL_ROLE_PATCH"); v != "" {
		cfg.Models.Roles.Patch = v
	}
	if v := os.Getenv("BONSAI_MODEL_ROLE_CHAT"); v != "" {
		cfg.Models.Roles.Chat = v
	}
	if v := os.Getenv("BONSAI_OUTPUT_DIR"); v != "" {
		cfg.Output.Dir = v
	}
	if v := os.Getenv("BONSAI_CHECK_JOBS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Check.Concurrency = intPtr(n)
		}
	}
	if v := os.Getenv("BONSAI_SKILLS_EXTRA_DIRS"); v != "" {
		cfg.Skills.ExtraDirs = strings.Split(v, ":")
	}
}
