package diff

import "github.com/justapithecus/bonsai/internal/config"

// DetermineMode determines the governance mode from a diff profile.
// This is a faithful port of determine_mode() from ai-implement.sh.
//
// Precedence cascade:
//  1. HEAVY if diff_lines > 500 OR files > 15 OR (structural AND api)
//  2. STRUCTURAL if top-level dirs > 1 OR renames > 0
//  3. API if public surface paths touched
//  4. PATCH if plan says patch OR (≤3 files, no new files, no renames)
//  5. NORMAL (default)
func DetermineMode(profile *Profile, cfg *config.Config, planIntent string) string {
	// 1. HEAVY
	if profile.DiffLines > cfg.Diff.HeavyDiffLines || profile.FilesChanged > cfg.Diff.HeavyFilesChanged {
		return "HEAVY"
	}
	if profile.HasStructural && len(profile.PublicSurfacePaths) > 0 {
		return "HEAVY"
	}

	// 2. STRUCTURAL if top-level dirs changed (>1) OR renames/moves
	if len(profile.TopLevelDirs) > 1 || profile.Renames > 0 {
		return "STRUCTURAL"
	}

	// 3. API if public surface paths touched
	if len(profile.PublicSurfacePaths) > 0 {
		return "API"
	}

	// 4. PATCH if plan says patch OR (≤3 files, no new files, no renames)
	if planIntent == "patch" {
		return "PATCH"
	}
	if profile.FilesChanged <= cfg.Diff.PatchMaxFiles && profile.NewFiles == 0 && profile.Renames == 0 {
		return "PATCH"
	}

	// 5. NORMAL (default)
	return "NORMAL"
}
