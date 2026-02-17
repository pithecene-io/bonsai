package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/justapithecus/bonsai/internal/agent"
	"github.com/justapithecus/bonsai/internal/assets"
	"github.com/justapithecus/bonsai/internal/config"
	"github.com/justapithecus/bonsai/internal/gitutil"
	"github.com/justapithecus/bonsai/internal/prompt"
	"github.com/justapithecus/bonsai/internal/registry"
	"github.com/justapithecus/bonsai/internal/repo"
	"github.com/justapithecus/bonsai/internal/skill"
	"github.com/urfave/cli/v2"
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

	// Build diff payload
	var diffPayload string
	if baseRef != "" && gitutil.IsInsideWorkTree(repoRoot) {
		diff, err := gitutil.Diff(repoRoot, baseRef)
		if err == nil {
			diffPayload = diff
		}

		// Append synthetic diffs for untracked files
		untracked, _ := gitutil.UntrackedFiles(repoRoot)
		if len(untracked) > 0 {
			diffPayload += buildSyntheticUntrackedDiff(repoRoot, untracked)
		}
	}

	// Create agent and builder
	claudeAgent := agent.NewClaude(cfg.Agents.Claude.Bin)
	builder := prompt.NewBuilder(resolver, repoRoot)
	runner := skill.NewRunner(claudeAgent, builder)

	// Run skill
	output, err := runner.Run(context.Background(), def, skill.RunOpts{
		RepoTree:    strings.Join(repoTree, "\n"),
		DiffPayload: diffPayload,
		BaseRef:     baseRef,
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

// buildSyntheticUntrackedDiff creates fake unified diff headers for
// untracked files. Matches ai-skill.sh lines 264-285.
func buildSyntheticUntrackedDiff(repoRoot string, files []string) string {
	var buf strings.Builder
	for _, f := range files {
		fullPath := filepath.Join(repoRoot, f)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		// Detect file mode
		fmode := "100644"
		if info.Mode()&0o111 != 0 {
			fmode = "100755"
		}

		buf.WriteString(fmt.Sprintf("\ndiff --git a/%s b/%s\n", f, f))
		buf.WriteString(fmt.Sprintf("new file mode %s\n", fmode))
		buf.WriteString(fmt.Sprintf("--- /dev/null\n"))
		buf.WriteString(fmt.Sprintf("+++ b/%s\n", f))

		// Read file content for diff body
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		lines := strings.Split(string(content), "\n")
		buf.WriteString(fmt.Sprintf("@@ -0,0 +1,%d @@\n", len(lines)))
		for _, line := range lines {
			buf.WriteString("+" + line + "\n")
		}
	}
	return buf.String()
}
