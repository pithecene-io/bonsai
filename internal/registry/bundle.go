package registry

// SkillsForBundle returns the ordered list of skills for a bundle.
// Bundle listing order is authoritative — the bundle author controls
// execution sequence intentionally.
func (r *Registry) SkillsForBundle(bundleName string) ([]Skill, error) {
	names, ok := r.Bundles[bundleName]
	if !ok {
		return nil, &BundleNotFoundError{Name: bundleName, Available: r.BundleNames()}
	}

	var skills []Skill
	for _, name := range names {
		s, ok := r.LookupSkill(name)
		if !ok {
			// Skill in bundle but not in registry — skip with warning.
			// This allows repo-local bundles to reference repo-local skills.
			continue
		}
		skills = append(skills, *s)
	}

	return skills, nil
}

// BundleNotFoundError is returned when a bundle is not found.
type BundleNotFoundError struct {
	Name      string
	Available []string
}

func (e *BundleNotFoundError) Error() string {
	return "bundle not found: " + e.Name
}
