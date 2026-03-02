package cli

import (
	"fmt"
	"sort"

	"github.com/urfave/cli/v2"

	"github.com/pithecene-io/bonsai/internal/registry"
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
	env, err := bootstrap()
	if err != nil {
		return err
	}
	reg := env.Registry

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
		printSkills(reg)
	}
	if showBundles {
		printBundles(reg)
	}
	if showRoles {
		printRoles()
	}

	return nil
}

func printSkills(reg *registry.Registry) {
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

func printBundles(reg *registry.Registry) {
	fmt.Println("Bundles:")
	names := reg.BundleNames()
	sort.Strings(names)
	for _, name := range names {
		skills := reg.Bundles[name]
		fmt.Printf("  %-20s (%d skills)\n", name, len(skills))
	}
	fmt.Println()
}

func printRoles() {
	fmt.Println("Roles:")
	roles := []string{"architect", "implementer", "planner", "reviewer", "patcher"}
	for _, r := range roles {
		fmt.Printf("  %s\n", r)
	}
	fmt.Println()
}
