// Package config provides configuration resolution with a multi-source
// merge chain: embedded defaults → user config → repo config → env → flags.
package config

// Config is the top-level bonsai configuration.
type Config struct {
	Diff    DiffConfig    `yaml:"diff"`
	Routing RoutingConfig `yaml:"routing"`
	Gate    GateConfig    `yaml:"gate"`
	Check   CheckConfig   `yaml:"check"`
	Agents  AgentsConfig  `yaml:"agents"`
	Output  OutputConfig  `yaml:"output"`
	Skills  SkillsConfig  `yaml:"skills"`
}

// CheckConfig controls the check command.
type CheckConfig struct {
	Concurrency int `yaml:"concurrency"`
}

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

// AgentsConfig holds agent binary paths and model routing.
// Model routing is a bonsai-level concern (not per-agent), so Models
// lives here rather than inside each AgentBinConfig.
type AgentsConfig struct {
	Claude AgentBinConfig `yaml:"claude"`
	Codex  AgentBinConfig `yaml:"codex"`
	Models ModelRouting   `yaml:"models"`
}

// AgentBinConfig holds the path to an agent binary.
type AgentBinConfig struct {
	Bin string `yaml:"bin"`
}

// ModelRouting controls model selection per role and cost tier.
//
// YAML path: agents.models
//
//	agents:
//	  models:
//	    default: sonnet
//	    check:
//	      cheap: haiku
//	      moderate: sonnet
//	      heavy: sonnet
//	    implement: opus
//	    plan: opus
//	    review: sonnet
//	    patch: sonnet
//	    chat: sonnet
type ModelRouting struct {
	Default   string      `yaml:"default"`
	Check     CostModels  `yaml:"check"`
	Implement string      `yaml:"implement"`
	Plan      string      `yaml:"plan"`
	Review    string      `yaml:"review"`
	Patch     string      `yaml:"patch"`
	Chat      string      `yaml:"chat"`
}

// CostModels maps cost tiers to model names within a role.
type CostModels struct {
	Cheap    string `yaml:"cheap"`
	Moderate string `yaml:"moderate"`
	Heavy    string `yaml:"heavy"`
}

// ModelForCheck returns the model for a check skill given its cost tier.
// Falls back to CostModels defaults, then ModelRouting.Default.
func (r ModelRouting) ModelForCheck(cost string) string {
	switch cost {
	case "cheap":
		if r.Check.Cheap != "" {
			return r.Check.Cheap
		}
	case "moderate":
		if r.Check.Moderate != "" {
			return r.Check.Moderate
		}
	case "heavy":
		if r.Check.Heavy != "" {
			return r.Check.Heavy
		}
	}
	return r.Default
}

// ModelForRole returns the model for a given interactive role.
// Falls back to ModelRouting.Default.
func (r ModelRouting) ModelForRole(role string) string {
	var m string
	switch role {
	case "implement":
		m = r.Implement
	case "plan":
		m = r.Plan
	case "review":
		m = r.Review
	case "patch":
		m = r.Patch
	case "chat":
		m = r.Chat
	}
	if m != "" {
		return m
	}
	return r.Default
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
			Concurrency: 4,
		},
		Agents: AgentsConfig{
			Claude: AgentBinConfig{Bin: "claude"},
			Codex:  AgentBinConfig{Bin: "codex"},
			Models: ModelRouting{
				Default: "sonnet",
				Check: CostModels{
					Cheap:    "haiku",
					Moderate: "sonnet",
					Heavy:    "sonnet",
				},
				Implement: "sonnet",
				Plan:      "sonnet",
				Review:    "sonnet",
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
