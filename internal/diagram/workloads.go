package diagram

import (
	"log/slog"
	"strings"

	"github.com/blushft/go-diagrams/diagram"
	"github.com/blushft/go-diagrams/nodes/k8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (d *Diagram) addDeployments(deployments []appsv1.Deployment) {
	for _, deploy := range deployments {
		if deploy.Status.AvailableReplicas == 0 {
			continue
		}

		slog.Debug("Generating deployment", "name", deploy.Name)
		d.addNode(kindDeployment, deploy.Name, k8s.Compute.Deploy)
	}
}

func (d *Diagram) addDaemonSets(daemonSets []appsv1.DaemonSet) {
	for _, ds := range daemonSets {
		if ds.Status.CurrentNumberScheduled == 0 {
			continue
		}

		slog.Debug("Generating daemonSet", "name", ds.Name)
		d.addSet(kindDaemonSet, ds.Name, "ds", k8s.Compute.Ds)
	}
}

func (d *Diagram) addReplicaSets(replicaSets []appsv1.ReplicaSet) {
	for _, rs := range replicaSets {
		if rs.Status.Replicas == 0 {
			continue
		}

		slog.Debug("Generating replicaSet", "name", rs.Name)

		n := d.addSet(kindReplicaSet, rs.Name, "rs", k8s.Compute.Rs)

		owner := metav1.GetControllerOf(&rs)
		if owner == nil || owner.Kind != kindDeployment {
			continue
		}

		deploy, ok := d.nodes[objectKey{kind: kindDeployment, namespace: d.namespace, name: owner.Name}]
		if !ok {
			continue
		}

		d.connect(deploy, n, "")
		setLabel(n, childLabel(owner.Name, strings.TrimPrefix(rs.Name, owner.Name+"-")))
	}
}

func (d *Diagram) addStatefulSets(statefulSets []appsv1.StatefulSet) {
	for _, sts := range statefulSets {
		if sts.Status.Replicas == 0 {
			continue
		}

		slog.Debug("Generating statefulSet", "name", sts.Name)
		d.addSet(kindStatefulSet, sts.Name, "sts", k8s.Compute.Sts)
	}
}

// addSet draws a set of pods, and the group in which its pods are drawn.
func (d *Diagram) addSet(kind, name, groupLabel string, icon icon) *diagram.Node {
	n := d.addNode(kind, name, icon)

	g := newGroup(kind+"-"+name, groupLabel, setColor)
	d.groups[objectKey{kind: kind, namespace: d.namespace, name: name}] = g
	d.nsGroup.Group(g)

	return n
}

func (d *Diagram) addPods(pods []corev1.Pod) {
	for _, pod := range pods {
		slog.Debug("Generating pod", "name", pod.Name)

		n := newNode(k8s.Compute.Pod, pod.Name)
		d.nodes[objectKey{kind: kindPod, namespace: d.namespace, name: pod.Name}] = n

		g := d.addPodToSet(&pod, n)
		if g == nil {
			g = d.nsGroup
			g.Add(n)
		}

		d.podGroups[pod.Name] = g
	}
}

// addPodToSet draws a pod in the group of the set controlling it, if any, and returns this group.
func (d *Diagram) addPodToSet(pod *corev1.Pod, n *diagram.Node) *diagram.Group {
	owner := metav1.GetControllerOf(pod)
	if owner == nil {
		return nil
	}

	key := objectKey{kind: owner.Kind, namespace: d.namespace, name: owner.Name}

	g, ok := d.groups[key]
	if !ok {
		return nil
	}

	set := d.nodes[key]

	slog.Debug("Adding pod to set", "pod", pod.Name, "kind", owner.Kind, "set", owner.Name)
	g.Add(n)
	d.connect(set, n, "")
	setLabel(n, childLabel(labelOf(set), strings.TrimPrefix(pod.Name, owner.Name+"-")))

	return g
}

// childLabel labels an object named after its parent on two lines, to keep nodes narrow.
func childLabel(parentLabel, suffix string) string {
	return parentLabel + "-\\n" + suffix
}
