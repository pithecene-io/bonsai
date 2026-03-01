package skill

import (
	"encoding/json"
	"errors"
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

// validStatuses is the set of allowed status enum values.
var validStatuses = map[string]bool{"pass": true, "fail": true}

// requiredKeys lists all keys that must be present in the JSON object.
var requiredKeys = []string{"skill", "version", "status", "blocking", "major", "warning", "info"}

// ParseOutput parses and validates a JSON response against the unified
// output schema. Unmarshals once into Output, then validates struct
// fields. A lightweight key-presence check catches missing required keys
// that encoding/json silently zero-fills.
func ParseOutput(raw string) (*Output, error) {
	raw = strings.TrimSpace(stripCodeFences(raw))
	if raw == "" {
		return nil, errors.New("empty response")
	}

	// Parse into struct — validates types in a single pass.
	var out Output
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Key-presence check: encoding/json silently zero-fills missing keys,
	// so we verify presence with a lightweight map parse (no per-element work).
	var keys map[string]json.RawMessage
	_ = json.Unmarshal([]byte(raw), &keys) // can't fail — already parsed above
	if errs := validate(&out, keys); len(errs) > 0 {
		return nil, fmt.Errorf("schema validation failed:\n  %s", strings.Join(errs, "\n  "))
	}

	return &out, nil
}

// validate checks required-key presence and enum constraints.
func validate(o *Output, keys map[string]json.RawMessage) []string {
	var errs []string
	for _, key := range requiredKeys {
		if _, ok := keys[key]; !ok {
			errs = append(errs, "missing required key: "+key)
		}
	}
	if o.Status != "" && !validStatuses[o.Status] {
		errs = append(errs, fmt.Sprintf("status: must be \"pass\" or \"fail\", got %q", o.Status))
	}
	return errs
}

// ShouldFail returns true if the output indicates a blocking failure.
// Matches ai-skill.sh exit code logic: exit 1 only if status == "fail"
// AND blocking is non-empty.
func (o *Output) ShouldFail() bool {
	return o.Status == "fail" && len(o.Blocking) > 0
}

// stripCodeFences removes markdown code fence wrappers (```json / ```)
// from JSON responses.
func stripCodeFences(s string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "```" || trimmed == "```json" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	return b.String()
}
