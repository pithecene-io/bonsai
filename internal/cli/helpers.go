package cli

import (
	"github.com/urfave/cli/v2"

	"github.com/pithecene-io/bonsai/internal/agent"
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

// newAgentRouter creates an agent router from config.
func newAgentRouter(cfg *config.Config) *agent.Router {
	var apiOpts []agent.AnthropicOption
	if cfg.Providers.Anthropic.APIKey != "" {
		apiOpts = append(apiOpts, agent.WithAPIKey(cfg.Providers.Anthropic.APIKey))
	}
	return agent.NewRouter(cfg.Agents.Claude.Bin, cfg.Agents.Codex.Bin, apiOpts...)
}

// resolveSkillSet resolves skills from either mode or bundle.
func resolveSkillSet(reg *registry.Registry, mode, bundle string) ([]registry.Skill, string, error) {
	if mode != "" {
		skills, err := reg.SkillsForMode(mode)
		return skills, "mode:" + mode, err
	}
	skills, err := reg.SkillsForBundle(bundle)
	return skills, "bundle:" + bundle, err
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
