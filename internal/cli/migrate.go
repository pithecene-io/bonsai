package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/justapithecus/bonsai/internal/agent"
	"github.com/justapithecus/bonsai/internal/assets"
	"github.com/justapithecus/bonsai/internal/config"
	"github.com/justapithecus/bonsai/internal/orchestrator"
	"github.com/justapithecus/bonsai/internal/registry"
	"github.com/urfave/cli/v2"
)

func migrateCommand() *cli.Command {
	return &cli.Command{
		Name:      "migrate",
		Usage:     "Scaffold AI governance into a repository (6-phase migration)",
		ArgsUsage: "[path]",
		Action:    runMigrate,
	}
}

func runMigrate(c *cli.Context) error {
	target := c.Args().First()
	if target == "" {
		target = "."
	}

	// Validate target is a directory
	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("directory not found: %s", target)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", target)
	}

	// Normalize to absolute path
	target, err = filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	fmt.Println("═══ bonsai migrate ═══")
	fmt.Printf("Target: %s\n\n", target)

	// Load config (from the target repo or defaults)
	cfg, err := config.Load(target)
	if err != nil {
		cfg = config.Default()
	}

	// Create resolver (relative to target)
	resolver := assets.NewResolver(target)

	// Phase A — Scan
	topDirs, languages, existingDocs := phaseAScan(target)

	// Phase B — ARCH_INDEX
	phaseBArchIndex(c, target, cfg)

	// Phase C — CLAUDE.md
	phaseCClaudeMD(target, resolver)

	// Phase D — Scaffolds
	phaseDScaffolds(target, resolver)

	// Phase E — Baselines
	phaseEBaselines(target, topDirs)

	// Phase F — Validate
	phaseFValidate(c, target, cfg, resolver)

	// Summary
	fmt.Println()
	fmt.Println("═══ Migration Complete ═══")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review and customize CLAUDE.md")
	fmt.Println("  2. Review and customize docs/ARCH_INDEX.md")
	fmt.Println("  3. Review the repo-convention-enforcer skill")
	fmt.Println("  4. Run: bonsai check --mode NORMAL")
	fmt.Println("  5. Fix any violations reported")
	fmt.Println("  6. Commit governance artifacts")

	// Suppress unused variable warnings
	_ = languages
	_ = existingDocs

	return nil
}

// phaseAScan scans the target repository.
func phaseAScan(target string) (topDirs []string, languages []string, existingDocs []string) {
	fmt.Println("▶ Phase A: Scanning repository...")

	// Detect top-level directories
	entries, err := os.ReadDir(target)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				topDirs = append(topDirs, e.Name())
			}
		}
		sort.Strings(topDirs)
	}

	if len(topDirs) > 0 {
		fmt.Printf("  Directories: %s\n", strings.Join(topDirs, " "))
	} else {
		fmt.Println("  No subdirectories found")
	}

	// Detect languages/ecosystems
	langDetectors := map[string]string{
		"go.mod":         "go",
		"package.json":   "node",
		"pyproject.toml": "python",
		"Cargo.toml":     "rust",
		"pom.xml":        "java",
		"Gemfile":        "ruby",
	}
	for file, lang := range langDetectors {
		if fileExists(filepath.Join(target, file)) {
			languages = append(languages, lang)
		}
	}
	sort.Strings(languages)

	if len(languages) > 0 {
		fmt.Printf("  Languages: %s\n", strings.Join(languages, " "))
	} else {
		fmt.Println("  Languages: (none detected)")
	}

	// Detect existing docs
	docCandidates := []string{"README.md", "CLAUDE.md", "AGENTS.md", "docs/ARCH_INDEX.md", "ARCH_INDEX.md"}
	for _, doc := range docCandidates {
		if fileExists(filepath.Join(target, doc)) {
			existingDocs = append(existingDocs, doc)
		}
	}

	if len(existingDocs) > 0 {
		fmt.Printf("  Existing docs: %s\n", strings.Join(existingDocs, " "))
	} else {
		fmt.Println("  Existing docs: (none)")
	}

	fmt.Println()
	return topDirs, languages, existingDocs
}

// phaseBArchIndex handles ARCH_INDEX.md creation/review.
func phaseBArchIndex(c *cli.Context, target string, cfg *config.Config) {
	fmt.Println("▶ Phase B: ARCH_INDEX.md")

	// Check candidates
	candidates := []string{
		filepath.Join(target, "docs", "ARCH_INDEX.md"),
		filepath.Join(target, "ARCH_INDEX.md"),
	}

	var archFile string
	for _, candidate := range candidates {
		if fileExists(candidate) {
			archFile = candidate
			break
		}
	}

	if archFile == "" {
		fmt.Println("  No ARCH_INDEX.md found")
		fmt.Println()

		if confirmPrompt("  Create ARCH_INDEX.md interactively with Claude? [Y/n] ", true) {
			claudeAgent := agent.NewClaude(cfg.Agents.Claude.Bin)
			archPrompt := fmt.Sprintf("You are an architect assistant. Examine the repository at %s and create a docs/ARCH_INDEX.md file. The file should be a fast lookup table for agents, summarizing what exists and where.", target)

			if err := os.MkdirAll(filepath.Join(target, "docs"), 0o755); err == nil {
				fmt.Println("  Launching architect session to create ARCH_INDEX.md...")
				fmt.Println()
				_ = claudeAgent.Interactive(c.Context, archPrompt, []string{"-p", "Create docs/ARCH_INDEX.md for this repository. Examine the directory structure and create a navigation index."})
			}

			if !fileExists(filepath.Join(target, "docs", "ARCH_INDEX.md")) {
				// Claude didn't create it — scaffold a minimal one
				scaffoldMinimalArchIndex(target)
			}
		} else {
			fmt.Println("  Skipping ARCH_INDEX.md creation")
			fmt.Println("  WARNING: repository will lack agent orientation without this file")
		}
	} else {
		fmt.Printf("  Found: %s\n", archFile)
		fmt.Println()

		if confirmPrompt("  Review/upgrade ARCH_INDEX.md? [y/N] ", false) {
			claudeAgent := agent.NewClaude(cfg.Agents.Claude.Bin)
			archPrompt := fmt.Sprintf("You are an architect assistant. Review the ARCH_INDEX.md at %s against the actual repository structure at %s. Suggest improvements.", archFile, target)
			_ = claudeAgent.Interactive(c.Context, archPrompt, []string{"-p", "Review and suggest improvements to ARCH_INDEX.md"})
		}
	}

	fmt.Println()
}

// scaffoldMinimalArchIndex creates a minimal ARCH_INDEX.md.
func scaffoldMinimalArchIndex(target string) {
	if err := os.MkdirAll(filepath.Join(target, "docs"), 0o755); err != nil {
		return
	}

	content := `# ARCH_INDEX.md — Architecture Index

This file is a fast lookup table for agents opening this repository.
It summarizes what exists and where, not how things are implemented.

---

## Root

- ` + "`ARCH_INDEX.md`" + ` — this file (or ` + "`docs/ARCH_INDEX.md`" + `)
- ` + "`README.md`" + ` — repository overview
- ` + "`CLAUDE.md`" + ` — repository constitution

---

<!-- Add sections for each top-level directory -->
`

	path := filepath.Join(target, "docs", "ARCH_INDEX.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err == nil {
		fmt.Println("  Scaffolded minimal docs/ARCH_INDEX.md")
	}
}

// phaseCClaudeMD handles CLAUDE.md creation/review.
func phaseCClaudeMD(target string, resolver *assets.Resolver) {
	fmt.Println("▶ Phase C: Repository CLAUDE.md")

	claudeDst := filepath.Join(target, "CLAUDE.md")
	if fileExists(claudeDst) {
		fmt.Println("  CLAUDE.md already exists")
		fmt.Println()

		if confirmPrompt("  Review CLAUDE.md? [y/N] ", false) {
			data, err := os.ReadFile(claudeDst)
			if err == nil {
				lines := strings.Split(string(data), "\n")
				fmt.Println("  --- Current CLAUDE.md (first 20 lines) ---")
				for i, line := range lines {
					if i >= 20 {
						break
					}
					fmt.Printf("  %s\n", line)
				}
				fmt.Println("  ---")
			}
		}
	} else {
		// Try to copy from embedded template
		templateData, err := resolver.ReadEmbedded("templates/migration/CLAUDE.md")
		if err == nil {
			if err := os.WriteFile(claudeDst, templateData, 0o644); err == nil {
				fmt.Println("  Created CLAUDE.md from template")
				fmt.Println("  Review and customize for this repository")
			}
		} else {
			fmt.Fprintln(os.Stderr, "  Migration CLAUDE.md template not found")
			fmt.Fprintln(os.Stderr, "  Create CLAUDE.md manually")
		}
	}

	fmt.Println()
}

// phaseDScaffolds creates directory scaffolds and default artifacts.
func phaseDScaffolds(target string, resolver *assets.Resolver) {
	fmt.Println("▶ Phase D: Directory scaffolds")

	// Create ai/ directories
	dirs := []string{"ai/skills", "ai/baselines", "ai/out"}
	for _, dir := range dirs {
		fullPath := filepath.Join(target, dir)
		if isDirectory(fullPath) {
			fmt.Printf("  %s/ exists\n", dir)
		} else {
			if err := os.MkdirAll(fullPath, 0o755); err == nil {
				fmt.Printf("  Created %s/\n", dir)
			}
		}
	}

	// Create repo-local skill scaffold
	skillDst := filepath.Join(target, "ai", "skills", "repo-convention-enforcer", "v1")
	if isDirectory(skillDst) {
		fmt.Println("  Skill scaffold already exists")
	} else {
		if err := os.MkdirAll(skillDst, 0o755); err == nil {
			skillFiles := []string{"SKILL.md", "input.schema.json", "output.schema.json"}
			for _, f := range skillFiles {
				data, err := resolver.ReadEmbedded(filepath.Join("templates", "migration", "skill", f))
				if err == nil {
					_ = os.WriteFile(filepath.Join(skillDst, f), data, 0o644)
				}
			}
			fmt.Printf("  Created skill scaffold: %s/\n", skillDst)
		}
	}

	// Update .gitignore with ai/out/
	gitignorePath := filepath.Join(target, ".gitignore")
	if fileExists(gitignorePath) {
		data, _ := os.ReadFile(gitignorePath)
		if !strings.Contains(string(data), "ai/out/") {
			f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0o644)
			if err == nil {
				_, _ = f.WriteString("\n# AI governance runtime artifacts\nai/out/\n")
				f.Close()
				fmt.Println("  Added ai/out/ to .gitignore")
			}
		} else {
			fmt.Println("  ai/out/ already in .gitignore")
		}
	} else {
		if err := os.WriteFile(gitignorePath, []byte("ai/out/\n"), 0o644); err == nil {
			fmt.Println("  Created .gitignore with ai/out/")
		}
	}

	fmt.Println()
}

// phaseEBaselines generates optional baseline snapshots.
func phaseEBaselines(target string, topDirs []string) {
	fmt.Println("▶ Phase E: Baselines")

	if !confirmPrompt("  Generate baseline snapshots? [y/N] ", false) {
		fmt.Println("  Skipped")
		fmt.Println()
		return
	}

	baselineDir := filepath.Join(target, "ai", "baselines")
	if err := os.MkdirAll(baselineDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "  Failed to create baselines dir: %v\n", err)
		fmt.Println()
		return
	}

	// Directory listing baseline
	if len(topDirs) > 0 {
		dirListing := strings.Join(topDirs, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(baselineDir, "directories.txt"), []byte(dirListing), 0o644); err == nil {
			fmt.Println("  Saved directory listing to baselines/directories.txt")
		}
	}

	// File count baseline
	count := countFiles(target)
	metricsContent := fmt.Sprintf("total_files: %d\n", count)
	if err := os.WriteFile(filepath.Join(baselineDir, "metrics.yaml"), []byte(metricsContent), 0o644); err == nil {
		fmt.Println("  Saved metrics baseline")
	}

	fmt.Println()
}

// phaseFValidate runs governance validation on the target.
func phaseFValidate(c *cli.Context, target string, cfg *config.Config, resolver *assets.Resolver) {
	fmt.Println("▶ Phase F: Validation")
	fmt.Println()

	reg, err := registry.Load(resolver)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Could not load registry: %v\n", err)
		return
	}

	skills, err := reg.SkillsForBundle("default")
	if err != nil {
		fmt.Fprintf(os.Stderr, "  No default bundle: %v\n", err)
		return
	}

	checkAgent := agent.NewClaude(cfg.Agents.Claude.Bin)
	orch := orchestrator.New(checkAgent, resolver)
	logger := func(msg string) { fmt.Println(msg) }

	report, err := orch.Run(c.Context, orchestrator.RunOpts{
		Skills:   skills,
		Source:   "bundle:default",
		RepoRoot: target,
		Config:   cfg,
	}, logger)

	if err != nil {
		fmt.Fprintf(os.Stderr, "  Validation error: %v\n", err)
		return
	}

	if report.ShouldFail() {
		fmt.Println("  Validation completed with findings (review output above)")
	}
}

// countFiles counts non-hidden files in a directory tree.
func countFiles(root string) int {
	count := 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		// Skip .git
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
