package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"github.com/okra-platform/okra/internal/commands"
)

var (
	// Build information. Populated at build-time via -ldflags flag.
	version = "dev"
	commit  = "HEAD"
	date    = "now"
)

func build() string {
	short := commit
	if len(commit) > 7 {
		short = commit[:7]
	}

	return fmt.Sprintf("%s (%s) %s", version, short, date)
}

func main() {
	ctrl := &commands.Controller{
		Flags: &commands.Flags{},
	}

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	app := &cli.Command{
		Name:    "okra",
		Usage:   `Open-source backend platform for building services, agentic workflows, and distributed systems—simple, observable, and fun.`,
		Version: build(),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "log level (debug, info, warn, error, fatal, panic)",
				Sources: cli.EnvVars("LOG_FORMAT"),
				Value:   "panic",
			},
		},
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			level, err := zerolog.ParseLevel(c.String("log-level"))
			if err != nil {
				return ctx, fmt.Errorf("failed to parse log level: %w", err)
			}

			log.Logger = log.Level(level)

			return ctx, nil
		},
		Commands: []*cli.Command{
			{
				Name:  "init",
				Usage: "Create a new OKRA project from a template",
				Action: func(ctx context.Context, c *cli.Command) error {
					return ctrl.Init(ctx)
				},
			},
			{
				Name:  "dev",
				Usage: "Start development server with hot-reloading",
				Action: func(ctx context.Context, c *cli.Command) error {
					return ctrl.Dev(ctx)
				},
			},
			{
				Name:  "build",
				Usage: "Build OKRA service package",
				Action: func(ctx context.Context, c *cli.Command) error {
					return ctrl.Build(ctx)
				},
			},
			{
				Name:  "deploy",
				Usage: "Deploy OKRA service to runtime",
				Action: func(ctx context.Context, c *cli.Command) error {
					return ctrl.Deploy(ctx)
				},
			},
		},
	}

	ctx := context.Background()

	if err := app.Run(ctx, os.Args); err != nil {
		log.Fatal().Err(err).Msg("failed to run okra")
	}
}
