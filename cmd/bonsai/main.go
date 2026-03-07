// Package main is the entrypoint for the bonsai CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/pithecene-io/bonsai/internal/cli"
)

func main() {
	// Install OS signal handling once at the top level so all
	// subcommands receive a cancellable context via c.Context.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	app := cli.NewApp()
	if err := app.RunContext(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
