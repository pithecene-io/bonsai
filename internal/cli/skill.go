package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/gitutil"
	"github.com/pithecene-io/bonsai/internal/prompt"
	"github.com/pithecene-io/bonsai/internal/registry"
	"github.com/pithecene-io/bonsai/internal/repo"
	"github.com/pithecene-io/bonsai/internal/skill"
)

func skillCommand() *cli.Command {
	return &cli.Command{
		Name:      "skill",
		Usage:     "Run a single governance skill",
		ArgsUsage: "<skill-name>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "version", Usage: "Skill version override"},
			&cli.StringFlag{Name: "scope", Usage: "Comma-separated path prefixes to filter repo tree"},
			&cli.StringFlag{Name: "base", Usage: "Git ref for diff context"},
			&cli.StringFlag{Name: "model", Usage: "Model override (e.g. haiku, sonnet, opus)"},
		},
		Action: runSkill,
	}
}

func runSkill(c *cli.Context) error {
	skillName := c.Args().First()
	if skillName == "" {
		return fmt.Errorf("usage: bonsai skill <skill-name> [--version vX] [--scope path1,path2] [--base <ref>]")
	}

	skillVersion := c.String("version")
	scope := c.String("scope")
	baseRef := c.String("base")
	modelOverride := c.String("model")

	// Detect repo
	repoRoot := "."
	if gitutil.IsInsideWorkTree(".") {
		if r, err := gitutil.ShowToplevel("."); err == nil {
			repoRoot = r
		}
	}

	// Load config
	cfg, err := config.Load(repoRoot)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Create resolver
	resolver := assets.NewResolver(repoRoot)
	resolver.ExtraSkillDirs = cfg.Skills.ExtraDirs

	// Load registry to resolve version if not overridden
	reg, err := registry.Load(resolver)
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	if skillVersion == "" {
		s, ok := reg.LookupSkill(skillName)
		if !ok {
			return fmt.Errorf("skill %q not in registry; use --version to specify", skillName)
		}
		skillVersion = s.Version
	}

	// Load skill definition
	def, err := skill.Load(resolver, skillName, skillVersion)
	if err != nil {
		return fmt.Errorf("load skill: %w", err)
	}

	// Build repo tree
	repoTree, err := repo.TreeWithScope(repoRoot, scope)
	if err != nil {
		return fmt.Errorf("repo tree: %w", err)
	}
	if len(repoTree) == 0 {
		return fmt.Errorf("scope produced empty repo tree")
	}

	// Diff payload is best-effort; runs without diff context on error.
	diffPayload, _ := skill.BuildDiffPayload(repoRoot, baseRef)

	// Create agent router (routes to codex/anthropic/claude based on model)
	var apiOpts []agent.AnthropicOption
	if cfg.Providers.Anthropic.APIKey != "" {
		apiOpts = append(apiOpts, agent.WithAPIKey(cfg.Providers.Anthropic.APIKey))
	}
	agentRouter := agent.NewRouter(cfg.Agents.Claude.Bin, cfg.Agents.Codex.Bin, apiOpts...)
	builder := prompt.NewBuilder(resolver, repoRoot)
	runner := skill.NewRunner(agentRouter, builder)

	// Resolve model: explicit flag > config routing by cost tier
	model := modelOverride
	if model == "" {
		if s, ok := reg.LookupSkill(skillName); ok {
			model = cfg.Models.ModelForSkill(s.Cost)
		}
	}

	// Run skill
	output, err := runner.Run(context.Background(), def, skill.RunOpts{
		RepoTree:    strings.Join(repoTree, "\n"),
		DiffPayload: diffPayload,
		BaseRef:     baseRef,
		Model:       agent.Model(model),
	})
	if err != nil {
		return err
	}

	// Print output as formatted JSON
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		return err
	}

	// Exit code: fail if status=fail AND blocking is non-empty
	if output.ShouldFail() {
		os.Exit(1)
	}

	return nil
}
