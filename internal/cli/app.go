// Package cli defines the bonsai command-line application and all subcommands.
package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// NewApp creates the top-level bonsai CLI application.
func NewApp() *cli.App {
	return &cli.App{
		Name:    "bonsai",
		Usage:   "AI governance toolkit for software repositories",
		Version: Version,
		Commands: []*cli.Command{
			versionCommand(),
			skillCommand(),
			listCommand(),
		},
	}
}

func versionCommand() *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "Print the bonsai version",
		Action: func(_ *cli.Context) error {
			fmt.Println(Version)
			return nil
		},
	}
}
