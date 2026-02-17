package diff_test

import (
	"testing"

	"github.com/justapithecus/bonsai/internal/config"
	"github.com/justapithecus/bonsai/internal/diff"
)

func defaultCfg() *config.Config {
	return config.Default()
}

func TestDetermineMode_Heavy_DiffLines(t *testing.T) {
	p := &diff.Profile{DiffLines: 501, FilesChanged: 1}
	mode := diff.DetermineMode(p, defaultCfg(), "")
	if mode != "HEAVY" {
		t.Errorf("got %q, want HEAVY", mode)
	}
}

func TestDetermineMode_Heavy_FilesChanged(t *testing.T) {
	p := &diff.Profile{DiffLines: 100, FilesChanged: 16}
	mode := diff.DetermineMode(p, defaultCfg(), "")
	if mode != "HEAVY" {
		t.Errorf("got %q, want HEAVY", mode)
	}
}

func TestDetermineMode_Heavy_StructuralAndAPI(t *testing.T) {
	p := &diff.Profile{
		DiffLines:          100,
		FilesChanged:       5,
		HasStructural:      true,
		PublicSurfacePaths: []string{"api/v1/handler.go"},
	}
	mode := diff.DetermineMode(p, defaultCfg(), "")
	if mode != "HEAVY" {
		t.Errorf("got %q, want HEAVY", mode)
	}
}

func TestDetermineMode_Structural_TopDirs(t *testing.T) {
	// >1 top-level dirs triggers STRUCTURAL
	p := &diff.Profile{
		DiffLines:    100,
		FilesChanged: 5,
		TopLevelDirs: []string{"src", "docs"},
	}
	mode := diff.DetermineMode(p, defaultCfg(), "")
	if mode != "STRUCTURAL" {
		t.Errorf("got %q, want STRUCTURAL", mode)
	}
}

func TestDetermineMode_Structural_SingleTopDir(t *testing.T) {
	// Exactly 1 top-level dir should NOT trigger STRUCTURAL
	p := &diff.Profile{
		DiffLines:    100,
		FilesChanged: 5,
		TopLevelDirs: []string{"src"},
	}
	mode := diff.DetermineMode(p, defaultCfg(), "")
	if mode == "STRUCTURAL" {
		t.Errorf("got STRUCTURAL, want non-STRUCTURAL (single top dir)")
	}
}

func TestDetermineMode_Structural_Renames(t *testing.T) {
	p := &diff.Profile{
		DiffLines:    100,
		FilesChanged: 5,
		Renames:      1,
		TopLevelDirs: []string{"src"},
	}
	mode := diff.DetermineMode(p, defaultCfg(), "")
	if mode != "STRUCTURAL" {
		t.Errorf("got %q, want STRUCTURAL", mode)
	}
}

func TestDetermineMode_API(t *testing.T) {
	p := &diff.Profile{
		DiffLines:          100,
		FilesChanged:       5,
		TopLevelDirs:       []string{"src"},
		PublicSurfacePaths: []string{"api/v1/handler.go"},
	}
	mode := diff.DetermineMode(p, defaultCfg(), "")
	if mode != "API" {
		t.Errorf("got %q, want API", mode)
	}
}

func TestDetermineMode_Patch_PlanIntent(t *testing.T) {
	p := &diff.Profile{
		DiffLines:    100,
		FilesChanged: 10,
		TopLevelDirs: []string{"src"},
	}
	mode := diff.DetermineMode(p, defaultCfg(), "patch")
	if mode != "PATCH" {
		t.Errorf("got %q, want PATCH", mode)
	}
}

func TestDetermineMode_Patch_SmallDiff(t *testing.T) {
	p := &diff.Profile{
		DiffLines:    50,
		FilesChanged: 2,
		NewFiles:     0,
		Renames:      0,
		TopLevelDirs: []string{"src"},
	}
	mode := diff.DetermineMode(p, defaultCfg(), "")
	if mode != "PATCH" {
		t.Errorf("got %q, want PATCH", mode)
	}
}

func TestDetermineMode_Patch_NewFilesPrevent(t *testing.T) {
	// New files prevent PATCH even with small file count
	p := &diff.Profile{
		DiffLines:    50,
		FilesChanged: 2,
		NewFiles:     1,
		TopLevelDirs: []string{"src"},
	}
	mode := diff.DetermineMode(p, defaultCfg(), "")
	if mode != "NORMAL" {
		t.Errorf("got %q, want NORMAL (new files prevent PATCH)", mode)
	}
}

func TestDetermineMode_Normal_Default(t *testing.T) {
	p := &diff.Profile{
		DiffLines:    100,
		FilesChanged: 5,
		NewFiles:     1,
		TopLevelDirs: []string{"src"},
	}
	mode := diff.DetermineMode(p, defaultCfg(), "")
	if mode != "NORMAL" {
		t.Errorf("got %q, want NORMAL", mode)
	}
}
