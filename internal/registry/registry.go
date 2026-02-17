// Package registry provides skills.yaml parsing, bundle-based skill
// selection, and mode-based skill selection with cost/mode sorting.
package registry

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/justapithecus/bonsai/internal/assets"
	"gopkg.in/yaml.v3"
)

// Registry holds the parsed skills.yaml content.
type Registry struct {
	Version  int       `yaml:"version"`
	Defaults Defaults  `yaml:"defaults"`
	Skills   []Skill   `yaml:"registry"`
	Bundles  Bundles   `yaml:"bundles"`
}

// Defaults holds default values from the registry.
type Defaults struct {
	SkillVersion  string   `yaml:"skill_version"`
	BundlesFailOn string   `yaml:"bundles_fail_on"`
	CostOrder     []string `yaml:"cost_order"`
	ModeOrder     []string `yaml:"mode_order"`
	RequiresDiff  *bool    `yaml:"requires_diff,omitempty"`
}

// EffectiveRequiresDiff returns the effective requires_diff default.
// When the field is omitted or null, returns true to match the shell
// fallback (ai-check.sh treats missing defaults.requires_diff as true).
func (d *Defaults) EffectiveRequiresDiff() bool {
	if d.RequiresDiff != nil {
		return *d.RequiresDiff
	}
	return true
}

// Skill represents a single skill entry in the registry.
type Skill struct {
	Name         string   `yaml:"name"`
	Version      string   `yaml:"version"`
	Path         string   `yaml:"path"`
	Domain       string   `yaml:"domain"`
	Cost         string   `yaml:"cost"`
	Mode         string   `yaml:"mode"`
	Mandatory    bool     `yaml:"mandatory"`
	Trigger      string   `yaml:"trigger"`
	RequiresDiff *bool    `yaml:"requires_diff,omitempty"`
	RunWhen      RunWhen  `yaml:"run_when"`
}

// RunWhen defines which modes a skill runs in.
type RunWhen struct {
	Modes []string `yaml:"modes"`
}

// Bundles maps bundle names to ordered lists of skill names.
type Bundles map[string][]string

// EffectiveRequiresDiff returns whether this skill requires a diff,
// considering the registry-level default.
func (s *Skill) EffectiveRequiresDiff(defaultVal bool) bool {
	if s.RequiresDiff != nil {
		return *s.RequiresDiff
	}
	return defaultVal
}

// Load loads the skills registry from the resolver.
// It checks repo-local first, then embedded.
func Load(resolver *assets.Resolver) (*Registry, error) {
	data, err := resolver.ReadFile("skills.yaml")
	if err != nil {
		return nil, fmt.Errorf("read skills.yaml: %w", err)
	}

	var reg Registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse skills.yaml: %w", err)
	}

	return &reg, nil
}

// LoadFromFS loads the skills registry from an embed.FS (for testing).
func LoadFromFS(efs fs.FS, path string) (*Registry, error) {
	data, err := fs.ReadFile(efs, path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var reg Registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	return &reg, nil
}

// LoadFromFile loads the skills registry from a filesystem path.
func LoadFromFile(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var reg Registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	return &reg, nil
}

// LookupSkill finds a skill by name in the registry.
func (r *Registry) LookupSkill(name string) (*Skill, bool) {
	for i := range r.Skills {
		if r.Skills[i].Name == name {
			return &r.Skills[i], true
		}
	}
	return nil, false
}

// BundleNames returns all available bundle names.
func (r *Registry) BundleNames() []string {
	var names []string
	for name := range r.Bundles {
		names = append(names, name)
	}
	return names
}

// Ensure unused import is referenced
var _ = filepath.Join
