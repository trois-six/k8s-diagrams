package main

import (
	"os"

	"github.com/Trois-Six/k8s-diagrams/cmd"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "k8s-diagrams CLI",
		Usage: "Run k8s-diagrams",
		Commands: []*cli.Command{
			diagramCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("Error while executing command")
	}
}

func diagramCommand() *cli.Command {
	return &cli.Command{
		Name:        "diagram",
		Usage:       "Create diagram from Kubernetes API.",
		Description: "Create diagram from Kubernetes API.",
		Action:      cmd.Run,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "namespace",
				Usage:   "The namespace we want to draw.",
				EnvVars: []string{"KUBECTL_NAMESPACE"},
				Value:   "default",
			},
		},
	}
}
