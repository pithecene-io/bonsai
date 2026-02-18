// Package orchestrator provides multi-skill execution, fail-fast logic,
// all-skipped detection, and aggregate JSON report generation.
// Faithful port of ai-check.sh.
package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/prompt"
	"github.com/pithecene-io/bonsai/internal/registry"
	"github.com/pithecene-io/bonsai/internal/repo"
	"github.com/pithecene-io/bonsai/internal/skill"
)

// RunOpts configures an orchestrator run.
type RunOpts struct {
	Skills              []registry.Skill // Ordered list of skills to run
	Source              string           // "mode:NORMAL" or "bundle:default"
	BaseRef             string           // Git ref for diff context
	Scope               string           // Comma-separated path prefixes
	FailFast            bool             // Stop on first mandatory failure
	RepoRoot            string           // Repository root
	Config              *config.Config
	DefaultRequiresDiff bool // Registry defaults.requires_diff value
}

// Result holds the outcome of a single skill invocation.
type Result struct {
	Name          string `json:"name"`
	Status        string `json:"status"`
	SkippedReason string `json:"skipped_reason,omitempty"`
	Blocking      int    `json:"blocking"`
	Major         int    `json:"major"`
	Warning       int    `json:"warning"`
	ExitCode      int    `json:"exit_code"`
	Mandatory     bool   `json:"mandatory"`
}

// Report holds the aggregate orchestrator output.
type Report struct {
	Source         string   `json:"source"`
	Timestamp      string   `json:"timestamp"`
	Total          int      `json:"total"`
	Passed         int      `json:"passed"`
	Failed         int      `json:"failed"`
	Skipped        int      `json:"skipped"`
	BlockingFailed int      `json:"blocking_failed"`
	Results        []Result `json:"results"`
}

// Orchestrator runs a set of skills and aggregates results.
type Orchestrator struct {
	agent    agent.Agent
	resolver *assets.Resolver
}

// New creates an orchestrator.
func New(a agent.Agent, resolver *assets.Resolver) *Orchestrator {
	return &Orchestrator{agent: a, resolver: resolver}
}

// Run executes the skill set and returns an aggregate report.
// Logger is called for each skill with status updates.
func (o *Orchestrator) Run(ctx context.Context, opts RunOpts, logger func(string)) (*Report, error) {
	timestamp := time.Now().Format("20060102-150405")

	report := &Report{
		Source:    opts.Source,
		Timestamp: timestamp,
	}

	builder := prompt.NewBuilder(o.resolver, opts.RepoRoot)
	runner := skill.NewRunner(o.agent, builder)

	// Build repo tree
	repoTree, err := repo.TreeWithScope(opts.RepoRoot, opts.Scope)
	if err != nil {
		return nil, fmt.Errorf("repo tree: %w", err)
	}
	repoTreeStr := joinLines(repoTree)

	// Build diff payload
	var diffPayload string
	if opts.BaseRef != "" {
		diffPayload = buildDiffPayload(opts.RepoRoot, opts.BaseRef)
	}

	for i := range opts.Skills {
		s := opts.Skills[i]
		report.Total++

		// Check if skill requires diff and no base provided.
		// Uses registry defaults.requires_diff (ref: ai-check.sh:166-168).
		requiresDiff := s.EffectiveRequiresDiff(opts.DefaultRequiresDiff)
		if requiresDiff && opts.BaseRef == "" {
			report.Skipped++
			result := Result{
				Name:          s.Name,
				Status:        "skipped",
				SkippedReason: "requires_diff without --base",
				Mandatory:     s.Mandatory,
			}
			report.Results = append(report.Results, result)
			if logger != nil {
				logger(fmt.Sprintf("  ⊘ %s [skipped: requires --base for diff context]", s.Name))
			}
			continue
		}

		if logger != nil {
			logger(fmt.Sprintf("▶ Running: %s [%s]", s.Name, s.Cost))
		}

		// Load skill definition
		version := s.Version
		if version == "" {
			version = "v1"
		}
		def, err := skill.Load(o.resolver, s.Name, version)
		if err != nil {
			// Skill load failure — treat as error
			report.Failed++
			result := Result{
				Name:      s.Name,
				Status:    "error",
				ExitCode:  1,
				Mandatory: s.Mandatory,
			}
			if s.Mandatory {
				report.BlockingFailed++
			}
			report.Results = append(report.Results, result)
			if logger != nil {
				logger(fmt.Sprintf("  ✖ %s [error: %v]", s.Name, err))
			}
			if opts.FailFast && s.Mandatory {
				if logger != nil {
					logger(fmt.Sprintf("✖ Mandatory failure (--fail-fast): %s", s.Name))
				}
				break
			}
			continue
		}

		// Run skill
		output, err := runner.Run(ctx, def, skill.RunOpts{
			RepoTree:    repoTreeStr,
			DiffPayload: diffPayload,
			BaseRef:     opts.BaseRef,
		})

		var result Result
		if err != nil {
			report.Failed++
			result = Result{
				Name:      s.Name,
				Status:    "error",
				ExitCode:  1,
				Mandatory: s.Mandatory,
			}
			if s.Mandatory {
				report.BlockingFailed++
			}
			if logger != nil {
				logger(fmt.Sprintf("  ✖ %s [error: %v]", s.Name, err))
			}
		} else {
			exitCode := 0
			if output.ShouldFail() {
				exitCode = 1
			}

			result = Result{
				Name:      s.Name,
				Status:    output.Status,
				Blocking:  len(output.Blocking),
				Major:     len(output.Major),
				Warning:   len(output.Warning),
				ExitCode:  exitCode,
				Mandatory: s.Mandatory,
			}

			if exitCode == 0 {
				report.Passed++
				if logger != nil {
					logger(fmt.Sprintf("  ✔ %s (blocking:%d major:%d warning:%d)",
						s.Name, len(output.Blocking), len(output.Major), len(output.Warning)))
				}
			} else {
				report.Failed++
				if s.Mandatory {
					report.BlockingFailed++
					if logger != nil {
						logger(fmt.Sprintf("  ✖ %s [mandatory] (blocking:%d major:%d warning:%d)",
							s.Name, len(output.Blocking), len(output.Major), len(output.Warning)))
					}
				} else if logger != nil {
					logger(fmt.Sprintf("  ⚠ %s [non-mandatory] (blocking:%d major:%d warning:%d)",
						s.Name, len(output.Blocking), len(output.Major), len(output.Warning)))
				}
			}
		}

		report.Results = append(report.Results, result)

		if opts.FailFast && result.ExitCode != 0 && s.Mandatory {
			if logger != nil {
				logger(fmt.Sprintf("✖ Mandatory failure (--fail-fast): %s", s.Name))
			}
			break
		}
	}

	return report, nil
}

// ShouldFail returns true if the report indicates a blocking failure.
// Matches ai-check.sh exit logic:
//   - exit 1 if all skills were skipped (no validation occurred)
//   - exit 1 if blocking_failed > 0
func (r *Report) ShouldFail() bool {
	if r.Total > 0 && r.Skipped == r.Total {
		return true // All skipped = false pass
	}
	return r.BlockingFailed > 0
}

func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}

func buildDiffPayload(repoRoot, baseRef string) string {
	// This reuses the same logic as the skill command
	diff, _ := skill.BuildDiffPayload(repoRoot, baseRef)
	return diff
}
