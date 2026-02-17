package skill

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Output represents the unified output schema for all skills.
type Output struct {
	Skill    string                 `json:"skill"`
	Version  string                 `json:"version"`
	Status   string                 `json:"status"`
	Blocking []string               `json:"blocking"`
	Major    []string               `json:"major"`
	Warning  []string               `json:"warning"`
	Info     []string               `json:"info"`
	Notes    []string               `json:"notes,omitempty"`
	Details  map[string]interface{} `json:"details,omitempty"`
}

// ParseOutput parses and validates a JSON response against the unified
// output schema. This is the defense-in-depth validation matching
// ai-skill.sh lines 333-373.
func ParseOutput(raw string) (*Output, error) {
	// Strip markdown code fences if present
	raw = stripCodeFences(raw)

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty response")
	}

	// Parse as raw JSON first for validation
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &rawMap); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Validate required string fields
	var errors []string
	for _, key := range []string{"skill", "version", "status"} {
		rawVal, ok := rawMap[key]
		if !ok {
			errors = append(errors, fmt.Sprintf("missing required key: %s", key))
			continue
		}
		var s string
		if err := json.Unmarshal(rawVal, &s); err != nil {
			errors = append(errors, fmt.Sprintf("%s: expected string, got %s", key, string(rawVal)))
		}
	}

	// Validate status enum
	if statusRaw, ok := rawMap["status"]; ok {
		var status string
		if err := json.Unmarshal(statusRaw, &status); err == nil {
			if status != "pass" && status != "fail" {
				errors = append(errors, fmt.Sprintf("status: must be \"pass\" or \"fail\", got %q", status))
			}
		}
	}

	// Validate required array fields (must be arrays of strings)
	for _, key := range []string{"blocking", "major", "warning", "info"} {
		rawVal, ok := rawMap[key]
		if !ok {
			errors = append(errors, fmt.Sprintf("missing required key: %s", key))
			continue
		}
		var arr []json.RawMessage
		if err := json.Unmarshal(rawVal, &arr); err != nil {
			errors = append(errors, fmt.Sprintf("%s: expected array, got %s", key, string(rawVal)))
			continue
		}
		for i, elem := range arr {
			var s string
			if err := json.Unmarshal(elem, &s); err != nil {
				errors = append(errors, fmt.Sprintf("%s[%d]: expected string, got %s", key, i, string(elem)))
			}
		}
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("schema validation failed:\n  %s", strings.Join(errors, "\n  "))
	}

	// Parse into struct
	var out Output
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("parse output: %w", err)
	}

	return &out, nil
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
