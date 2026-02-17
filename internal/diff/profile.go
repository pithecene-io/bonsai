// Package diff provides diff profiling and mode determination,
// exact ports of compute_diff_profile() and determine_mode() from
// ai-implement.sh.
package diff

import (
	"strings"

	"github.com/justapithecus/bonsai/internal/config"
	"github.com/justapithecus/bonsai/internal/gitutil"
)

// Profile holds the diff profile computed from repository changes.
// This is an exact port of compute_diff_profile() from ai-implement.sh.
type Profile struct {
	FilesChanged       int      `json:"files_changed"`
	NewFiles           int      `json:"new_files"`
	Renames            int      `json:"renames"`
	LinesAdded         int      `json:"lines_added"`
	LinesRemoved       int      `json:"lines_removed"`
	DiffLines          int      `json:"diff_lines"`
	TopLevelDirs       []string `json:"top_level_dirs"`
	PublicSurfacePaths []string `json:"public_surface_paths"`
	HasStructural      bool     `json:"has_structural"`
}

// ComputeProfile computes a diff profile from the given base ref.
// This is a faithful port of compute_diff_profile() from ai-implement.sh.
//
// Key fidelity point: includes untracked files in both files_changed
// and new_files counts. Untracked files get synthetic A\t<name> entries.
func ComputeProfile(repoRoot, base string, cfg *config.Config) (*Profile, error) {
	p := &Profile{}

	// Get diff output
	diffOutput, _ := gitutil.Diff(repoRoot, base)

	// Get name-status
	nameStatus, _ := gitutil.DiffNameStatus(repoRoot, base)

	// Get diff names
	diffNames, _ := gitutil.DiffNameOnly(repoRoot, base)

	// Include untracked files (git diff never shows these)
	untracked, _ := gitutil.UntrackedFiles(repoRoot)
	if len(untracked) > 0 {
		// Merge untracked into diffNames (dedup)
		seen := make(map[string]bool, len(diffNames))
		for _, f := range diffNames {
			seen[f] = true
		}
		for _, f := range untracked {
			if !seen[f] {
				diffNames = append(diffNames, f)
				seen[f] = true
			}
		}

		// Append to nameStatus as additions
		for _, f := range untracked {
			nameStatus = append(nameStatus, "A\t"+f)
		}
	}

	// Count files changed
	p.FilesChanged = len(diffNames)

	// Count new files and renames from name-status
	for _, entry := range nameStatus {
		if strings.HasPrefix(entry, "A") {
			p.NewFiles++
		}
		if strings.HasPrefix(entry, "R") {
			p.Renames++
		}
	}

	// Count lines from diff output
	if diffOutput != "" {
		for _, line := range strings.Split(diffOutput, "\n") {
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "++") {
				p.LinesAdded++
			}
			if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "--") {
				p.LinesRemoved++
			}
		}
		p.DiffLines = len(strings.Split(diffOutput, "\n"))
	}

	// Top-level directories touched
	topDirSet := make(map[string]bool)
	for _, f := range diffNames {
		parts := strings.SplitN(f, "/", 2)
		topDirSet[parts[0]] = true
	}
	for d := range topDirSet {
		p.TopLevelDirs = append(p.TopLevelDirs, d)
	}

	// Public surface paths
	publicGlobs := cfg.Routing.PublicSurfaceGlobs
	for _, f := range diffNames {
		for _, glob := range publicGlobs {
			if strings.HasPrefix(f, glob) {
				p.PublicSurfacePaths = append(p.PublicSurfacePaths, f)
				break
			}
		}
	}

	// Structural detection
	structuralPatterns := cfg.Routing.StructuralPatterns
	for _, f := range diffNames {
		for _, pat := range structuralPatterns {
			if strings.Contains(f, pat) {
				p.HasStructural = true
				break
			}
		}
		if p.HasStructural {
			break
		}
	}

	return p, nil
}
