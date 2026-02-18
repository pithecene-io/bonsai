// Package config provides configuration resolution with a multi-source
// merge chain: embedded defaults → user config → repo config → env → flags.
package config

// Config is the top-level bonsai configuration.
type Config struct {
	Diff    DiffConfig    `yaml:"diff"`
	Routing RoutingConfig `yaml:"routing"`
	Gate    GateConfig    `yaml:"gate"`
	Agents  AgentsConfig  `yaml:"agents"`
	Output  OutputConfig  `yaml:"output"`
	Skills  SkillsConfig  `yaml:"skills"`
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

// AgentsConfig holds agent binary paths.
type AgentsConfig struct {
	Claude AgentBinConfig `yaml:"claude"`
	Codex  AgentBinConfig `yaml:"codex"`
}

// AgentBinConfig holds the path to an agent binary.
type AgentBinConfig struct {
	Bin string `yaml:"bin"`
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
		Agents: AgentsConfig{
			Claude: AgentBinConfig{Bin: "claude"},
			Codex:  AgentBinConfig{Bin: "codex"},
		},
		Output: OutputConfig{
			Dir: "ai/out",
		},
		Skills: SkillsConfig{
			ExtraDirs: nil,
		},
	}
}
