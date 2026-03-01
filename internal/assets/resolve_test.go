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

func TestReadFile_FallsBackToEmbedded(t *testing.T) {
	// With no repo root, ReadFile should fall back to embedded assets
	r := assets.NewResolver("")
	data, err := r.ReadFile("claude.md")
	if err != nil {
		t.Fatalf("ReadFile(claude.md): %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty claude.md from embedded")
	}
}

func TestReadFile_RepoLocalOverride(t *testing.T) {
	dir := t.TempDir()
	aiDir := filepath.Join(dir, "ai")
	if err := os.MkdirAll(aiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aiDir, "claude.md"), []byte("# Custom"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := assets.NewResolver(dir)
	data, err := r.ReadFile("claude.md")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "# Custom" {
		t.Errorf("expected repo-local override, got %q", string(data))
	}
}

func TestReadFile_NotFound(t *testing.T) {
	r := assets.NewResolver("")
	_, err := r.ReadFile("nonexistent-file-that-does-not-exist.xyz")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadEmbedded_NotFound(t *testing.T) {
	r := assets.NewResolver("")
	_, err := r.ReadEmbedded("nonexistent.md")
	if err == nil {
		t.Error("expected error for nonexistent embedded file")
	}
}

func TestResolveSkillDir_ExtraDirs(t *testing.T) {
	extraDir := t.TempDir()
	skillDir := filepath.Join(extraDir, "custom-skill", "v1")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	r := &assets.Resolver{
		ExtraSkillDirs: []string{extraDir},
	}
	fsPath, embedPath, err := r.ResolveSkillDir("custom-skill", "v1")
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

func TestResolveSkillDir_UserConfigOverride(t *testing.T) {
	userDir := t.TempDir()
	skillDir := filepath.Join(userDir, "skills", "my-skill", "v1")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	r := &assets.Resolver{
		UserConfigDir: userDir,
	}
	fsPath, embedPath, err := r.ResolveSkillDir("my-skill", "v1")
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

func TestResolveSkillDir_PriorityOrder(t *testing.T) {
	// Repo-local should win over extra dirs and user config
	repoDir := t.TempDir()
	extraDir := t.TempDir()
	userDir := t.TempDir()

	name, version := "priority-test", "v1"

	// Create skill in all three locations
	repoSkillDir := filepath.Join(repoDir, "ai", "skills", name, version)
	extraSkillDir := filepath.Join(extraDir, name, version)
	userSkillDir := filepath.Join(userDir, "skills", name, version)
	for _, d := range []string{repoSkillDir, extraSkillDir, userSkillDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	r := &assets.Resolver{
		RepoRoot:       repoDir,
		UserConfigDir:  userDir,
		ExtraSkillDirs: []string{extraDir},
	}

	fsPath, _, err := r.ResolveSkillDir(name, version)
	if err != nil {
		t.Fatalf("ResolveSkillDir: %v", err)
	}
	if fsPath != repoSkillDir {
		t.Errorf("expected repo-local to win, got %q", fsPath)
	}
}

func TestResolveRoleFile_UserConfigOverride(t *testing.T) {
	userDir := t.TempDir()
	roleDir := filepath.Join(userDir, "roles")
	if err := os.MkdirAll(roleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(roleDir, "reviewer.md"), []byte("# User reviewer"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &assets.Resolver{
		UserConfigDir: userDir,
	}
	data, err := r.ResolveRoleFile("reviewer")
	if err != nil {
		t.Fatalf("ResolveRoleFile: %v", err)
	}
	if string(data) != "# User reviewer" {
		t.Errorf("expected user config override, got %q", string(data))
	}
}

func TestResolveRoleFile_NotFound(t *testing.T) {
	r := assets.NewResolver("")
	_, err := r.ResolveRoleFile("nonexistent-role")
	if err == nil {
		t.Error("expected error for nonexistent role")
	}
}

func TestResolveRoleFile_RepoWinsOverUser(t *testing.T) {
	repoDir := t.TempDir()
	userDir := t.TempDir()

	// Create in both locations
	repoRoleDir := filepath.Join(repoDir, "ai", "roles")
	userRoleDir := filepath.Join(userDir, "roles")
	for _, d := range []string{repoRoleDir, userRoleDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repoRoleDir, "test.md"), []byte("repo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userRoleDir, "test.md"), []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &assets.Resolver{
		RepoRoot:      repoDir,
		UserConfigDir: userDir,
	}
	data, err := r.ResolveRoleFile("test")
	if err != nil {
		t.Fatalf("ResolveRoleFile: %v", err)
	}
	if string(data) != "repo" {
		t.Errorf("expected repo to win over user, got %q", string(data))
	}
}

func TestNewResolver_EmptyRepoRoot(t *testing.T) {
	r := assets.NewResolver("")
	if r.RepoRoot != "" {
		t.Errorf("RepoRoot = %q, want empty", r.RepoRoot)
	}
}
