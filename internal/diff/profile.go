// Package diff provides diff profiling and mode determination,
// exact ports of compute_diff_profile() and determine_mode() from
// ai-implement.sh.
package diff

import (
	"strings"

	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/gitutil"
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

	// Git commands may fail (e.g. no commits yet, detached HEAD);
	// fall back to empty results so profiling degrades gracefully.
	diffOutput, _ := gitutil.Diff(repoRoot, base)
	nameStatus, _ := gitutil.DiffNameStatus(repoRoot, base)
	diffNames, _ := gitutil.DiffNameOnly(repoRoot, base)
	untracked, _ := gitutil.UntrackedFiles(repoRoot)

	diffNames, nameStatus = mergeUntracked(diffNames, nameStatus, untracked)
	p.FilesChanged = len(diffNames)

	p.countNameStatus(nameStatus)
	p.countDiffLines(diffOutput)
	p.TopLevelDirs = collectTopDirs(diffNames)
	p.PublicSurfacePaths = matchPublicSurface(diffNames, cfg.Routing.PublicSurfaceGlobs)
	p.HasStructural = detectStructural(diffNames, cfg.Routing.StructuralPatterns)

	return p, nil
}

// mergeUntracked deduplicates untracked files into diffNames and appends
// synthetic A\t<name> entries to nameStatus.
func mergeUntracked(diffNames, nameStatus, untracked []string) (merged, status []string) {
	if len(untracked) == 0 {
		return diffNames, nameStatus
	}

	seen := make(map[string]bool, len(diffNames))
	for _, f := range diffNames {
		seen[f] = true
	}
	for _, f := range untracked {
		if !seen[f] {
			diffNames = append(diffNames, f)
			seen[f] = true
		}
		nameStatus = append(nameStatus, "A\t"+f)
	}
	return diffNames, nameStatus
}

// countNameStatus counts new files and renames from name-status output.
func (p *Profile) countNameStatus(nameStatus []string) {
	for _, entry := range nameStatus {
		if strings.HasPrefix(entry, "A") {
			p.NewFiles++
		}
		if strings.HasPrefix(entry, "R") {
			p.Renames++
		}
	}
}

// countDiffLines counts added/removed lines from diff output.
func (p *Profile) countDiffLines(diffOutput string) {
	if diffOutput == "" {
		return
	}
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

// collectTopDirs returns the set of top-level directories touched.
func collectTopDirs(diffNames []string) []string {
	topDirSet := make(map[string]bool)
	for _, f := range diffNames {
		parts := strings.SplitN(f, "/", 2)
		topDirSet[parts[0]] = true
	}
	var dirs []string
	for d := range topDirSet {
		dirs = append(dirs, d)
	}
	return dirs
}

// matchPublicSurface returns files matching any public surface glob.
func matchPublicSurface(diffNames, publicGlobs []string) []string {
	var paths []string
	for _, f := range diffNames {
		for _, glob := range publicGlobs {
			if strings.HasPrefix(f, glob) {
				paths = append(paths, f)
				break
			}
		}
	}
	return paths
}

// detectStructural returns true if any file matches a structural pattern.
func detectStructural(diffNames, patterns []string) bool {
	for _, f := range diffNames {
		for _, pat := range patterns {
			if strings.Contains(f, pat) {
				return true
			}
		}
	}
	return false
}
