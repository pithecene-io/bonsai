// Package config provides configuration resolution with a multi-source
// merge chain: embedded defaults → user config → repo config → env → flags.
package config

// Config is the top-level bonsai configuration.
type Config struct {
	Diff      DiffConfig      `yaml:"diff"`
	Routing   RoutingConfig   `yaml:"routing"`
	Gate      GateConfig      `yaml:"gate"`
	Check     CheckConfig     `yaml:"check"`
	Providers ProvidersConfig `yaml:"providers"`
	Agents    AgentsConfig    `yaml:"agents"`
	Models    ModelsConfig    `yaml:"models"`
	Output    OutputConfig    `yaml:"output"`
	Skills    SkillsConfig    `yaml:"skills"`
}

// CheckConfig controls the check command.
type CheckConfig struct {
	Concurrency *int `yaml:"concurrency"`
}

func intPtr(n int) *int { return &n }

// DiffConfig controls diff profiling thresholds.
type DiffConfig struct {
	HeavyDiffLines    int `yaml:"heavy_diff_lines"`
	HeavyFilesChanged int `yaml:"heavy_files_changed"`
	PatchMaxFiles     int `yaml:"patch_max_files"`
}

// RoutingConfig controls mode determination routing.
type RoutingConfig struct {
	PublicSurfaceGlobs  []string `yaml:"public_surface_globs"`
	StructuralPatterns  []string `yaml:"structural_patterns"`
	MergeBaseCandidates []string `yaml:"merge_base_candidates"`
}

// GateConfig controls the gating loop.
type GateConfig struct {
	MaxIterations int `yaml:"max_iterations"`
}

// ProvidersConfig holds upstream API credentials.
type ProvidersConfig struct {
	Anthropic AnthropicConfig `yaml:"anthropic"`
}

// AnthropicConfig holds direct Anthropic API settings.
// When APIKey is empty, the agent falls back to ANTHROPIC_API_KEY env.
type AnthropicConfig struct {
	APIKey string `yaml:"api_key"`
}

// AgentsConfig holds agent binary paths.
type AgentsConfig struct {
	Claude AgentBinConfig `yaml:"claude"`
	Codex  AgentBinConfig `yaml:"codex"`
}

// AgentBinConfig holds the path to an agent binary.
type AgentBinConfig struct {
	Bin string `yaml:"bin"`
}

// ModelsConfig controls model selection per skill cost tier and role.
//
// YAML path: models
//
//	models:
//	  skills:
//	    cheap: haiku           # fast governance checks
//	    moderate: sonnet       # medium-complexity checks
//	    heavy: opus            # expensive checks
//	  roles:
//	    implement: opus        # feature work
//	    plan: opus             # planning sessions
//	    review: codex          # code review (uses codex agent)
//	    patch: sonnet          # patch surgery
//	    chat: sonnet           # interactive chat
type ModelsConfig struct {
	Skills SkillModels `yaml:"skills"`
	Roles  RoleModels  `yaml:"roles"`
}

// SkillModels maps cost tiers to model names for skill invocations.
type SkillModels struct {
	Cheap    string `yaml:"cheap"`
	Moderate string `yaml:"moderate"`
	Heavy    string `yaml:"heavy"`
}

// RoleModels maps interactive roles to model names.
type RoleModels struct {
	Implement string `yaml:"implement"`
	Plan      string `yaml:"plan"`
	Review    string `yaml:"review"`
	Patch     string `yaml:"patch"`
	Chat      string `yaml:"chat"`
}

// ModelForSkill returns the model for a skill given its cost tier.
// Returns empty string for unknown cost (agent picks its own default).
func (m ModelsConfig) ModelForSkill(cost string) string {
	switch cost {
	case "cheap":
		return m.Skills.Cheap
	case "moderate":
		return m.Skills.Moderate
	case "heavy":
		return m.Skills.Heavy
	}
	return ""
}

// ModelForRole returns the model for a given interactive role.
// Returns empty string for unknown role (agent picks its own default).
func (m ModelsConfig) ModelForRole(role string) string {
	switch role {
	case "implement":
		return m.Roles.Implement
	case "plan":
		return m.Roles.Plan
	case "review":
		return m.Roles.Review
	case "patch":
		return m.Roles.Patch
	case "chat":
		return m.Roles.Chat
	}
	return ""
}

// OutputConfig controls output directory.
type OutputConfig struct {
	Dir string `yaml:"dir"`
}

// SkillsConfig controls additional skill search directories.
type SkillsConfig struct {
	ExtraDirs []string `yaml:"extra_dirs"`
}

// Default returns the default configuration, matching the values
// from the original shell scripts.
func Default() *Config {
	return &Config{
		Diff: DiffConfig{
			HeavyDiffLines:    500,
			HeavyFilesChanged: 15,
			PatchMaxFiles:     3,
		},
		Routing: RoutingConfig{
			PublicSurfaceGlobs: []string{
				"api/",
				"sdk/",
				"public/",
				"cmd/",
				"cli/",
			},
			StructuralPatterns: []string{
				"control/",
				"orchestrator/",
				"state_machine/",
				"persistence/",
				"auth/",
			},
			MergeBaseCandidates: []string{
				"main",
				"master",
				"origin/main",
				"origin/master",
			},
		},
		Gate: GateConfig{
			MaxIterations: 3,
		},
		Check: CheckConfig{
			Concurrency: intPtr(0), // 0 = unlimited (all skills in parallel)
		},
		Agents: AgentsConfig{
			Claude: AgentBinConfig{Bin: "claude"},
			Codex:  AgentBinConfig{Bin: "codex"},
		},
		Models: ModelsConfig{
			Skills: SkillModels{
				Cheap:    "haiku",
				Moderate: "sonnet",
				Heavy:    "sonnet",
			},
			Roles: RoleModels{
				Implement: "opus",
				Plan:      "opus",
				Review:    "codex",
				Patch:     "sonnet",
				Chat:      "sonnet",
			},
		},
		Output: OutputConfig{
			Dir: "ai/out",
		},
		Skills: SkillsConfig{
			ExtraDirs: nil,
		},
	}
}
