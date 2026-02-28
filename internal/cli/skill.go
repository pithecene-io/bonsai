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

	repoRoot := detectRepoRoot()
	cfg, err := config.Load(repoRoot)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	resolver := assets.NewResolver(repoRoot)
	resolver.ExtraSkillDirs = cfg.Skills.ExtraDirs

	reg, err := registry.Load(resolver)
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	def, err := loadSkillDef(resolver, reg, skillName, c.String("version"))
	if err != nil {
		return err
	}

	scope := c.String("scope")
	repoTree, err := repo.TreeWithScope(repoRoot, scope)
	if err != nil {
		return fmt.Errorf("repo tree: %w", err)
	}
	if len(repoTree) == 0 {
		return fmt.Errorf("scope produced empty repo tree")
	}

	baseRef := c.String("base")
	// Diff payload is best-effort; runs without diff context on error.
	diffPayload, _ := skill.BuildDiffPayload(repoRoot, baseRef)

	runner := skill.NewRunner(newAgentRouter(cfg), prompt.NewBuilder(resolver, repoRoot))
	model := resolveSkillModel(c.String("model"), reg, cfg, skillName)

	output, err := runner.Run(context.Background(), def, skill.RunOpts{
		RepoTree:    strings.Join(repoTree, "\n"),
		DiffPayload: diffPayload,
		BaseRef:     baseRef,
		Model:       agent.Model(model),
	})
	if err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		return err
	}

	if output.ShouldFail() {
		os.Exit(1)
	}
	return nil
}

// loadSkillDef resolves the skill version from the registry (if not overridden)
// and loads the skill definition.
func loadSkillDef(resolver *assets.Resolver, reg *registry.Registry, name, version string) (*skill.Definition, error) {
	if version == "" {
		s, ok := reg.LookupSkill(name)
		if !ok {
			return nil, fmt.Errorf("skill %q not in registry; use --version to specify", name)
		}
		version = s.Version
	}
	def, err := skill.Load(resolver, name, version)
	if err != nil {
		return nil, fmt.Errorf("load skill: %w", err)
	}
	return def, nil
}

// resolveSkillModel returns the model for a skill: explicit flag > config routing.
func resolveSkillModel(override string, reg *registry.Registry, cfg *config.Config, name string) string {
	if override != "" {
		return override
	}
	if s, ok := reg.LookupSkill(name); ok {
		return cfg.Models.ModelForSkill(s.Cost)
	}
	return ""
}
