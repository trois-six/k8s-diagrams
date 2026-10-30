package diagram

import (
	"cmp"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/blushft/go-diagrams/diagram"
	"github.com/blushft/go-diagrams/nodes/apps"
	"github.com/blushft/go-diagrams/nodes/generic"
	"github.com/blushft/go-diagrams/nodes/k8s"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/trois-six/k8s-diagrams/internal/discovery"
)

// annotationIngressClass is the deprecated annotation selecting the class of an Ingress.
const annotationIngressClass = "kubernetes.io/ingress.class"

func (d *Diagram) addServices(services []corev1.Service) {
	for _, svc := range services {
		slog.Debug("Generating service", "name", svc.Name)

		n := d.addNode(kindService, svc.Name, k8s.Network.Svc)

		switch svc.Spec.Type {
		case corev1.ServiceTypeExternalName:
			// Not connected to the Internet node, at the bottom of the diagram, to keep the edge short.
			external, _ := d.node(kindExternalName, "", svc.Spec.ExternalName, apps.Network.Internet)
			d.connect(n, external, "")
		case corev1.ServiceTypeNodePort:
			d.connectFromInternet(n, nodePorts(svc.Spec.Ports))
		case corev1.ServiceTypeClusterIP, corev1.ServiceTypeLoadBalancer:
		}

		for _, ip := range svc.Spec.ExternalIPs {
			d.connectFromInternet(n, ip)
		}

		for _, lb := range svc.Status.LoadBalancer.Ingress {
			d.connectFromInternet(n, cmp.Or(lb.IP, lb.Hostname))
		}
	}
}

func nodePorts(ports []corev1.ServicePort) string {
	nodePorts := make([]string, 0, len(ports))
	for _, p := range ports {
		if p.NodePort != 0 {
			nodePorts = append(nodePorts, strconv.Itoa(int(p.NodePort)))
		}
	}

	return "NodePort " + strings.Join(nodePorts, ",")
}

// addEndpointSlices connects the services to the pods backing them.
func (d *Diagram) addEndpointSlices(endpointSlices []discoveryv1.EndpointSlice) {
	for _, slice := range endpointSlices {
		service := slice.Labels[discoveryv1.LabelServiceName]

		svc, ok := d.nodes[objectKey{kind: kindService, namespace: d.namespace, name: service}]
		if !ok {
			continue
		}

		for _, ep := range slice.Endpoints {
			if ep.TargetRef == nil || ep.TargetRef.Kind != kindPod {
				continue
			}

			pod, ok := d.nodes[objectKey{kind: kindPod, namespace: d.namespace, name: ep.TargetRef.Name}]
			if !ok {
				continue
			}

			var opts []func(*edge)
			if !ptr.Deref(ep.Conditions.Ready, true) {
				opts = append(opts, dashed)
			}

			d.connectTraffic(svc, pod, "", opts...)
		}
	}
}

func (d *Diagram) addIngresses(ingresses []networkingv1.Ingress, classes []networkingv1.IngressClass) {
	controllers := make(map[string]string, len(classes))

	var defaultClass string

	for _, c := range classes {
		controllers[c.Name] = c.Spec.Controller
		if c.Annotations[networkingv1.AnnotationIsDefaultIngressClass] == "true" {
			defaultClass = c.Name
		}
	}

	for _, ing := range ingresses {
		slog.Debug("Generating ingress", "name", ing.Name)

		n := d.addNode(kindIngress, ing.Name, k8s.Network.Ing)

		className := cmp.Or(ptr.Deref(ing.Spec.IngressClassName, ""), ing.Annotations[annotationIngressClass], defaultClass)
		if className != "" {
			class, _ := d.node(kindIngressClass, "", className, controllerIcon(controllers[className]))
			d.connectTraffic(class, n, "", dashed)
		}

		if b := ing.Spec.DefaultBackend; b != nil {
			d.connectIngressBackend(n, b, "default")
		}

		for _, rule := range ing.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}

			for _, path := range rule.HTTP.Paths {
				d.connectIngressBackend(n, &path.Backend, rule.Host+path.Path)
			}
		}

		for _, lb := range ing.Status.LoadBalancer.Ingress {
			d.connectFromInternet(n, cmp.Or(lb.IP, lb.Hostname))
		}
	}
}

func (d *Diagram) connectIngressBackend(ing *diagram.Node, backend *networkingv1.IngressBackend, route string) {
	if backend.Service == nil {
		return
	}

	svc, ok := d.nodes[objectKey{kind: kindService, namespace: d.namespace, name: backend.Service.Name}]
	if !ok {
		return
	}

	port := backend.Service.Port.Name
	if port == "" {
		port = strconv.Itoa(int(backend.Service.Port.Number))
	}

	d.connectTraffic(ing, svc, route+" :"+port)
}

func (d *Diagram) addGateways(gateways []gatewayv1.Gateway, classes []gatewayv1.GatewayClass) {
	controllers := make(map[string]string, len(classes))
	for _, c := range classes {
		controllers[c.Name] = string(c.Spec.ControllerName)
	}

	for _, gw := range gateways {
		slog.Debug("Generating gateway", "name", gw.Name)

		n := d.addNode(kindGateway, gw.Name, generic.Network.Router)

		className := string(gw.Spec.GatewayClassName)
		class, _ := d.node(kindGatewayClass, "", className, controllerIcon(controllers[className]))
		d.connectTraffic(class, n, "", dashed)

		for _, addr := range gw.Status.Addresses {
			d.connectFromInternet(n, addr.Value)
		}
	}
}

func (d *Diagram) addRoutes(routes []discovery.Route) {
	for _, r := range routes {
		slog.Debug("Generating route", "kind", r.Kind, "name", r.Name)

		n := d.addNode(r.Kind, r.Name, k8s.Network.Ing)
		setLabel(n, fmt.Sprintf("%s\\n(%s)", r.Name, r.Kind))

		for _, ref := range r.ParentRefs {
			parent, ok := d.routeParent(ref, r.Namespace)
			if !ok {
				continue
			}

			label := ""
			if ref.SectionName != nil {
				label = string(*ref.SectionName)
			}

			d.connectTraffic(parent, n, label)
		}

		for _, ref := range r.BackendRefs {
			if ptr.Deref(ref.Group, "") != "" || cmp.Or(ptr.Deref(ref.Kind, ""), kindService) != kindService {
				continue
			}

			namespace := cmp.Or(string(ptr.Deref(ref.Namespace, "")), r.Namespace)

			svc, ok := d.node(kindService, namespace, string(ref.Name), k8s.Network.Svc)
			if !ok {
				continue
			}

			label := ""
			if ref.Port != nil {
				label = ":" + strconv.Itoa(int(*ref.Port))
			}

			d.connectTraffic(n, svc, label)
		}
	}
}

// routeParent returns the node of the parent of a route: a Gateway, or a Service for the service mesh (GAMMA) routes.
func (d *Diagram) routeParent(ref gatewayv1.ParentReference, routeNamespace string) (*diagram.Node, bool) {
	namespace := cmp.Or(string(ptr.Deref(ref.Namespace, "")), routeNamespace)
	group := ptr.Deref(ref.Group, "")
	kind := cmp.Or(ptr.Deref(ref.Kind, ""), kindGateway)

	switch {
	case cmp.Or(group, gatewayv1.GroupName) == gatewayv1.GroupName && kind == kindGateway:
		return d.node(kindGateway, namespace, string(ref.Name), generic.Network.Router)
	case group == "" && kind == kindService:
		return d.node(kindService, namespace, string(ref.Name), k8s.Network.Svc)
	default:
		return nil, false
	}
}

// controllerIcon returns the icon of an ingress or gateway controller.
func controllerIcon(controller string) icon {
	controller = strings.ToLower(controller)

	switch {
	case strings.Contains(controller, "traefik"):
		return apps.Network.Traefik
	case strings.Contains(controller, "nginx"):
		return apps.Network.Nginx
	case strings.Contains(controller, "haproxy"):
		return apps.Network.Haproxy
	case strings.Contains(controller, "kong"):
		return apps.Network.Kong
	case strings.Contains(controller, "istio"):
		return apps.Network.Istio
	case strings.Contains(controller, "envoy"), strings.Contains(controller, "contour"):
		return apps.Network.Envoy
	default:
		return k8s.Others.Crd
	}
}
