package assets

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Resolver resolves asset files with filesystem-first override semantics.
// Resolution order per asset type:
//
//	Skills: repo-local ai/skills/ → user ~/.config/bonsai/skills/ → embedded
//	Roles:  repo-local ai/roles/  → user ~/.config/bonsai/roles/  → embedded
//	Global CLAUDE.md: always embedded (sovereign, cannot be overridden)
//	Skills registry: embedded baseline, repo-local can add but not remove
type Resolver struct {
	// RepoRoot is the repository root directory (may be empty).
	RepoRoot string

	// UserConfigDir is the user config directory (e.g., ~/.config/bonsai).
	// If empty, defaults to $XDG_CONFIG_HOME/bonsai or ~/.config/bonsai.
	UserConfigDir string

	// ExtraSkillDirs are additional directories to search for skills.
	ExtraSkillDirs []string
}

// NewResolver creates a Resolver with default user config directory.
func NewResolver(repoRoot string) *Resolver {
	return &Resolver{
		RepoRoot:      repoRoot,
		UserConfigDir: defaultUserConfigDir(),
	}
}

func defaultUserConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "bonsai")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "bonsai")
}

// ReadFile resolves and reads a file. It checks filesystem override
// locations first, then falls back to embedded assets.
func (r *Resolver) ReadFile(name string) ([]byte, error) {
	// Try filesystem overrides
	for _, dir := range r.overrideDirs() {
		path := filepath.Join(dir, name)
		if data, err := os.ReadFile(path); err == nil {
			return data, nil
		}
	}

	// Fall back to embedded
	return fs.ReadFile(embeddedFS, filepath.Join("data", name))
}

// ReadEmbedded reads a file only from embedded assets, ignoring overrides.
// Used for sovereign files like the global CLAUDE.md.
func (r *Resolver) ReadEmbedded(name string) ([]byte, error) {
	return fs.ReadFile(embeddedFS, filepath.Join("data", name))
}

// ResolveSkillDir finds a skill directory with filesystem-first precedence:
//
//  1. repo-local: <repo>/ai/skills/<name>/<version>/
//  2. extra dirs: <dir>/<name>/<version>/
//  3. user config: ~/.config/bonsai/skills/<name>/<version>/
//  4. embedded: data/skills/<name>/<version>/
//
// Returns the path (filesystem) or empty string + embedded FS sub-path.
func (r *Resolver) ResolveSkillDir(name, version string) (fsPath, embedPath string, err error) {
	rel := filepath.Join("skills", name, version)

	// 1. Repo-local
	if r.RepoRoot != "" {
		p := filepath.Join(r.RepoRoot, "ai", rel)
		if isDir(p) {
			return p, "", nil
		}
	}

	// 2. Extra dirs
	for _, dir := range r.ExtraSkillDirs {
		p := filepath.Join(dir, name, version)
		if isDir(p) {
			return p, "", nil
		}
	}

	// 3. User config
	if r.UserConfigDir != "" {
		p := filepath.Join(r.UserConfigDir, rel)
		if isDir(p) {
			return p, "", nil
		}
	}

	// 4. Embedded
	embedRel := filepath.Join("data", rel)
	if _, err := fs.Stat(embeddedFS, embedRel); err == nil {
		return "", embedRel, nil
	}

	return "", "", fmt.Errorf("skill not found: %s/%s", name, version)
}

// ResolveRoleFile finds a role definition file with filesystem-first precedence:
//
//  1. repo-local: <repo>/ai/roles/<name>.md
//  2. user config: ~/.config/bonsai/roles/<name>.md
//  3. embedded: data/roles/<name>.md
func (r *Resolver) ResolveRoleFile(name string) ([]byte, error) {
	rel := filepath.Join("roles", name+".md")

	// 1. Repo-local
	if r.RepoRoot != "" {
		p := filepath.Join(r.RepoRoot, "ai", rel)
		if data, err := os.ReadFile(p); err == nil {
			return data, nil
		}
	}

	// 2. User config
	if r.UserConfigDir != "" {
		p := filepath.Join(r.UserConfigDir, rel)
		if data, err := os.ReadFile(p); err == nil {
			return data, nil
		}
	}

	// 3. Embedded
	return fs.ReadFile(embeddedFS, filepath.Join("data", rel))
}

// overrideDirs returns filesystem directories to check before embedded,
// in priority order (highest first).
func (r *Resolver) overrideDirs() []string {
	var dirs []string
	if r.RepoRoot != "" {
		dirs = append(dirs, filepath.Join(r.RepoRoot, "ai"))
	}
	if r.UserConfigDir != "" {
		dirs = append(dirs, r.UserConfigDir)
	}
	return dirs
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
