package prompt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/prompt"
)

func TestBuildInteractive_Architect(t *testing.T) {
	r := assets.NewResolver("")
	b := prompt.NewBuilder(r, "/tmp/test-repo")

	result, err := b.BuildInteractive(prompt.InteractiveOpts{
		Mode: prompt.ModeArchitect,
		Role: "architect",
	})
	if err != nil {
		t.Fatalf("BuildInteractive: %v", err)
	}

	assertContains(t, result, "ARCHITECT mode")
	assertContains(t, result, "Repository root: /tmp/test-repo")
	assertContains(t, result, "CLAUDE.md")
	assertContains(t, result, "Do not write code unless explicitly asked.")
}

func TestBuildInteractive_Implementer(t *testing.T) {
	r := assets.NewResolver("")
	b := prompt.NewBuilder(r, "/tmp/test-repo")

	result, err := b.BuildInteractive(prompt.InteractiveOpts{
		Mode: prompt.ModeImplementer,
		Role: "implementer",
	})
	if err != nil {
		t.Fatalf("BuildInteractive: %v", err)
	}

	assertContains(t, result, "IMPLEMENTER mode")
	// Implementer mode should NOT have "Do not write code" preamble
	if strings.Contains(result, "Do not write code unless explicitly asked.") {
		t.Error("implementer mode should not have architect preamble")
	}
}

func TestBuildInteractive_WithExtraContext(t *testing.T) {
	r := assets.NewResolver("")
	b := prompt.NewBuilder(r, "/tmp/test-repo")

	findings := "SKILL: repo-convention-enforcer | blocking: 2 | major: 0 | warning: 1"
	result, err := b.BuildInteractive(prompt.InteractiveOpts{
		Mode:         prompt.ModeImplementer,
		ExtraContext: findings,
	})
	if err != nil {
		t.Fatalf("BuildInteractive: %v", err)
	}

	assertContains(t, result, "Previous governance findings (fix these)")
	assertContains(t, result, findings)
}

func TestBuildInteractive_WithRepoFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Test AGENTS"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "ARCH_INDEX.md"), []byte("# Arch Index"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	r := assets.NewResolver(dir)
	b := prompt.NewBuilder(r, dir)

	result, err := b.BuildInteractive(prompt.InteractiveOpts{
		Mode: prompt.ModeArchitect,
	})
	if err != nil {
		t.Fatalf("BuildInteractive: %v", err)
	}

	assertContains(t, result, "Repository context:")
	assertContains(t, result, "# Test AGENTS")
	assertContains(t, result, "Repository architecture index:")
	assertContains(t, result, "# Arch Index")
}

func TestBuildReview(t *testing.T) {
	r := assets.NewResolver("")
	b := prompt.NewBuilder(r, "/tmp/test-repo")

	result, err := b.BuildReview()
	if err != nil {
		t.Fatalf("BuildReview: %v", err)
	}

	assertContains(t, result, "REVIEWER mode")
	assertContains(t, result, "Review architecture:")
}

func TestBuildValidator(t *testing.T) {
	r := assets.NewResolver("")
	b := prompt.NewBuilder(r, "/tmp/test-repo")

	result, err := b.BuildValidator(prompt.ValidatorOpts{
		SkillBody:    "Check for convention violations.",
		OutputSchema: `{"type": "object"}`,
	})
	if err != nil {
		t.Fatalf("BuildValidator: %v", err)
	}

	assertContains(t, result, "VALIDATOR mode")
	assertContains(t, result, "Check for convention violations.")
	assertContains(t, result, `{"type": "object"}`)
	assertContains(t, result, "No markdown. No prose. No explanation. No code fences. JSON only.")
}

func TestBuildValidator_WithRepoClaude(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Repo Constitution"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	r := assets.NewResolver(dir)
	b := prompt.NewBuilder(r, dir)

	result, err := b.BuildValidator(prompt.ValidatorOpts{
		SkillBody:    "Test skill body",
		OutputSchema: "{}",
	})
	if err != nil {
		t.Fatalf("BuildValidator: %v", err)
	}

	assertContains(t, result, "Repo-local constitution (CLAUDE.md):")
	assertContains(t, result, "# Repo Constitution")
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected prompt to contain %q", needle)
	}
}
