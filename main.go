// Command k8s-diagrams creates a diagram from the objects of a Kubernetes namespace.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v3"
	// Registers the client-go authentication plugins (oidc, and stubs for the removed cloud providers ones).
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/trois-six/k8s-diagrams/internal/diagram"
	"github.com/trois-six/k8s-diagrams/internal/discovery"
)

// Set at build time with -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(); err != nil {
		slog.Error("k8s-diagrams failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return newCommand().Run(ctx, os.Args)
}

func newCommand() *cli.Command {
	return &cli.Command{
		Name:    "k8s-diagrams",
		Usage:   "Create diagram from the Kubernetes API: a network diagram by default, or a storage one.",
		Version: fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "namespace",
				Aliases: []string{"n"},
				Usage:   "The namespace we want to draw (defaults to the namespace of the kubeconfig context).",
				Sources: cli.EnvVars("KUBECTL_NAMESPACE"),
			},
			&cli.StringFlag{
				Name:    "kubeconfig",
				Aliases: []string{"c"},
				Usage:   "The path to your kube config file (defaults to $KUBECONFIG, then ~/.kube/config).",
			},
			&cli.StringFlag{
				Name:  "context",
				Usage: "The kubeconfig context to use (defaults to the current context).",
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
			&cli.BoolFlag{
				Name:  "storage",
				Usage: "Draw the volumes used by the workloads, instead of how the traffic reaches them.",
			},
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "The log level (debug, info, warn, error).",
				Sources: cli.EnvVars("LOG_LEVEL"),
				Value:   "info",
			},
		},
		Action: generate,
	}
}

func generate(ctx context.Context, cmd *cli.Command) error {
	if err := setupLogger(cmd.String("log-level")); err != nil {
		return err
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = cmd.String("kubeconfig")

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{CurrentContext: cmd.String("context")},
	)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("loading kubeconfig: %w", err)
	}

	namespace := cmd.String("namespace")
	if namespace == "" {
		if namespace, _, err = clientConfig.Namespace(); err != nil {
			return fmt.Errorf("getting namespace from kubeconfig: %w", err)
		}
	}

	client, err := discovery.NewClient(restConfig)
	if err != nil {
		return err
	}

	mode := discovery.ModeNetwork
	if cmd.Bool("storage") {
		mode = discovery.ModeStorage
	}

	objects, err := client.Discover(ctx, namespace, mode)
	if err != nil {
		return err
	}

	d, err := diagram.New(cmd.String("outputDirectory"), cmd.String("outputFilename"), cmd.String("label"))
	if err != nil {
		return err
	}

	d.Build(objects)

	return d.Render()
}

func setupLogger(level string) error {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		return fmt.Errorf("parsing log level: %w", err)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})))

	return nil
}
