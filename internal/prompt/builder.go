// Package prompt provides system prompt assembly matching the 5 prompt
// patterns from the shell scripts.
package prompt

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pithecene-io/bonsai/internal/assets"
)

// Mode represents the operating mode declared in the system prompt.
type Mode string

// Operating modes declared in system prompts.
const (
	ModeArchitect      Mode = "ARCHITECT"
	ModePlanner        Mode = "PLANNER"
	ModeImplementer    Mode = "IMPLEMENTER"
	ModeReviewer       Mode = "REVIEWER"
	ModeValidator      Mode = "VALIDATOR"
	ModePatchArchitect Mode = "PATCH-ARCHITECT"
	ModePatcher        Mode = "PATCHER"
)

// Builder composes system prompts from layered governance documents.
type Builder struct {
	resolver *assets.Resolver
	repoRoot string
}

// NewBuilder creates a prompt builder with the given resolver and repo root.
func NewBuilder(resolver *assets.Resolver, repoRoot string) *Builder {
	return &Builder{
		resolver: resolver,
		repoRoot: repoRoot,
	}
}

// InteractiveOpts holds options for building interactive session prompts.
type InteractiveOpts struct {
	Mode         Mode
	Role         string // Role name (e.g., "architect", "implementer")
	ExtraContext string // Optional findings context for re-entry
}

// BuildInteractive builds a system prompt for interactive sessions
// (chat, plan, implement). Matches the shell pattern:
//
//	preamble + mode + CLAUDE.md + context/*.md + role + AGENTS.md + ARCH_INDEX.md
func (b *Builder) BuildInteractive(opts InteractiveOpts) (string, error) {
	var parts []string

	// Preamble
	parts = append(parts,
		"You are an AI assistant engaged in an interactive technical conversation.",
		"Follow the role definition exactly.",
	)
	if opts.Mode == ModeArchitect {
		parts = append(parts, "Do not write code unless explicitly asked.")
	}
	parts = append(parts, "")
	parts = append(parts, fmt.Sprintf("Repository root: %s", b.repoRoot))
	parts = append(parts, "")

	// Mode declaration
	parts = append(parts, fmt.Sprintf("You are operating in %s mode.", opts.Mode))
	parts = append(parts, "")

	// Global CLAUDE.md (sovereign, always from embedded)
	claudeMD, err := b.resolver.ReadEmbedded("claude.md")
	if err != nil {
		return "", fmt.Errorf("read claude.md: %w", err)
	}
	parts = append(parts, string(claudeMD))
	parts = append(parts, "")

	// Context layers (context/*.md, sorted)
	contextParts, err := b.loadContextLayers()
	if err != nil {
		return "", err
	}
	for _, cp := range contextParts {
		parts = append(parts, cp)
		parts = append(parts, "")
	}

	// Role definition
	roleName := opts.Role
	if roleName == "" {
		roleName = modeDefaultRole(opts.Mode)
	}
	roleData, err := b.resolver.ResolveRoleFile(roleName)
	if err != nil {
		return "", fmt.Errorf("resolve role %q: %w", roleName, err)
	}
	parts = append(parts, string(roleData))

	// AGENTS.md (repo-local)
	if agentsMD := b.readRepoFile("AGENTS.md"); agentsMD != "" {
		parts = append(parts, "")
		parts = append(parts, "Repository context:")
		parts = append(parts, agentsMD)
	}

	// ARCH_INDEX.md (repo-local, check root and docs/)
	if archIndex := b.readArchIndex(); archIndex != "" {
		parts = append(parts, "")
		parts = append(parts, "Repository architecture index:")
		parts = append(parts, archIndex)
	}

	// Extra context (findings from previous governance gate iteration)
	if opts.ExtraContext != "" {
		parts = append(parts, "")
		parts = append(parts, "═══ Previous governance findings (fix these) ═══")
		parts = append(parts, opts.ExtraContext)
	}

	return strings.Join(parts, "\n"), nil
}

// BuildReview builds a system prompt for the review session.
// Same as interactive but adds REVIEW_ARCHITECTURE.md after the role.
func (b *Builder) BuildReview() (string, error) {
	var parts []string

	// Preamble
	parts = append(parts,
		"You are an AI assistant engaged in an interactive technical conversation.",
		"Follow the role definition exactly.",
		"",
		fmt.Sprintf("Repository root: %s", b.repoRoot),
		"",
		fmt.Sprintf("You are operating in %s mode.", ModeReviewer),
		"",
	)

	// Global CLAUDE.md
	claudeMD, err := b.resolver.ReadEmbedded("claude.md")
	if err != nil {
		return "", fmt.Errorf("read claude.md: %w", err)
	}
	parts = append(parts, string(claudeMD), "")

	// Context layers
	contextParts, err := b.loadContextLayers()
	if err != nil {
		return "", err
	}
	for _, cp := range contextParts {
		parts = append(parts, cp, "")
	}

	// Role
	roleData, err := b.resolver.ResolveRoleFile("reviewer")
	if err != nil {
		return "", fmt.Errorf("resolve reviewer role: %w", err)
	}
	parts = append(parts, string(roleData))

	// REVIEW_ARCHITECTURE.md
	reviewArch, err := b.resolver.ReadFile("review_architecture.md")
	if err == nil && len(reviewArch) > 0 {
		parts = append(parts, "", "Review architecture:", string(reviewArch))
	}

	// AGENTS.md
	if agentsMD := b.readRepoFile("AGENTS.md"); agentsMD != "" {
		parts = append(parts, "", "Repository context:", agentsMD)
	}

	// ARCH_INDEX.md
	if archIndex := b.readArchIndex(); archIndex != "" {
		parts = append(parts, "", "Repository architecture index:", archIndex)
	}

	return strings.Join(parts, "\n"), nil
}

// ValidatorOpts holds options for building validator (skill) prompts.
type ValidatorOpts struct {
	SkillBody    string // SKILL.md body (frontmatter stripped)
	OutputSchema string // output.schema.json content
}

// BuildValidator builds a system prompt for skill validation.
// Injection order: Global CLAUDE.md → Repo CLAUDE.md → AGENTS.md → ARCH_INDEX → SKILL.md → schema → suffix
func (b *Builder) BuildValidator(opts ValidatorOpts) (string, error) {
	var parts []string

	// Preamble + mode
	parts = append(parts, "You are operating in VALIDATOR mode.", "")

	// Global CLAUDE.md (sovereign)
	claudeMD, err := b.resolver.ReadEmbedded("claude.md")
	if err != nil {
		return "", fmt.Errorf("read claude.md: %w", err)
	}
	parts = append(parts, string(claudeMD))

	// Repo-local CLAUDE.md (additive)
	if repoClaude := b.readRepoFile("CLAUDE.md"); repoClaude != "" {
		parts = append(parts, "", "Repo-local constitution (CLAUDE.md):", "", repoClaude)
	}

	// AGENTS.md
	if agentsMD := b.readRepoFile("AGENTS.md"); agentsMD != "" {
		parts = append(parts, "", "Repo-local constraints (AGENTS.md):", "", agentsMD)
	}

	// ARCH_INDEX.md
	if archIndex := b.readArchIndex(); archIndex != "" {
		parts = append(parts, "", "Architecture index (docs/ARCH_INDEX.md):", "", archIndex)
	}

	// SKILL.md body
	parts = append(parts, "", opts.SkillBody)

	// Output schema + JSON-only suffix
	parts = append(parts, "",
		"You must output valid JSON conforming exactly to this schema:",
		"",
		opts.OutputSchema,
		"",
		"No markdown. No prose. No explanation. No code fences. JSON only.",
	)

	return strings.Join(parts, "\n"), nil
}

// loadContextLayers reads context/*.md files from the repo, sorted.
func (b *Builder) loadContextLayers() ([]string, error) {
	if b.repoRoot == "" {
		return nil, nil
	}

	ctxDir := filepath.Join(b.repoRoot, "ai", "context")
	entries, err := os.ReadDir(ctxDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var parts []string
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(ctxDir, name))
		if err != nil {
			continue
		}
		parts = append(parts, string(data))
	}

	return parts, nil
}

// readRepoFile reads a file from the repo root. Returns empty string if not found.
func (b *Builder) readRepoFile(name string) string {
	if b.repoRoot == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(b.repoRoot, name))
	if err != nil {
		return ""
	}
	return string(data)
}

// readArchIndex reads ARCH_INDEX.md from root or docs/.
func (b *Builder) readArchIndex() string {
	if b.repoRoot == "" {
		return ""
	}
	for _, rel := range []string{"ARCH_INDEX.md", "docs/ARCH_INDEX.md"} {
		data, err := os.ReadFile(filepath.Join(b.repoRoot, rel))
		if err == nil {
			return string(data)
		}
	}
	return ""
}

// modeDefaultRole returns the default role name for a given mode.
func modeDefaultRole(m Mode) string {
	switch m {
	case ModeArchitect:
		return "architect"
	case ModePlanner:
		return "planner"
	case ModeImplementer:
		return "implementer"
	case ModeReviewer:
		return "reviewer"
	case ModePatchArchitect:
		return "patch-architect"
	case ModePatcher:
		return "patcher"
	default:
		return "architect"
	}
}

// Ensure the fs import is used via loadContextLayers' error handling
var _ fs.DirEntry = nil
