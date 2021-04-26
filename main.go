package main

import (
	"os"

	"github.com/Trois-Six/k8s-diagrams/cmd"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "k8s-diagrams",
		Usage: "Create diagram from the Kubernetes API.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "namespace",
				Aliases: []string{"n"},
				Usage:   "The namespace we want to draw.",
				EnvVars: []string{"KUBECTL_NAMESPACE"},
				Value:   "default",
			},
			&cli.StringFlag{
				Name:    "kubeconfig",
				Aliases: []string{"c"},
				Usage:   "The path to your kube config file.",
				EnvVars: []string{"KUBECONFIG"},
			},
			&cli.StringFlag{
				Name:    "outputFilename",
				Aliases: []string{"o"},
				Usage:   "The output filename.",
				Value:   "k8s",
			},
			&cli.StringFlag{
				Name:    "outputDirectory",
				Aliases: []string{"d"},
				Usage:   "The output directory.",
				Value:   "diagrams",
			},
			&cli.StringFlag{
				Name:    "label",
				Aliases: []string{"l"},
				Usage:   "The diagram label.",
				Value:   "Kubernetes",
			},
		},
		Action: cmd.Run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("Error while executing command")
	}
}
