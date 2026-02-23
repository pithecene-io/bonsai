package diff_test

import (
	"testing"

	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/diff"
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

// --- Exact boundary threshold tests ---

func TestDetermineMode_Heavy_ExactThreshold_DiffLines(t *testing.T) {
	// 500 lines is the threshold (default). Exactly 500 should NOT trigger HEAVY.
	p := &diff.Profile{DiffLines: 500, FilesChanged: 1, TopLevelDirs: []string{"src"}}
	mode := diff.DetermineMode(p, defaultCfg(), "")
	if mode == "HEAVY" {
		t.Errorf("got HEAVY at exact threshold (500 lines); want non-HEAVY")
	}
}

func TestDetermineMode_Heavy_ExactThreshold_FilesChanged(t *testing.T) {
	// 15 files is the threshold (default). Exactly 15 should NOT trigger HEAVY.
	p := &diff.Profile{DiffLines: 100, FilesChanged: 15, TopLevelDirs: []string{"src"}}
	mode := diff.DetermineMode(p, defaultCfg(), "")
	if mode == "HEAVY" {
		t.Errorf("got HEAVY at exact threshold (15 files); want non-HEAVY")
	}
}

func TestDetermineMode_Patch_ExactThreshold_MaxFiles(t *testing.T) {
	// 3 files is PatchMaxFiles (default). Exactly 3 should qualify for PATCH.
	p := &diff.Profile{
		DiffLines:    50,
		FilesChanged: 3,
		NewFiles:     0,
		Renames:      0,
		TopLevelDirs: []string{"src"},
	}
	mode := diff.DetermineMode(p, defaultCfg(), "")
	if mode != "PATCH" {
		t.Errorf("got %q at PatchMaxFiles boundary (3 files); want PATCH", mode)
	}
}

func TestDetermineMode_Patch_OverMaxFiles(t *testing.T) {
	// 4 files exceeds PatchMaxFiles (default 3). Should NOT be PATCH.
	p := &diff.Profile{
		DiffLines:    50,
		FilesChanged: 4,
		NewFiles:     0,
		Renames:      0,
		TopLevelDirs: []string{"src"},
	}
	mode := diff.DetermineMode(p, defaultCfg(), "")
	if mode == "PATCH" {
		t.Errorf("got PATCH at 4 files (PatchMaxFiles=3); want non-PATCH")
	}
}

func TestDetermineMode_NewFilesOnly(t *testing.T) {
	// Only new (untracked) files, no tracked changes.
	// DiffLines=0, 1 dir, no API, no structural.
	// NewFiles>0 prevents PATCH → NORMAL.
	p := &diff.Profile{
		DiffLines:    0,
		FilesChanged: 2,
		NewFiles:     2,
		Renames:      0,
		TopLevelDirs: []string{"src"},
	}
	mode := diff.DetermineMode(p, defaultCfg(), "")
	if mode != "NORMAL" {
		t.Errorf("got %q for new-files-only; want NORMAL", mode)
	}
}

func TestDetermineMode_RenameOnly(t *testing.T) {
	// A single rename with no other signals → STRUCTURAL.
	p := &diff.Profile{
		DiffLines:    10,
		FilesChanged: 1,
		NewFiles:     0,
		Renames:      1,
		TopLevelDirs: []string{"src"},
	}
	mode := diff.DetermineMode(p, defaultCfg(), "")
	if mode != "STRUCTURAL" {
		t.Errorf("got %q for rename-only; want STRUCTURAL", mode)
	}
}

func TestDetermineMode_LargeSingleFile(t *testing.T) {
	// 1 file with 600 diff lines → HEAVY regardless of other signals.
	p := &diff.Profile{
		DiffLines:    600,
		FilesChanged: 1,
		NewFiles:     0,
		Renames:      0,
		TopLevelDirs: []string{"src"},
	}
	mode := diff.DetermineMode(p, defaultCfg(), "")
	if mode != "HEAVY" {
		t.Errorf("got %q for large single file (600 lines); want HEAVY", mode)
	}
}
