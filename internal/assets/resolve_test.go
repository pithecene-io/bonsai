package assets_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pithecene-io/bonsai/internal/assets"
)

func TestReadEmbeddedClaudeMD(t *testing.T) {
	r := assets.NewResolver("")
	data, err := r.ReadEmbedded("claude.md")
	if err != nil {
		t.Fatalf("ReadEmbedded(claude.md): %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty claude.md")
	}
}

func TestReadEmbeddedRole(t *testing.T) {
	r := assets.NewResolver("")
	data, err := r.ResolveRoleFile("architect")
	if err != nil {
		t.Fatalf("ResolveRoleFile(architect): %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty architect role")
	}
}

func TestReadEmbeddedSkillsYAML(t *testing.T) {
	r := assets.NewResolver("")
	data, err := r.ReadEmbedded("skills.yaml")
	if err != nil {
		t.Fatalf("ReadEmbedded(skills.yaml): %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty skills.yaml")
	}
}

func TestResolveSkillDir_Embedded(t *testing.T) {
	r := assets.NewResolver("")
	fsPath, embedPath, err := r.ResolveSkillDir("repo-convention-enforcer", "v1")
	if err != nil {
		t.Fatalf("ResolveSkillDir: %v", err)
	}
	if fsPath != "" {
		t.Errorf("expected empty fsPath, got %q", fsPath)
	}
	if embedPath == "" {
		t.Error("expected non-empty embedPath")
	}
}

func TestResolveSkillDir_RepoLocalOverride(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "ai", "skills", "repo-convention-enforcer", "v1")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write SKILL.md to make it a valid skill dir
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# test"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	r := assets.NewResolver(dir)
	fsPath, embedPath, err := r.ResolveSkillDir("repo-convention-enforcer", "v1")
	if err != nil {
		t.Fatalf("ResolveSkillDir: %v", err)
	}
	if fsPath != skillDir {
		t.Errorf("fsPath = %q, want %q", fsPath, skillDir)
	}
	if embedPath != "" {
		t.Errorf("expected empty embedPath, got %q", embedPath)
	}
}

func TestResolveRoleFile_Filesystem(t *testing.T) {
	dir := t.TempDir()
	roleDir := filepath.Join(dir, "ai", "roles")
	if err := os.MkdirAll(roleDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(roleDir, "architect.md"), []byte("# Custom architect"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	r := assets.NewResolver(dir)
	data, err := r.ResolveRoleFile("architect")
	if err != nil {
		t.Fatalf("ResolveRoleFile: %v", err)
	}
	if string(data) != "# Custom architect" {
		t.Errorf("got %q, want custom architect content", string(data))
	}
}

func TestResolveSkillDir_NotFound(t *testing.T) {
	r := assets.NewResolver("")
	_, _, err := r.ResolveSkillDir("nonexistent-skill", "v99")
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}
