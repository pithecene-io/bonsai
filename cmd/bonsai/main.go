// Package main is the entrypoint for the bonsai CLI.
package main

import (
	"fmt"
	"os"

	"github.com/pithecene-io/bonsai/internal/cli"
)

func main() {
	app := cli.NewApp()
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
