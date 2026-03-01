package registry

import (
	"fmt"
	"sort"
)

// GovMode represents a governance mode determined by diff profiling.
type GovMode string

// Governance modes determined by diff profiling, ordered least to most comprehensive.
const (
	GovModePatch      GovMode = "PATCH"
	GovModeNormal     GovMode = "NORMAL"
	GovModeStructural GovMode = "STRUCTURAL"
	GovModeAPI        GovMode = "API"
	GovModeHeavy      GovMode = "HEAVY"
	GovModeAudit      GovMode = "AUDIT"
)

// govModeSet is the set of valid governance modes.
var govModeSet = map[GovMode]struct{}{
	GovModePatch:      {},
	GovModeNormal:     {},
	GovModeStructural: {},
	GovModeAPI:        {},
	GovModeHeavy:      {},
	GovModeAudit:      {},
}

// ParseGovMode validates and returns a GovMode from a raw string.
func ParseGovMode(s string) (GovMode, error) {
	m := GovMode(s)
	if _, ok := govModeSet[m]; !ok {
		return "", fmt.Errorf("invalid mode %q (valid: %v)", s, ValidModes())
	}
	return m, nil
}

// modeRanks maps analysis modes to sort-order ranks.
var modeRanks = map[string]int{
	"deterministic": 0,
	"heuristic":     1,
	"semantic":      2,
}

// modeRank returns a numeric rank for mode-based sorting.
func modeRank(mode string) int {
	if r, ok := modeRanks[mode]; ok {
		return r
	}
	return 99
}

// SkillsForMode returns skills that match the given mode, sorted by
// cost (cheap→moderate→heavy) then mode (deterministic→heuristic→semantic).
// Matches ai-check.sh mode-based routing.
func (r *Registry) SkillsForMode(mode GovMode) ([]Skill, error) {
	modeName := string(mode)
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
		ci, cj := matched[i].Cost.Rank(), matched[j].Cost.Rank()
		if ci != cj {
			return ci < cj
		}
		return modeRank(matched[i].Mode) < modeRank(matched[j].Mode)
	})

	return matched, nil
}

// ValidModes returns the list of valid governance modes.
func ValidModes() []GovMode {
	return []GovMode{
		GovModePatch, GovModeNormal, GovModeStructural,
		GovModeAPI, GovModeHeavy, GovModeAudit,
	}
}
