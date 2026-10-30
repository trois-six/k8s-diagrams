package discovery

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Gateway API route kinds.
const (
	KindHTTPRoute = "HTTPRoute"
	KindGRPCRoute = "GRPCRoute"
	KindTLSRoute  = "TLSRoute"
	KindTCPRoute  = "TCPRoute"
	KindUDPRoute  = "UDPRoute"
)

// Route is a Gateway API route, whatever its kind.
type Route struct {
	Kind        string
	Name        string
	Namespace   string
	ParentRefs  []gatewayv1.ParentReference
	BackendRefs []gatewayv1.BackendObjectReference
}

// servedGatewayResources returns the Gateway API v1 resources served by the cluster,
// which may have no or only some of the Gateway API CRDs installed.
func (c *Client) servedGatewayResources(ctx context.Context) (map[string]bool, error) {
	list, err := c.kube.Discovery().ServerResourcesForGroupVersionWithContext(ctx, gatewayv1.GroupVersion.String())
	if apierrors.IsNotFound(err) {
		return map[string]bool{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("discovering gateway API resources: %w", err)
	}

	served := make(map[string]bool, len(list.APIResources))
	for _, r := range list.APIResources {
		served[r.Name] = true
	}

	return served, nil
}

func (c *Client) discoverGatewayAPI(ctx context.Context, o *Objects, served map[string]bool) error {
	if served["gatewayclasses"] {
		if classes, err := c.gateway.GatewayV1().GatewayClasses().List(ctx, metav1.ListOptions{}); err == nil {
			o.GatewayClasses = classes.Items
		} else if err = ignoreForbidden(err, "gatewayclasses"); err != nil {
			return err
		}
	}

	if served["gateways"] {
		gateways, err := c.gateway.GatewayV1().Gateways(o.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("listing gateways: %w", err)
		}

		o.Gateways = gateways.Items
	}

	routeListers := []struct {
		resource string
		list     func(context.Context, string) ([]Route, error)
	}{
		{"httproutes", c.httpRoutes},
		{"grpcroutes", c.grpcRoutes},
		{"tlsroutes", c.tlsRoutes},
		{"tcproutes", c.tcpRoutes},
		{"udproutes", c.udpRoutes},
	}

	for _, l := range routeListers {
		if !served[l.resource] {
			continue
		}

		routes, err := l.list(ctx, o.Namespace)
		if err != nil {
			return fmt.Errorf("listing %s: %w", l.resource, err)
		}

		o.Routes = append(o.Routes, routes...)
	}

	return nil
}

func (c *Client) httpRoutes(ctx context.Context, namespace string) ([]Route, error) {
	list, err := c.gateway.GatewayV1().HTTPRoutes(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err //nolint:wrapcheck // Wrapped by the caller.
	}

	routes := make([]Route, 0, len(list.Items))
	for _, r := range list.Items {
		route := newRoute(KindHTTPRoute, r.ObjectMeta, r.Spec.CommonRouteSpec)
		for _, rule := range r.Spec.Rules {
			for _, ref := range rule.BackendRefs {
				route.BackendRefs = append(route.BackendRefs, ref.BackendObjectReference)
			}
		}

		routes = append(routes, route)
	}

	return routes, nil
}

func (c *Client) grpcRoutes(ctx context.Context, namespace string) ([]Route, error) {
	list, err := c.gateway.GatewayV1().GRPCRoutes(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err //nolint:wrapcheck // Wrapped by the caller.
	}

	routes := make([]Route, 0, len(list.Items))
	for _, r := range list.Items {
		route := newRoute(KindGRPCRoute, r.ObjectMeta, r.Spec.CommonRouteSpec)
		for _, rule := range r.Spec.Rules {
			for _, ref := range rule.BackendRefs {
				route.BackendRefs = append(route.BackendRefs, ref.BackendObjectReference)
			}
		}

		routes = append(routes, route)
	}

	return routes, nil
}

func (c *Client) tlsRoutes(ctx context.Context, namespace string) ([]Route, error) {
	list, err := c.gateway.GatewayV1().TLSRoutes(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err //nolint:wrapcheck // Wrapped by the caller.
	}

	routes := make([]Route, 0, len(list.Items))
	for _, r := range list.Items {
		route := newRoute(KindTLSRoute, r.ObjectMeta, r.Spec.CommonRouteSpec)
		for _, rule := range r.Spec.Rules {
			route.addBackendRefs(rule.BackendRefs)
		}

		routes = append(routes, route)
	}

	return routes, nil
}

func (c *Client) tcpRoutes(ctx context.Context, namespace string) ([]Route, error) {
	list, err := c.gateway.GatewayV1().TCPRoutes(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err //nolint:wrapcheck // Wrapped by the caller.
	}

	routes := make([]Route, 0, len(list.Items))
	for _, r := range list.Items {
		route := newRoute(KindTCPRoute, r.ObjectMeta, r.Spec.CommonRouteSpec)
		for _, rule := range r.Spec.Rules {
			route.addBackendRefs(rule.BackendRefs)
		}

		routes = append(routes, route)
	}

	return routes, nil
}

func (c *Client) udpRoutes(ctx context.Context, namespace string) ([]Route, error) {
	list, err := c.gateway.GatewayV1().UDPRoutes(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err //nolint:wrapcheck // Wrapped by the caller.
	}

	routes := make([]Route, 0, len(list.Items))
	for _, r := range list.Items {
		route := newRoute(KindUDPRoute, r.ObjectMeta, r.Spec.CommonRouteSpec)
		for _, rule := range r.Spec.Rules {
			route.addBackendRefs(rule.BackendRefs)
		}

		routes = append(routes, route)
	}

	return routes, nil
}

func newRoute(kind string, meta metav1.ObjectMeta, spec gatewayv1.CommonRouteSpec) Route {
	return Route{
		Kind:       kind,
		Name:       meta.Name,
		Namespace:  meta.Namespace,
		ParentRefs: spec.ParentRefs,
	}
}

func (r *Route) addBackendRefs(refs []gatewayv1.BackendRef) {
	for _, ref := range refs {
		r.BackendRefs = append(r.BackendRefs, ref.BackendObjectReference)
	}
}
