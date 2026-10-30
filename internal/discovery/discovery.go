// Package discovery fetches from the Kubernetes API the objects of a namespace.
package discovery

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

// Mode is the kind of diagram the objects are discovered for.
type Mode int

const (
	// ModeNetwork discovers the workloads, and how the traffic reaches them.
	ModeNetwork Mode = iota
	// ModeStorage discovers the workloads, and the volumes they use.
	ModeStorage
)

// Objects are the Kubernetes objects of a namespace.
// Cluster-scoped objects (IngressClasses, GatewayClasses, PersistentVolumes, StorageClasses) are not filtered.
type Objects struct {
	Namespace string
	Mode      Mode

	Pods []corev1.Pod

	DaemonSets   []appsv1.DaemonSet
	Deployments  []appsv1.Deployment
	ReplicaSets  []appsv1.ReplicaSet
	StatefulSets []appsv1.StatefulSet

	Services       []corev1.Service
	EndpointSlices []discoveryv1.EndpointSlice
	Ingresses      []networkingv1.Ingress
	IngressClasses []networkingv1.IngressClass

	GatewayClasses []gatewayv1.GatewayClass
	Gateways       []gatewayv1.Gateway
	Routes         []Route

	PersistentVolumeClaims []corev1.PersistentVolumeClaim
	PersistentVolumes      []corev1.PersistentVolume
	StorageClasses         []storagev1.StorageClass
}

// Client discovers Kubernetes objects.
type Client struct {
	kube    kubernetes.Interface
	gateway gatewayclient.Interface
}

// NewClient creates a Client from a REST configuration.
func NewClient(config *rest.Config) (*Client, error) {
	kube, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}

	gateway, err := gatewayclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating gateway API client: %w", err)
	}

	return &Client{kube: kube, gateway: gateway}, nil
}

// Discover fetches the objects of namespace needed by a diagram of the given mode.
func (c *Client) Discover(ctx context.Context, namespace string, mode Mode) (*Objects, error) {
	if err := c.checkNamespace(ctx, namespace); err != nil {
		return nil, err
	}

	o := &Objects{Namespace: namespace, Mode: mode}

	// Each goroutine writes to its own fields of o.
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return c.discoverPods(ctx, o) })
	g.Go(func() error { return c.discoverApps(ctx, o) })

	switch mode {
	case ModeNetwork:
		gatewayResources, err := c.servedGatewayResources(ctx)
		if err != nil {
			return nil, err
		}

		g.Go(func() error { return c.discoverNetworking(ctx, o) })
		g.Go(func() error { return c.discoverGatewayAPI(ctx, o, gatewayResources) })
	case ModeStorage:
		g.Go(func() error { return c.discoverStorage(ctx, o) })
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return o, nil
}

// checkNamespace fails if namespace does not exist.
// Users allowed to read a namespace but not the namespace object itself are not blocked.
func (c *Client) checkNamespace(ctx context.Context, namespace string) error {
	_, err := c.kube.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	switch {
	case err == nil:
		return nil
	case apierrors.IsForbidden(err):
		slog.Warn("Cannot check that the namespace exists", "namespace", namespace, "error", err)

		return nil
	default:
		return fmt.Errorf("getting namespace %q: %w", namespace, err)
	}
}

func (c *Client) discoverPods(ctx context.Context, o *Objects) error {
	pods, err := c.kube.CoreV1().Pods(o.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing pods: %w", err)
	}

	o.Pods = pods.Items

	return nil
}

func (c *Client) discoverApps(ctx context.Context, o *Objects) error {
	apps := c.kube.AppsV1()

	daemonSets, err := apps.DaemonSets(o.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing daemonsets: %w", err)
	}

	deployments, err := apps.Deployments(o.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing deployments: %w", err)
	}

	replicaSets, err := apps.ReplicaSets(o.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing replicasets: %w", err)
	}

	statefulSets, err := apps.StatefulSets(o.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing statefulsets: %w", err)
	}

	o.DaemonSets = daemonSets.Items
	o.Deployments = deployments.Items
	o.ReplicaSets = replicaSets.Items
	o.StatefulSets = statefulSets.Items

	return nil
}

func (c *Client) discoverNetworking(ctx context.Context, o *Objects) error {
	services, err := c.kube.CoreV1().Services(o.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing services: %w", err)
	}

	slices, err := c.kube.DiscoveryV1().EndpointSlices(o.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing endpointslices: %w", err)
	}

	networking := c.kube.NetworkingV1()

	ingresses, err := networking.Ingresses(o.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing ingresses: %w", err)
	}

	o.Services = services.Items
	o.EndpointSlices = slices.Items
	o.Ingresses = ingresses.Items

	if classes, err := networking.IngressClasses().List(ctx, metav1.ListOptions{}); err == nil {
		o.IngressClasses = classes.Items
	} else if err = ignoreForbidden(err, "ingressclasses"); err != nil {
		return err
	}

	return nil
}

func (c *Client) discoverStorage(ctx context.Context, o *Objects) error {
	core := c.kube.CoreV1()

	claims, err := core.PersistentVolumeClaims(o.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing persistentvolumeclaims: %w", err)
	}

	o.PersistentVolumeClaims = claims.Items

	if volumes, err := core.PersistentVolumes().List(ctx, metav1.ListOptions{}); err == nil {
		o.PersistentVolumes = volumes.Items
	} else if err = ignoreForbidden(err, "persistentvolumes"); err != nil {
		return err
	}

	if classes, err := c.kube.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{}); err == nil {
		o.StorageClasses = classes.Items
	} else if err = ignoreForbidden(err, "storageclasses"); err != nil {
		return err
	}

	return nil
}

// ignoreForbidden logs and drops forbidden errors, which are expected when listing
// cluster-scoped resources with namespace-scoped permissions.
func ignoreForbidden(err error, resource string) error {
	if apierrors.IsForbidden(err) {
		slog.Warn("Not allowed to list cluster-scoped resource, skipping it", "resource", resource, "error", err)

		return nil
	}

	return fmt.Errorf("listing %s: %w", resource, err)
}
