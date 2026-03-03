package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/pithecene-io/bonsai/internal/agent"
	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/gitutil"
	"github.com/pithecene-io/bonsai/internal/registry"
)

// detectRepoRoot returns the git repository root or "." as fallback.
func detectRepoRoot() string {
	if gitutil.IsInsideWorkTree(".") {
		if r, err := gitutil.ShowToplevel("."); err == nil {
			return r
		}
	}
	return "."
}

// cmdEnv holds the resolved runtime environment shared by all commands.
// Immutable after construction.
type cmdEnv struct {
	RepoRoot string
	Config   *config.Config
	Resolver *assets.Resolver
	Registry *registry.Registry // nil for commands that don't need skill resolution
}

// bootstrap resolves the full command environment:
// repo root → config → resolver → registry.
func bootstrap() (cmdEnv, error) {
	return bootstrapFrom(detectRepoRoot())
}

// bootstrapFrom resolves the full command environment from a given repo root.
// Used by commands that resolve the repo root themselves (e.g. after
// auto-creating a worktree).
func bootstrapFrom(repoRoot string) (cmdEnv, error) {
	env, err := bootstrapLight(repoRoot)
	if err != nil {
		return cmdEnv{}, err
	}
	reg, err := registry.Load(env.Resolver)
	if err != nil {
		return cmdEnv{}, fmt.Errorf("load registry: %w", err)
	}
	env.Registry = reg
	return env, nil
}

// bootstrapLight resolves the command environment without loading
// the skill registry. Used by interactive commands (chat, plan,
// review, implement) that don't need skill resolution.
func bootstrapLight(repoRoot string) (cmdEnv, error) {
	cfg, err := config.Load(repoRoot)
	if err != nil {
		return cmdEnv{}, fmt.Errorf("load config: %w", err)
	}
	resolver := assets.NewResolver(repoRoot)
	resolver.ExtraSkillDirs = cfg.Skills.ExtraDirs
	return cmdEnv{
		RepoRoot: repoRoot,
		Config:   cfg,
		Resolver: resolver,
	}, nil
}

// newAgentRouter creates an agent router from config.
func newAgentRouter(cfg *config.Config) *agent.Router {
	var apiOpts []agent.AnthropicOption
	if cfg.Providers.Anthropic.APIKey != "" {
		apiOpts = append(apiOpts, agent.WithAPIKey(cfg.Providers.Anthropic.APIKey))
	}
	return agent.NewRouter(cfg.Agents.Claude.Bin, cfg.Agents.Codex.Bin, apiOpts...)
}

// skillSet holds a resolved set of skills and their provenance.
type skillSet struct {
	Skills []registry.Skill
	Source string // e.g. "mode:NORMAL" or "bundle:default"
}

// resolveSkillSet resolves skills from either mode or bundle.
func resolveSkillSet(reg *registry.Registry, mode, bundle string) (skillSet, error) {
	if mode != "" {
		skills, err := reg.SkillsForMode(registry.GovMode(mode))
		return skillSet{Skills: skills, Source: "mode:" + mode}, err
	}
	skills, err := reg.SkillsForBundle(bundle)
	return skillSet{Skills: skills, Source: "bundle:" + bundle}, err
}

// resolveConcurrency resolves concurrency: flag > config > unlimited (0).
func resolveConcurrency(cfg *config.Config, c *cli.Context) int {
	concurrency := 0
	if cfg.Check.Concurrency != nil {
		concurrency = *cfg.Check.Concurrency
	}
	if c.IsSet("jobs") {
		concurrency = c.Int("jobs")
	}
	return concurrency
}

// fileExists checks whether a file exists at the given absolute path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isDirectory checks whether the given absolute path is a directory.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// worktreeResult holds the outcome of ensureFeatureBranch.
type worktreeResult struct {
	RepoRoot    string // effective repo root (may be a new worktree)
	IsWorktree  bool   // true if a worktree was created
	WorktreePath string // absolute path to the created worktree (empty if none)
}

// ensureFeatureBranch checks whether the repository is on main/master.
// If so, it automatically creates a git worktree with a timestamped
// branch, changes process CWD to the new worktree, and returns the
// updated repo root. Agent subprocesses inherit CWD, so this ensures
// they execute in the correct directory.
//
// If the directory is not a git repository, this is a no-op (returns
// the original root unchanged).
func ensureFeatureBranch(repoRoot, command string) (worktreeResult, error) {
	noOp := worktreeResult{RepoRoot: repoRoot}

	// Distinguish "not a git repo" (skip silently) from real git
	// errors like permissions or corruption (surface to the user).
	inside, err := gitutil.CheckInsideWorkTree(repoRoot)
	if err != nil {
		return noOp, fmt.Errorf("detect git repo: %w", err)
	}
	if !inside {
		return noOp, nil
	}

	branch, err := gitutil.CurrentBranch(repoRoot)
	if err != nil {
		return noOp, fmt.Errorf("detect branch: %w", err)
	}

	if branch != "main" && branch != "master" {
		// Already on a feature branch — nothing to do.
		return noOp, nil
	}

	// Warn if the working tree has uncommitted changes — those edits
	// will NOT be present in the new worktree (it starts from HEAD).
	if dirty, _ := gitutil.IsDirty(repoRoot); dirty {
		fmt.Fprintln(os.Stderr, "warning: uncommitted changes on "+branch+" will not be present in the new worktree")
	}

	// Generate a timestamped worktree name:
	//   ../bonsai-<command>-<YYYYMMDD-HHMMSS>
	ts := time.Now().Format("20060102-150405")
	repoName := filepath.Base(repoRoot)
	wtDir := fmt.Sprintf("%s-%s-%s", repoName, command, ts)
	wtPath := filepath.Join(filepath.Dir(repoRoot), wtDir)
	branchName := fmt.Sprintf("bonsai/%s/%s", command, ts)

	if err := gitutil.CreateWorktree(repoRoot, wtPath, branchName); err != nil {
		return noOp, fmt.Errorf("create worktree: %w", err)
	}

	// Change process CWD so agent subprocesses (claude, codex) that
	// inherit CWD execute in the worktree, not the original checkout.
	if err := os.Chdir(wtPath); err != nil {
		// Clean up the worktree we just created to avoid orphans.
		_ = gitutil.RemoveWorktree(repoRoot, wtPath)
		return noOp, fmt.Errorf("chdir to worktree: %w", err)
	}

	fmt.Printf("Created worktree: %s (branch: %s)\n", wtPath, branchName)

	return worktreeResult{
		RepoRoot:     wtPath,
		IsWorktree:   true,
		WorktreePath: wtPath,
	}, nil
}

// printWorktreeReminder prints a post-session reminder if a worktree was created.
func printWorktreeReminder(wt worktreeResult) {
	if !wt.IsWorktree {
		return
	}
	fmt.Printf("\nWorktree: %s — remember to clean up when done\n", wt.WorktreePath)
}
