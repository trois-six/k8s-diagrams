package diagram

import (
	"strings"

	"github.com/blushft/go-diagrams/diagram"
	"github.com/blushft/go-diagrams/nodes/k8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	kindDeployment = "deployment"
	setColor       = "#9EBCDA"
)

func (d *Diagram) GenerateDeployments(namespace string, o *appsv1.DeploymentList) {
	for _, v := range o.Items {
		if v.Namespace != namespace || v.Status.Replicas == 0 || v.Status.AvailableReplicas == 0 {
			continue
		}

		d.deployments[v.Name] = k8s.Compute.Deploy(
			diagram.NodeLabel(v.Name),
			diagram.SetFontOptions(diagram.Font{Size: nodeFontSize}),
			diagram.Width(nodeWidth),
		)
		d.namespaceGroups[namespace].Add(d.deployments[v.Name])
	}
}

func (d *Diagram) GenerateDaemonSets(namespace string, o *appsv1.DaemonSetList) {
	for _, v := range o.Items {
		if v.Namespace != namespace || v.Status.CurrentNumberScheduled == 0 {
			continue
		}

		d.daemonSets[v.Name] = k8s.Compute.Ds(
			diagram.NodeLabel(v.Name),
			diagram.SetFontOptions(diagram.Font{Size: nodeFontSize}),
			diagram.Width(nodeWidth),
		)
		d.daemonSetGroups[v.Name] = diagram.NewGroup(v.Name, func(o *diagram.GroupOptions) {
			o.Font = diagram.Font{
				Size: groupFontSize,
			}
			o.BackgroundColor = setColor
		}).Label("ds")
		d.namespaceGroups[namespace].Group(d.daemonSetGroups[v.Name])
		d.namespaceGroups[namespace].Add(d.daemonSets[v.Name])
	}
}

func (d *Diagram) GenerateReplicaSets(namespace string, o *appsv1.ReplicaSetList) {
	for _, v := range o.Items {
		if v.Namespace != namespace || v.Status.Replicas == 0 {
			continue
		}

		d.replicaSets[v.Name] = k8s.Compute.Rs(
			diagram.NodeLabel(v.Name),
			diagram.SetFontOptions(diagram.Font{Size: nodeFontSize}),
			diagram.Width(nodeWidth),
		)
		d.replicaSetGroups[v.Name] = diagram.NewGroup(v.Name, func(o *diagram.GroupOptions) {
			o.Font = diagram.Font{
				Size: groupFontSize,
			}
			o.BackgroundColor = setColor
		}).Label("rs")
		d.namespaceGroups[namespace].Add(d.replicaSets[v.Name]).Group(d.replicaSetGroups[v.Name])

		for _, o := range v.GetOwnerReferences() {
			if strings.ToLower(o.Kind) != kindDeployment {
				continue
			}

			d.namespaceGroups[namespace].Connect(d.deployments[o.Name], d.replicaSets[v.Name])
			d.replicaSets[v.Name].Label(o.Name + "-\\n" + strings.TrimPrefix(v.Name, o.Name+"-"))
		}
	}
}

func (d *Diagram) GenerateStatefulSets(namespace string, o *appsv1.StatefulSetList) {
	for _, v := range o.Items {
		if v.Namespace != namespace || v.Status.Replicas == 0 {
			continue
		}

		d.statefulSets[v.Name] = k8s.Compute.Sts(
			diagram.NodeLabel(v.Name),
			diagram.SetFontOptions(diagram.Font{Size: nodeFontSize}),
			diagram.Width(nodeWidth),
		)
		d.statefulSetGroups[v.Name] = diagram.NewGroup(v.Name, func(o *diagram.GroupOptions) {
			o.Font = diagram.Font{
				Size: groupFontSize,
			}
			o.BackgroundColor = setColor
		}).Label("sts")
		d.namespaceGroups[namespace].Group(d.statefulSetGroups[v.Name])
		d.namespaceGroups[namespace].Add(d.statefulSets[v.Name])
	}
}

func (d *Diagram) GeneratePods(namespace string, o *corev1.PodList) {
	for _, v := range o.Items {
		if v.Namespace != namespace {
			continue
		}

		d.pods[v.Name] = k8s.Compute.Pod(
			diagram.NodeLabel(v.Name),
			diagram.SetFontOptions(diagram.Font{Size: nodeFontSize}),
			diagram.Width(nodeWidth),
		)

		if len(v.GetOwnerReferences()) > 0 {
			for _, o := range v.GetOwnerReferences() {
				switch strings.ToLower(o.Kind) {
				case "daemonset":
					d.daemonSetGroups[o.Name].Add(d.pods[v.Name])
					d.namespaceGroups[namespace].Connect(d.daemonSets[o.Name], d.pods[v.Name])
					d.pods[v.Name].Label(o.Name + "-\\n" + strings.TrimPrefix(v.Name, o.Name+"-"))
				case "replicaset":
					d.replicaSetGroups[o.Name].Add(d.pods[v.Name])
					d.namespaceGroups[namespace].Connect(d.replicaSets[o.Name], d.pods[v.Name])
					d.pods[v.Name].Label(d.replicaSets[o.Name].Options.Label + "-\\n" + strings.TrimPrefix(v.Name, o.Name+"-"))
				case "statefulset":
					d.statefulSetGroups[o.Name].Add(d.pods[v.Name])
					d.namespaceGroups[namespace].Connect(d.statefulSets[o.Name], d.pods[v.Name])
					d.pods[v.Name].Label(o.Name + "-\\n" + strings.TrimPrefix(v.Name, o.Name+"-"))
				default:
				}
			}
		} else {
			d.namespaceGroups[namespace].Add(d.pods[v.Name])
		}
	}
}

func (d *Diagram) GenerateServicePodsLinks(namespace string, service string, endpoints *corev1.EndpointsList) {
	for _, ep := range endpoints.Items {
		if ep.Namespace != namespace {
			continue
		}

		if service != ep.Name {
			continue
		}

		for _, subset := range ep.Subsets {
			for _, address := range subset.Addresses {
				if address.TargetRef == nil {
					continue
				}

				if strings.ToLower(address.TargetRef.Kind) != "pod" {
					continue
				}

				d.namespaceGroups[namespace].Connect(d.pods[address.TargetRef.Name], d.services[service], diagram.Reverse())
			}
		}
	}
}

func (d *Diagram) GenerateServices(namespace string, services *corev1.ServiceList, endpoints *corev1.EndpointsList) {
	for _, svc := range services.Items {
		if svc.Namespace != namespace {
			continue
		}

		d.services[svc.Name] = k8s.Network.Svc(
			diagram.NodeLabel(svc.Name),
			diagram.SetFontOptions(diagram.Font{Size: nodeFontSize}),
			diagram.Width(nodeWidth),
		)
		d.namespaceGroups[namespace].Add(d.services[svc.Name])

		d.GenerateServicePodsLinks(namespace, svc.Name, endpoints)

		// for _, lb := range svc.Status.LoadBalancer.Ingress {
		// 	var publicIPName string
		// 	if lb.IP != "" {
		// 		publicIPName = lb.IP
		// 	} else if lb.Hostname != "" {
		// 		publicIPName = lb.Hostname
		// 	}

		// 	fmt.Printf("svc: %s, public access: %#v\n", svc.Name, publicIPName)
		// }
	}
}
