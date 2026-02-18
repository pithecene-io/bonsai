package registry

import (
	"fmt"
	"sort"
)

// costRank returns a numeric rank for cost-based sorting.
// Matches: {"cheap":0,"moderate":1,"heavy":2}
func costRank(cost string) int {
	switch cost {
	case "cheap":
		return 0
	case "moderate":
		return 1
	case "heavy":
		return 2
	default:
		return 99
	}
}

// modeRank returns a numeric rank for mode-based sorting.
// Matches: {"deterministic":0,"heuristic":1,"semantic":2}
func modeRank(mode string) int {
	switch mode {
	case "deterministic":
		return 0
	case "heuristic":
		return 1
	case "semantic":
		return 2
	default:
		return 99
	}
}

// SkillsForMode returns skills that match the given mode, sorted by
// cost (cheap→moderate→heavy) then mode (deterministic→heuristic→semantic).
// Matches ai-check.sh mode-based routing.
func (r *Registry) SkillsForMode(modeName string) ([]Skill, error) {
	var matched []Skill

	for i := range r.Skills {
		for _, m := range r.Skills[i].RunWhen.Modes {
			if m == modeName {
				matched = append(matched, r.Skills[i])
				break
			}
		}
	}

	if len(matched) == 0 {
		return nil, fmt.Errorf("no skills matched mode %q", modeName)
	}

	// Sort: cost first, then mode within same cost
	sort.SliceStable(matched, func(i, j int) bool {
		ci, cj := costRank(matched[i].Cost), costRank(matched[j].Cost)
		if ci != cj {
			return ci < cj
		}
		return modeRank(matched[i].Mode) < modeRank(matched[j].Mode)
	})

	return matched, nil
}

// ValidModes returns the list of valid diff modes.
func ValidModes() []string {
	return []string{"PATCH", "NORMAL", "STRUCTURAL", "API", "HEAVY", "AUDIT"}
}

// IsValidMode returns true if the given mode string is valid.
func IsValidMode(mode string) bool {
	for _, m := range ValidModes() {
		if m == mode {
			return true
		}
	}
	return false
}
