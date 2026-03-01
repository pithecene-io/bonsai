package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	urfave "github.com/urfave/cli/v2"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/orchestrator"
	"github.com/pithecene-io/bonsai/internal/registry"
)

func migrateCommand() *urfave.Command {
	return &urfave.Command{
		Name:      "migrate",
		Usage:     "Scaffold AI governance into a repository (6-phase migration)",
		ArgsUsage: "[path]",
		Action:    runMigrate,
	}
}

// migration encapsulates the state and operations for scaffolding
// AI governance into a target repository.
type migration struct {
	target   string
	config   *config.Config
	resolver *assets.Resolver
	scan     scanResult
}

// scanResult holds the outcomes of the repository scan phase.
type scanResult struct {
	topDirs   []string
	languages []string
	docs      []string
}

func runMigrate(c *urfave.Context) error {
	target := c.Args().First()
	if target == "" {
		target = "."
	}

	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("directory not found: %s", target)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", target)
	}

	target, err = filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	fmt.Println("═══ bonsai migrate ═══")
	fmt.Printf("Target: %s\n\n", target)

	cfg, err := config.Load(target)
	if err != nil {
		cfg = config.Default()
	}

	m := &migration{
		target:   target,
		config:   cfg,
		resolver: assets.NewResolver(target),
	}

	m.scanRepo()
	m.ensureArchIndex(c)
	m.ensureConstitution()
	m.scaffold()
	m.baseline()
	m.validate(c)

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

	return nil
}

// scanRepo inventories the target repository structure.
func (m *migration) scanRepo() {
	fmt.Println("▶ Scanning repository...")

	m.scan.topDirs = m.topDirs()
	m.printList("  Directories", m.scan.topDirs, "No subdirectories found")

	m.scan.languages = m.languages()
	m.printList("  Languages", m.scan.languages, "(none detected)")

	m.scan.docs = m.existingDocs()
	m.printList("  Existing docs", m.scan.docs, "(none)")

	fmt.Println()
}

// langIndicators maps manifest files to language names.
var langIndicators = map[string]string{
	"go.mod":         "go",
	"package.json":   "node",
	"pyproject.toml": "python",
	"Cargo.toml":     "rust",
	"pom.xml":        "java",
	"Gemfile":        "ruby",
}

func (m *migration) topDirs() []string {
	entries, err := os.ReadDir(m.target)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	return dirs
}

func (m *migration) languages() []string {
	var langs []string
	for file, lang := range langIndicators {
		if m.fileExists(file) {
			langs = append(langs, lang)
		}
	}
	sort.Strings(langs)
	return langs
}

func (m *migration) existingDocs() []string {
	candidates := []string{"README.md", "CLAUDE.md", "AGENTS.md", "docs/ARCH_INDEX.md", "ARCH_INDEX.md"}
	var found []string
	for _, doc := range candidates {
		if m.fileExists(doc) {
			found = append(found, doc)
		}
	}
	return found
}

func (m *migration) printList(label string, items []string, fallback string) {
	if len(items) > 0 {
		fmt.Printf("%s: %s\n", label, strings.Join(items, " "))
	} else {
		fmt.Printf("%s: %s\n", label, fallback)
	}
}

// ensureArchIndex creates or reviews docs/ARCH_INDEX.md.
func (m *migration) ensureArchIndex(c *urfave.Context) {
	fmt.Println("▶ ARCH_INDEX.md")

	if path := m.findArchIndex(); path != "" {
		fmt.Printf("  Found: %s\n", path)
		fmt.Println()
		if confirmPrompt("  Review/upgrade ARCH_INDEX.md? [y/N] ", false) {
			claudeAgent := agent.NewClaude(m.config.Agents.Claude.Bin)
			archPrompt := fmt.Sprintf("You are an architect assistant. Review the ARCH_INDEX.md at %s against the actual repository structure at %s. Suggest improvements.", path, m.target)
			_ = claudeAgent.Interactive(c.Context, archPrompt, []string{"-p", "Review and suggest improvements to ARCH_INDEX.md"})
		}
	} else {
		fmt.Println("  No ARCH_INDEX.md found")
		fmt.Println()
		if confirmPrompt("  Create ARCH_INDEX.md interactively with Claude? [Y/n] ", true) {
			m.createArchIndex(c.Context)
		} else {
			fmt.Println("  Skipping ARCH_INDEX.md creation")
			fmt.Println("  WARNING: repository will lack agent orientation without this file")
		}
	}

	fmt.Println()
}

func (m *migration) findArchIndex() string {
	for _, rel := range []string{"docs/ARCH_INDEX.md", "ARCH_INDEX.md"} {
		if m.fileExists(rel) {
			return filepath.Join(m.target, rel)
		}
	}
	return ""
}

func (m *migration) createArchIndex(ctx context.Context) {
	docsDir := filepath.Join(m.target, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		return
	}

	claudeAgent := agent.NewClaude(m.config.Agents.Claude.Bin)
	archPrompt := fmt.Sprintf("You are an architect assistant. Examine the repository at %s and create a docs/ARCH_INDEX.md file. The file should be a fast lookup table for agents, summarizing what exists and where.", m.target)

	fmt.Println("  Launching architect session to create ARCH_INDEX.md...")
	fmt.Println()
	_ = claudeAgent.Interactive(ctx, archPrompt, []string{"-p", "Create docs/ARCH_INDEX.md for this repository. Examine the directory structure and create a navigation index."})

	if !m.fileExists("docs/ARCH_INDEX.md") {
		m.scaffoldMinimalArchIndex()
	}
}

func (m *migration) scaffoldMinimalArchIndex() {
	docsDir := filepath.Join(m.target, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
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

	path := filepath.Join(docsDir, "ARCH_INDEX.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err == nil {
		fmt.Println("  Scaffolded minimal docs/ARCH_INDEX.md")
	}
}

// ensureConstitution creates or reviews CLAUDE.md.
func (m *migration) ensureConstitution() {
	fmt.Println("▶ Repository CLAUDE.md")

	claudePath := filepath.Join(m.target, "CLAUDE.md")
	if m.fileExists("CLAUDE.md") {
		fmt.Println("  CLAUDE.md already exists")
		fmt.Println()
		if confirmPrompt("  Review CLAUDE.md? [y/N] ", false) {
			m.previewFile(claudePath, 20)
		}
	} else {
		templateData, err := m.resolver.ReadEmbedded("templates/migration/CLAUDE.md")
		if err != nil {
			fmt.Fprintln(os.Stderr, "  Migration CLAUDE.md template not found")
			fmt.Fprintln(os.Stderr, "  Create CLAUDE.md manually")
		} else if err := os.WriteFile(claudePath, templateData, 0o644); err == nil {
			fmt.Println("  Created CLAUDE.md from template")
			fmt.Println("  Review and customize for this repository")
		}
	}

	fmt.Println()
}

func (m *migration) previewFile(path string, maxLines int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	fmt.Println("  --- Current CLAUDE.md (first 20 lines) ---")
	for i, line := range lines {
		if i >= maxLines {
			break
		}
		fmt.Printf("  %s\n", line)
	}
	fmt.Println("  ---")
}

// scaffold creates directory structure and default skill artifacts.
func (m *migration) scaffold() {
	fmt.Println("▶ Directory scaffolds")

	for _, dir := range []string{"ai/skills", "ai/baselines", "ai/out"} {
		fullPath := filepath.Join(m.target, dir)
		if m.isDir(fullPath) {
			fmt.Printf("  %s/ exists\n", dir)
		} else if err := os.MkdirAll(fullPath, 0o755); err == nil {
			fmt.Printf("  Created %s/\n", dir)
		}
	}

	m.scaffoldSkill()
	m.ensureGitignore()

	fmt.Println()
}

func (m *migration) scaffoldSkill() {
	skillDst := filepath.Join(m.target, "ai", "skills", "repo-convention-enforcer", "v1")
	if m.isDir(skillDst) {
		fmt.Println("  Skill scaffold already exists")
		return
	}
	if err := os.MkdirAll(skillDst, 0o755); err != nil {
		return
	}
	for _, f := range []string{"SKILL.md", "input.schema.json", "output.schema.json"} {
		data, err := m.resolver.ReadEmbedded(filepath.Join("templates", "migration", "skill", f))
		if err == nil {
			_ = os.WriteFile(filepath.Join(skillDst, f), data, 0o644)
		}
	}
	fmt.Printf("  Created skill scaffold: %s/\n", skillDst)
}

func (m *migration) ensureGitignore() {
	gitignorePath := filepath.Join(m.target, ".gitignore")
	if !m.fileExists(".gitignore") {
		if err := os.WriteFile(gitignorePath, []byte("ai/out/\n"), 0o644); err == nil {
			fmt.Println("  Created .gitignore with ai/out/")
		}
		return
	}
	data, _ := os.ReadFile(gitignorePath)
	if strings.Contains(string(data), "ai/out/") {
		fmt.Println("  ai/out/ already in .gitignore")
		return
	}
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err == nil {
		_, _ = f.WriteString("\n# AI governance runtime artifacts\nai/out/\n")
		_ = f.Close()
		fmt.Println("  Added ai/out/ to .gitignore")
	}
}

// baseline generates optional baseline snapshots.
func (m *migration) baseline() {
	fmt.Println("▶ Baselines")

	if !confirmPrompt("  Generate baseline snapshots? [y/N] ", false) {
		fmt.Println("  Skipped")
		fmt.Println()
		return
	}

	baselineDir := filepath.Join(m.target, "ai", "baselines")
	if err := os.MkdirAll(baselineDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "  Failed to create baselines dir: %v\n", err)
		fmt.Println()
		return
	}

	if len(m.scan.topDirs) > 0 {
		dirListing := strings.Join(m.scan.topDirs, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(baselineDir, "directories.txt"), []byte(dirListing), 0o644); err == nil {
			fmt.Println("  Saved directory listing to baselines/directories.txt")
		}
	}

	count := m.countFiles()
	metricsContent := fmt.Sprintf("total_files: %d\n", count)
	if err := os.WriteFile(filepath.Join(baselineDir, "metrics.yaml"), []byte(metricsContent), 0o644); err == nil {
		fmt.Println("  Saved metrics baseline")
	}

	fmt.Println()
}

// validate runs governance validation on the scaffolded repository.
func (m *migration) validate(c *urfave.Context) {
	fmt.Println("▶ Validation")
	fmt.Println()

	reg, err := registry.Load(m.resolver)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Could not load registry: %v\n", err)
		return
	}

	skills, err := reg.SkillsForBundle("default")
	if err != nil {
		fmt.Fprintf(os.Stderr, "  No default bundle: %v\n", err)
		return
	}

	checkRouter := agent.NewRouter(m.config.Agents.Claude.Bin, m.config.Agents.Codex.Bin)
	orch := orchestrator.New(checkRouter, m.resolver)

	valCtx, cancel := context.WithTimeout(c.Context, 2*time.Minute)
	defer cancel()

	report, err := orch.RunWithLogger(valCtx, orchestrator.RunOpts{
		Skills:              skills,
		Source:              "bundle:default",
		RepoRoot:            m.target,
		Config:              m.config,
		DefaultRequiresDiff: reg.Defaults.EffectiveRequiresDiff(),
		Concurrency:         1,
	}, nil)
	if err != nil {
		if valCtx.Err() == context.DeadlineExceeded {
			fmt.Fprintln(os.Stderr, "  ⚠ Validation timed out (2m) — skipping (advisory only)")
		} else {
			fmt.Fprintf(os.Stderr, "  Validation error: %v\n", err)
		}
		return
	}

	if report.ShouldFail() {
		fmt.Println("  Validation completed with findings (review output above)")
	}
}

// --- filesystem helpers ---

func (m *migration) fileExists(rel string) bool {
	return fileExists(filepath.Join(m.target, rel))
}

func (m *migration) isDir(abs string) bool {
	return isDirectory(abs)
}

func (m *migration) countFiles() int {
	count := 0
	_ = filepath.WalkDir(m.target, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // WalkDir: skip unreadable entries
		}
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
