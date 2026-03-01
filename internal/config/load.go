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

	// Backward compat: promote legacy agents.anthropic / agents.models
	// into the new top-level paths when the new paths are absent.
	var legacy legacyAgentsOverlay
	if err := yaml.Unmarshal(data, &legacy); err == nil {
		promoteLegacyAgents(&overlay, &legacy)
	}

	mergeConfig(cfg, &overlay)
	return nil
}

// legacyAgentsOverlay captures the pre-v0.5 agents.anthropic and
// agents.models YAML paths so existing .bonsai.yaml files keep working.
type legacyAgentsOverlay struct {
	Agents struct {
		Anthropic AnthropicConfig `yaml:"anthropic"`
		Models    struct {
			Default string `yaml:"default"`
			Check   struct {
				Cheap    string `yaml:"cheap"`
				Moderate string `yaml:"moderate"`
				Heavy    string `yaml:"heavy"`
			} `yaml:"check"`
			Implement string `yaml:"implement"`
			Plan      string `yaml:"plan"`
			Review    string `yaml:"review"`
			Patch     string `yaml:"patch"`
			Chat      string `yaml:"chat"`
		} `yaml:"models"`
	} `yaml:"agents"`
}

// promoteLegacyAgents copies legacy agents.* values into the overlay's
// new top-level paths, but only when the new-path field is still empty
// (so an overlay that sets both old and new paths lets the new path win).
func promoteLegacyAgents(overlay *Config, legacy *legacyAgentsOverlay) {
	la := &legacy.Agents
	if la.Anthropic.APIKey != "" && overlay.Providers.Anthropic.APIKey == "" {
		overlay.Providers.Anthropic.APIKey = la.Anthropic.APIKey
	}

	lm := &la.Models

	// Table of legacy -> new field mappings.
	type mapping struct {
		src string
		dst *string
	}
	mappings := []mapping{
		{lm.Check.Cheap, &overlay.Models.Skills.Cheap},
		{lm.Check.Moderate, &overlay.Models.Skills.Moderate},
		{lm.Check.Heavy, &overlay.Models.Skills.Heavy},
		{lm.Implement, &overlay.Models.Roles.Implement},
		{lm.Plan, &overlay.Models.Roles.Plan},
		{lm.Review, &overlay.Models.Roles.Review},
		{lm.Patch, &overlay.Models.Roles.Patch},
		{lm.Chat, &overlay.Models.Roles.Chat},
	}

	for _, m := range mappings {
		if m.src != "" && *m.dst == "" {
			*m.dst = m.src
		}
	}

	// Legacy blanket default — fills any still-empty slot.
	if d := lm.Default; d != "" {
		for _, m := range mappings {
			if *m.dst == "" {
				*m.dst = d
			}
		}
	}
}

// mergeFromEnv applies BONSAI_* environment variable overrides.
func mergeFromEnv(cfg *Config) {
	mergeIntEnvs(cfg)
	mergeStringEnvs(cfg)
	mergeMiscEnvs(cfg)
	mergeLegacyEnvs(cfg)
	mergeLegacyDefaultEnv(cfg)
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
		{"BONSAI_MODEL_ROLE_IMPLEMENT", &cfg.Models.Roles.Implement},
		{"BONSAI_MODEL_ROLE_PLAN", &cfg.Models.Roles.Plan},
		{"BONSAI_MODEL_ROLE_REVIEW", &cfg.Models.Roles.Review},
		{"BONSAI_MODEL_ROLE_PATCH", &cfg.Models.Roles.Patch},
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

// mergeLegacyEnvs applies old-name env vars only when the new name is absent.
func mergeLegacyEnvs(cfg *Config) {
	legacyBindings := []struct {
		newEnv    string
		legacyEnv string
		dst       *string
	}{
		{"BONSAI_PROVIDER_ANTHROPIC_API_KEY", "BONSAI_ANTHROPIC_API_KEY", &cfg.Providers.Anthropic.APIKey},
		{"BONSAI_MODEL_SKILL_CHEAP", "BONSAI_MODEL_CHECK_CHEAP", &cfg.Models.Skills.Cheap},
		{"BONSAI_MODEL_SKILL_MODERATE", "BONSAI_MODEL_CHECK_MODERATE", &cfg.Models.Skills.Moderate},
		{"BONSAI_MODEL_SKILL_HEAVY", "BONSAI_MODEL_CHECK_HEAVY", &cfg.Models.Skills.Heavy},
		{"BONSAI_MODEL_ROLE_IMPLEMENT", "BONSAI_MODEL_IMPLEMENT", &cfg.Models.Roles.Implement},
		{"BONSAI_MODEL_ROLE_PLAN", "BONSAI_MODEL_PLAN", &cfg.Models.Roles.Plan},
		{"BONSAI_MODEL_ROLE_REVIEW", "BONSAI_MODEL_REVIEW", &cfg.Models.Roles.Review},
		{"BONSAI_MODEL_ROLE_PATCH", "BONSAI_MODEL_PATCH", &cfg.Models.Roles.Patch},
		{"BONSAI_MODEL_ROLE_CHAT", "BONSAI_MODEL_CHAT", &cfg.Models.Roles.Chat},
	}
	for _, b := range legacyBindings {
		if os.Getenv(b.newEnv) != "" {
			continue
		}
		if v := os.Getenv(b.legacyEnv); v != "" {
			*b.dst = v
		}
	}
}

// mergeLegacyDefaultEnv fills any slot not already overridden by
// a specific env var (new or legacy) with the blanket default.
func mergeLegacyDefaultEnv(cfg *Config) {
	v := os.Getenv("BONSAI_MODEL_DEFAULT")
	if v == "" {
		return
	}

	defaultBindings := []struct {
		guards []string // env names that block this default
		dst    *string
	}{
		{[]string{"BONSAI_MODEL_SKILL_CHEAP", "BONSAI_MODEL_CHECK_CHEAP"}, &cfg.Models.Skills.Cheap},
		{[]string{"BONSAI_MODEL_SKILL_MODERATE", "BONSAI_MODEL_CHECK_MODERATE"}, &cfg.Models.Skills.Moderate},
		{[]string{"BONSAI_MODEL_SKILL_HEAVY", "BONSAI_MODEL_CHECK_HEAVY"}, &cfg.Models.Skills.Heavy},
		{[]string{"BONSAI_MODEL_ROLE_IMPLEMENT", "BONSAI_MODEL_IMPLEMENT"}, &cfg.Models.Roles.Implement},
		{[]string{"BONSAI_MODEL_ROLE_PLAN", "BONSAI_MODEL_PLAN"}, &cfg.Models.Roles.Plan},
		{[]string{"BONSAI_MODEL_ROLE_REVIEW", "BONSAI_MODEL_REVIEW"}, &cfg.Models.Roles.Review},
		{[]string{"BONSAI_MODEL_ROLE_PATCH", "BONSAI_MODEL_PATCH"}, &cfg.Models.Roles.Patch},
		{[]string{"BONSAI_MODEL_ROLE_CHAT", "BONSAI_MODEL_CHAT"}, &cfg.Models.Roles.Chat},
	}
	for _, b := range defaultBindings {
		if anyEnvSet(b.guards) {
			continue
		}
		*b.dst = v
	}
}

// anyEnvSet returns true if any of the named env vars is non-empty.
func anyEnvSet(names []string) bool {
	for _, n := range names {
		if os.Getenv(n) != "" {
			return true
		}
	}
	return false
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
		{src.Roles.Implement, &dst.Roles.Implement},
		{src.Roles.Plan, &dst.Roles.Plan},
		{src.Roles.Review, &dst.Roles.Review},
		{src.Roles.Patch, &dst.Roles.Patch},
		{src.Roles.Chat, &dst.Roles.Chat},
	}
	for _, r := range roles {
		if r.src != "" {
			*r.dst = r.src
		}
	}
}
