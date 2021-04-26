package diagram

import (
	"strings"

	"github.com/blushft/go-diagrams/diagram"
	"github.com/blushft/go-diagrams/nodes/k8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	nodeFontSize   = 8
	kindDeployment = "deployment"
)

func (d *Diagram) GenerateDeployments(namespace string, o *appsv1.DeploymentList) {
	for _, v := range o.Items {
		if v.Namespace != namespace || v.Status.Replicas == 0 || v.Status.AvailableReplicas == 0 {
			continue
		}

		d.deployments[v.Name] = k8s.Compute.Deploy(
			diagram.NodeLabel(v.Name),
			diagram.Width(1),
			diagram.Height(1),
			diagram.SetFontOptions(diagram.Font{Size: nodeFontSize}),
		)
		d.namespaceGroups[namespace].Add(d.deployments[v.Name]).Connect(d.namespaces[namespace], d.deployments[v.Name])
	}
}

func (d *Diagram) GenerateDaemonSets(namespace string, o *appsv1.DaemonSetList) {
	for _, v := range o.Items {
		if v.Namespace != namespace || v.Status.CurrentNumberScheduled == 0 {
			continue
		}

		d.daemonSets[v.Name] = k8s.Compute.Ds(
			diagram.NodeLabel(v.Name),
			diagram.Width(1),
			diagram.Height(1),
			diagram.SetFontOptions(diagram.Font{Size: nodeFontSize}),
		)
		d.daemonSetGroups[v.Name] = diagram.NewGroup(v.Name).Label("ds")
		d.namespaceGroups[namespace].Add(d.daemonSets[v.Name]).Group(d.daemonSetGroups[v.Name])

		for _, o := range v.GetOwnerReferences() {
			if strings.ToLower(o.Kind) != kindDeployment {
				continue
			}

			d.namespaceGroups[namespace].Connect(d.deployments[o.Name], d.daemonSets[v.Name])
		}
	}
}

func (d *Diagram) GenerateReplicaSets(namespace string, o *appsv1.ReplicaSetList) {
	for _, v := range o.Items {
		if v.Namespace != namespace || v.Status.Replicas == 0 {
			continue
		}

		d.replicaSets[v.Name] = k8s.Compute.Rs(
			diagram.NodeLabel(v.Name),
			diagram.Width(1),
			diagram.Height(1),
			diagram.SetFontOptions(diagram.Font{Size: nodeFontSize}),
		)
		d.replicaSetGroups[v.Name] = diagram.NewGroup(v.Name).Label("rs")
		d.namespaceGroups[namespace].Add(d.replicaSets[v.Name]).Group(d.replicaSetGroups[v.Name])

		for _, o := range v.GetOwnerReferences() {
			if strings.ToLower(o.Kind) != kindDeployment {
				continue
			}

			d.namespaceGroups[namespace].Connect(d.deployments[o.Name], d.replicaSets[v.Name])
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
			diagram.Width(1),
			diagram.Height(1),
			diagram.SetFontOptions(diagram.Font{Size: nodeFontSize}),
		)
		d.statefulSetGroups[v.Name] = diagram.NewGroup(v.Name).Label("sts")
		d.namespaceGroups[namespace].Add(d.statefulSets[v.Name]).Group(d.statefulSetGroups[v.Name])

		for _, o := range v.GetOwnerReferences() {
			if strings.ToLower(o.Kind) != kindDeployment {
				continue
			}

			d.namespaceGroups[namespace].Connect(d.deployments[o.Name], d.statefulSets[v.Name])
		}
	}
}

func (d *Diagram) GeneratePods(namespace string, o *corev1.PodList) {
	for _, v := range o.Items {
		if v.Namespace != namespace {
			continue
		}

		pod := k8s.Compute.Pod(
			diagram.NodeLabel(v.Name),
			diagram.Width(1),
			diagram.Height(1),
			diagram.SetFontOptions(diagram.Font{Size: nodeFontSize}),
		)

		if len(v.GetOwnerReferences()) > 0 {
			for _, o := range v.GetOwnerReferences() {
				switch strings.ToLower(o.Kind) {
				case "daemonset":
					d.daemonSetGroups[o.Name].Add(pod)
					d.namespaceGroups[namespace].Connect(d.daemonSets[o.Name], pod)
				case "replicaset":
					d.replicaSetGroups[o.Name].Add(pod)
					d.namespaceGroups[namespace].Connect(d.replicaSets[o.Name], pod)
				case "statefulset":
					d.statefulSetGroups[o.Name].Add(pod)
					d.namespaceGroups[namespace].Connect(d.statefulSets[o.Name], pod)
				default:
				}
			}
		} else {
			d.namespaceGroups[namespace].Add(pod)
		}
	}
}
