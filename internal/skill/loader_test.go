package skill_test

import (
	"testing"

	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/skill"
)

func TestLoad_EmbeddedSkill(t *testing.T) {
	resolver := assets.NewResolver("")

	def, err := skill.Load(resolver, "repo-convention-enforcer", "v1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if def.Name != "repo-convention-enforcer" {
		t.Errorf("Name = %q, want repo-convention-enforcer", def.Name)
	}
	if def.Source != "embedded" {
		t.Errorf("Source = %q, want embedded", def.Source)
	}
	if def.Body == "" {
		t.Error("Body is empty")
	}
	if def.OutputSchema == "" {
		t.Error("OutputSchema is empty")
	}
	if def.InputSchema == "" {
		t.Error("InputSchema is empty")
	}
	if def.Description == "" {
		t.Error("Description is empty (expected from frontmatter)")
	}
}

func TestLoad_NotFound(t *testing.T) {
	resolver := assets.NewResolver("")

	_, err := skill.Load(resolver, "nonexistent-skill", "v1")
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}

func TestLoad_AllEmbeddedSkills(t *testing.T) {
	// Verify every embedded skill can be loaded without error.
	resolver := assets.NewResolver("")

	skills := []string{
		"repo-convention-enforcer",
		"arch-index-alignment",
		"orphan-directory-detector",
		"forbidden-top-level-detector",
		"required-directory-detector",
		"module-name-collision-detector",
	}

	for _, name := range skills {
		t.Run(name, func(t *testing.T) {
			def, err := skill.Load(resolver, name, "v1")
			if err != nil {
				t.Fatalf("Load(%s): %v", name, err)
			}
			if def.Body == "" {
				t.Error("Body is empty")
			}
			if def.OutputSchema == "" {
				t.Error("OutputSchema is empty")
			}
		})
	}
}
