package registry_test

import (
	"testing"

	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/registry"
)

func loadTestRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	r := assets.NewResolver("")
	reg, err := registry.Load(r)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return reg
}

func TestLoadRegistry(t *testing.T) {
	reg := loadTestRegistry(t)

	if reg.Version != 1 {
		t.Errorf("Version = %d, want 1", reg.Version)
	}
	if len(reg.Skills) == 0 {
		t.Error("expected non-empty skills list")
	}
	if len(reg.Bundles) == 0 {
		t.Error("expected non-empty bundles map")
	}
}

func TestLookupSkill(t *testing.T) {
	reg := loadTestRegistry(t)

	s, ok := reg.LookupSkill("repo-convention-enforcer")
	if !ok {
		t.Fatal("expected to find repo-convention-enforcer")
	}
	if s.Version != "v1" {
		t.Errorf("Version = %q", s.Version)
	}
	if !s.Mandatory {
		t.Error("expected mandatory to be true")
	}
	if s.Cost != registry.CostCheap {
		t.Errorf("Cost = %q", s.Cost)
	}
}

func TestLookupSkill_NotFound(t *testing.T) {
	reg := loadTestRegistry(t)
	_, ok := reg.LookupSkill("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestSkillsForBundle_Default(t *testing.T) {
	reg := loadTestRegistry(t)

	skills, err := reg.SkillsForBundle("default")
	if err != nil {
		t.Fatalf("SkillsForBundle: %v", err)
	}
	if len(skills) == 0 {
		t.Error("expected non-empty default bundle")
	}

	// First skill should be repo-convention-enforcer (bundle order)
	if skills[0].Name != "repo-convention-enforcer" {
		t.Errorf("first skill = %q, want repo-convention-enforcer", skills[0].Name)
	}
}

func TestSkillsForBundle_NotFound(t *testing.T) {
	reg := loadTestRegistry(t)

	_, err := reg.SkillsForBundle("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent bundle")
	}
}

func TestSkillsForMode_NORMAL(t *testing.T) {
	reg := loadTestRegistry(t)

	skills, err := reg.SkillsForMode(registry.GovModeNormal)
	if err != nil {
		t.Fatalf("SkillsForMode: %v", err)
	}
	if len(skills) == 0 {
		t.Error("expected non-empty skills for NORMAL mode")
	}

	// Verify sort order: cheap before moderate before heavy
	lastRank := -1
	for _, s := range skills {
		rank := s.Cost.Rank()
		if rank < lastRank {
			t.Errorf("sort order violated: %s (%s) after cost rank %d", s.Name, s.Cost, lastRank)
		}
		lastRank = rank
	}
}

func TestSkillsForMode_AUDIT(t *testing.T) {
	reg := loadTestRegistry(t)

	skills, err := reg.SkillsForMode(registry.GovModeAudit)
	if err != nil {
		t.Fatalf("SkillsForMode: %v", err)
	}

	// AUDIT mode should include all skills
	if len(skills) != len(reg.Skills) {
		t.Errorf("AUDIT skills = %d, want all %d", len(skills), len(reg.Skills))
	}
}

func TestSkillsForMode_Invalid(t *testing.T) {
	reg := loadTestRegistry(t)

	_, err := reg.SkillsForMode("INVALID_MODE")
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestParseGovMode(t *testing.T) {
	tests := []struct {
		mode    string
		wantErr bool
	}{
		{"PATCH", false},
		{"NORMAL", false},
		{"STRUCTURAL", false},
		{"API", false},
		{"HEAVY", false},
		{"AUDIT", false},
		{"INVALID", true},
		{"", true},
	}

	for _, tt := range tests {
		_, err := registry.ParseGovMode(tt.mode)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseGovMode(%q) error = %v, wantErr %v", tt.mode, err, tt.wantErr)
		}
	}
}

func TestEffectiveRequiresDiff(t *testing.T) {
	reg := loadTestRegistry(t)

	// repo-convention-enforcer has requires_diff: false
	s, _ := reg.LookupSkill("repo-convention-enforcer")
	if s.EffectiveRequiresDiff(true) {
		t.Error("repo-convention-enforcer should have requires_diff=false")
	}

	// dependency-layer-violation has no explicit requires_diff (should use default)
	s2, _ := reg.LookupSkill("dependency-layer-violation")
	if !s2.EffectiveRequiresDiff(true) {
		t.Error("dependency-layer-violation should inherit requires_diff=true from defaults")
	}
}

func TestDefaultsEffectiveRequiresDiff(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }

	tests := []struct {
		name string
		val  *bool
		want bool
	}{
		{"nil (omitted) defaults to true", nil, true},
		{"explicit true", boolPtr(true), true},
		{"explicit false", boolPtr(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := registry.Defaults{RequiresDiff: tt.val}
			if got := d.EffectiveRequiresDiff(); got != tt.want {
				t.Errorf("EffectiveRequiresDiff() = %v, want %v", got, tt.want)
			}
		})
	}
}
