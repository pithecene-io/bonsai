package cli

import (
	"fmt"
	"sort"

	"github.com/pithecene-io/bonsai/internal/assets"
	"github.com/pithecene-io/bonsai/internal/config"
	"github.com/pithecene-io/bonsai/internal/gitutil"
	"github.com/pithecene-io/bonsai/internal/registry"
	"github.com/urfave/cli/v2"
)

func listCommand() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List available skills, bundles, or roles",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "skills", Usage: "List skills"},
			&cli.BoolFlag{Name: "bundles", Usage: "List bundles"},
			&cli.BoolFlag{Name: "roles", Usage: "List roles"},
		},
		Action: runList,
	}
}

func runList(c *cli.Context) error {
	repoRoot := "."
	if gitutil.IsInsideWorkTree(".") {
		if r, err := gitutil.ShowToplevel("."); err == nil {
			repoRoot = r
		}
	}

	cfg, err := config.Load(repoRoot)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	resolver := assets.NewResolver(repoRoot)
	resolver.ExtraSkillDirs = cfg.Skills.ExtraDirs

	reg, err := registry.Load(resolver)
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	showSkills := c.Bool("skills")
	showBundles := c.Bool("bundles")
	showRoles := c.Bool("roles")

	// If nothing specified, show all
	if !showSkills && !showBundles && !showRoles {
		showSkills = true
		showBundles = true
		showRoles = true
	}

	if showSkills {
		fmt.Println("Skills:")
		for i := range reg.Skills {
			s := &reg.Skills[i]
			mandatory := ""
			if s.Mandatory {
				mandatory = " [mandatory]"
			}
			fmt.Printf("  %-45s %s/%s  %s%s\n", s.Name, s.Cost, s.Mode, s.Domain, mandatory)
		}
		fmt.Println()
	}

	if showBundles {
		fmt.Println("Bundles:")
		names := reg.BundleNames()
		sort.Strings(names)
		for _, name := range names {
			skills := reg.Bundles[name]
			fmt.Printf("  %-20s (%d skills)\n", name, len(skills))
		}
		fmt.Println()
	}

	if showRoles {
		fmt.Println("Roles:")
		roles := []string{"architect", "implementer", "planner", "reviewer", "patch-architect", "patcher"}
		for _, r := range roles {
			fmt.Printf("  %s\n", r)
		}
		fmt.Println()
	}

	return nil
}
