package cli

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/gitutil"
	"github.com/pithecene-io/bonsai/internal/registry"
)

// detectRepoRoot returns the git repository root or "." as fallback.
func detectRepoRoot() string {
	if gitutil.IsInsideWorkTree(".") {
		if r, err := gitutil.ShowToplevel("."); err == nil {
			return r
		}
	}
	return "."
}

// cmdEnv holds the resolved runtime environment shared by all commands.
// Immutable after construction.
type cmdEnv struct {
	RepoRoot string
	Config   *config.Config
	Resolver *assets.Resolver
	Registry *registry.Registry // nil for commands that don't need skill resolution
}

// bootstrap resolves the full command environment:
// repo root → config → resolver → registry.
func bootstrap() (cmdEnv, error) {
	repoRoot := detectRepoRoot()
	env, err := bootstrapLight(repoRoot)
	if err != nil {
		return cmdEnv{}, err
	}
	reg, err := registry.Load(env.Resolver)
	if err != nil {
		return cmdEnv{}, fmt.Errorf("load registry: %w", err)
	}
	env.Registry = reg
	return env, nil
}

// bootstrapLight resolves the command environment without loading
// the skill registry. Used by interactive commands (chat, plan,
// review, implement) that don't need skill resolution.
func bootstrapLight(repoRoot string) (cmdEnv, error) {
	cfg, err := config.Load(repoRoot)
	if err != nil {
		return cmdEnv{}, fmt.Errorf("load config: %w", err)
	}
	resolver := assets.NewResolver(repoRoot)
	resolver.ExtraSkillDirs = cfg.Skills.ExtraDirs
	return cmdEnv{
		RepoRoot: repoRoot,
		Config:   cfg,
		Resolver: resolver,
	}, nil
}

// newAgentRouter creates an agent router from config.
func newAgentRouter(cfg *config.Config) *agent.Router {
	var apiOpts []agent.AnthropicOption
	if cfg.Providers.Anthropic.APIKey != "" {
		apiOpts = append(apiOpts, agent.WithAPIKey(cfg.Providers.Anthropic.APIKey))
	}
	return agent.NewRouter(cfg.Agents.Claude.Bin, cfg.Agents.Codex.Bin, apiOpts...)
}

// skillSet holds a resolved set of skills and their provenance.
type skillSet struct {
	Skills []registry.Skill
	Source string // e.g. "mode:NORMAL" or "bundle:default"
}

// resolveSkillSet resolves skills from either mode or bundle.
func resolveSkillSet(reg *registry.Registry, mode, bundle string) (skillSet, error) {
	if mode != "" {
		skills, err := reg.SkillsForMode(registry.GovMode(mode))
		return skillSet{Skills: skills, Source: "mode:" + mode}, err
	}
	skills, err := reg.SkillsForBundle(bundle)
	return skillSet{Skills: skills, Source: "bundle:" + bundle}, err
}

// resolveConcurrency resolves concurrency: flag > config > unlimited (0).
func resolveConcurrency(cfg *config.Config, c *cli.Context) int {
	concurrency := 0
	if cfg.Check.Concurrency != nil {
		concurrency = *cfg.Check.Concurrency
	}
	if c.IsSet("jobs") {
		concurrency = c.Int("jobs")
	}
	return concurrency
}

// fileExists checks whether a file exists at the given absolute path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isDirectory checks whether the given absolute path is a directory.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
