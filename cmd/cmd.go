package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"

	"github.com/Trois-Six/k8s-diagrams/pkg/diagram"
	"github.com/Trois-Six/k8s-diagrams/pkg/discovery"
	"github.com/Trois-Six/k8s-diagrams/pkg/logger"
	"github.com/urfave/cli/v2"

	// Blank import to allow client-go to connect on azure.
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"

	// Blank import to allow client-go to connect on gcp.
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	// Blank import to allow client-go to connect on oidc.
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	// Blank import to allow client-go to connect on openstack.
	_ "k8s.io/client-go/plugin/pkg/client/auth/openstack"
	"k8s.io/client-go/tools/clientcmd"
)

// Run executes the command.
func Run(cliContext *cli.Context) error {
	logger.Setup()

	if err := setupEnvVars(cliContext); err != nil {
		return err
	}

	var kc string
	if cliContext.String("kubeconfig") != "" {
		kc = cliContext.String("kubeconfig")
	} else {
		u, err := user.Current()
		if err != nil {
			return fmt.Errorf("don't know where is your kubeconfig: %w", err)
		}
		kc = filepath.Join(u.HomeDir, ".kube", "config")
	}

	if _, err := os.Stat(kc); err != nil {
		return fmt.Errorf("can't read kubeconfig: %w", err)
	}

	ns := cliContext.String("namespace")

	config, err := clientcmd.BuildConfigFromFlags("", kc)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	k, err := discovery.NewDiscovery(ctx, config)
	if err != nil {
		log.Fatal(err)
	}

	o, err := k.GenerateAll(ns)
	if err != nil {
		log.Fatal(err)
	}

	d, err := diagram.NewDiagram(
		cliContext.String("outputDirectory"),
		cliContext.String("outputFilename"),
		cliContext.String("label"),
	)
	if err != nil {
		log.Fatal(err)
	}

	d.GenerateDiagram(ns, o)

	if err = d.RenderDiagram(); err != nil {
		log.Fatal(err)
	}

	return nil
}

func setupEnvVars(context *cli.Context) error {
	vars := map[string]string{
		"KUBECTL_NAMESPACE": "namespace",
		"KUBECONFIG":        "kubeconfig",
	}

	for name, flag := range vars {
		if err := os.Setenv(name, context.String(flag)); err != nil {
			return fmt.Errorf("failed to set environment variable: %w", err)
		}
	}

	return nil
}
