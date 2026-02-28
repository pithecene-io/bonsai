package skill

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Output represents the unified output schema for all skills.
type Output struct {
	Skill    string         `json:"skill"`
	Version  string         `json:"version"`
	Status   string         `json:"status"`
	Blocking []string       `json:"blocking"`
	Major    []string       `json:"major"`
	Warning  []string       `json:"warning"`
	Info     []string       `json:"info"`
	Notes    []string       `json:"notes,omitempty"`
	Details  map[string]any `json:"details,omitempty"`
}

// ParseOutput parses and validates a JSON response against the unified
// output schema. This is the defense-in-depth validation matching
// ai-skill.sh lines 333-373.
func ParseOutput(raw string) (*Output, error) {
	raw = strings.TrimSpace(stripCodeFences(raw))
	if raw == "" {
		return nil, fmt.Errorf("empty response")
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &rawMap); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	var errs []string
	errs = validateStringFields(rawMap, errs)
	errs = validateStatusEnum(rawMap, errs)
	errs = validateArrayFields(rawMap, errs)

	if len(errs) > 0 {
		return nil, fmt.Errorf("schema validation failed:\n  %s", strings.Join(errs, "\n  "))
	}

	var out Output
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("parse output: %w", err)
	}

	return &out, nil
}

// validateStringFields checks that required string keys exist and are strings.
func validateStringFields(rawMap map[string]json.RawMessage, errs []string) []string {
	for _, key := range []string{"skill", "version", "status"} {
		rawVal, ok := rawMap[key]
		if !ok {
			errs = append(errs, fmt.Sprintf("missing required key: %s", key))
			continue
		}
		var s string
		if err := json.Unmarshal(rawVal, &s); err != nil {
			errs = append(errs, fmt.Sprintf("%s: expected string, got %s", key, string(rawVal)))
		}
	}
	return errs
}

// validateStatusEnum checks that "status" is "pass" or "fail".
func validateStatusEnum(rawMap map[string]json.RawMessage, errs []string) []string {
	statusRaw, ok := rawMap["status"]
	if !ok {
		return errs
	}
	var status string
	if err := json.Unmarshal(statusRaw, &status); err != nil {
		return errs
	}
	if status != "pass" && status != "fail" {
		errs = append(errs, fmt.Sprintf("status: must be \"pass\" or \"fail\", got %q", status))
	}
	return errs
}

// validateArrayFields checks that required array keys are arrays of strings.
func validateArrayFields(rawMap map[string]json.RawMessage, errs []string) []string {
	for _, key := range []string{"blocking", "major", "warning", "info"} {
		rawVal, ok := rawMap[key]
		if !ok {
			errs = append(errs, fmt.Sprintf("missing required key: %s", key))
			continue
		}
		var arr []json.RawMessage
		if err := json.Unmarshal(rawVal, &arr); err != nil {
			errs = append(errs, fmt.Sprintf("%s: expected array, got %s", key, string(rawVal)))
			continue
		}
		for i, elem := range arr {
			var s string
			if err := json.Unmarshal(elem, &s); err != nil {
				errs = append(errs, fmt.Sprintf("%s[%d]: expected string, got %s", key, i, string(elem)))
			}
		}
	}
	return errs
}

// ShouldFail returns true if the output indicates a blocking failure.
// Matches ai-skill.sh exit code logic: exit 1 only if status == "fail"
// AND blocking is non-empty.
func (o *Output) ShouldFail() bool {
	return o.Status == "fail" && len(o.Blocking) > 0
}

// stripCodeFences removes markdown code fence wrappers from JSON responses.
// Matches: sed '/^```\(json\)\{0,1\}$/d'
func stripCodeFences(s string) string {
	lines := strings.Split(s, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "```" || trimmed == "```json" {
			continue
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}
