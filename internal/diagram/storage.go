package diagram

import (
	"cmp"
	"log/slog"
	"strings"

	"github.com/blushft/go-diagrams/diagram"
	"github.com/blushft/go-diagrams/nodes/k8s"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/utils/ptr"
)

// serviceAccountVolumePrefix prefixes the projected volumes injected in every pod to reach the API server,
// which are not drawn not to clutter the diagram.
const serviceAccountVolumePrefix = "kube-api-access-"

var accessModes = map[corev1.PersistentVolumeAccessMode]string{
	corev1.ReadWriteOnce:    "RWO",
	corev1.ReadOnlyMany:     "ROX",
	corev1.ReadWriteMany:    "RWX",
	corev1.ReadWriteOncePod: "RWOP",
}

// addPersistentVolumeClaims draws the claims, the volumes bound to them, and their storage classes.
func (d *Diagram) addPersistentVolumeClaims(
	claims []corev1.PersistentVolumeClaim,
	volumes []corev1.PersistentVolume,
	classes []storagev1.StorageClass,
) {
	volumeClasses := make(map[string]string, len(volumes))
	for _, pv := range volumes {
		volumeClasses[pv.Name] = pv.Spec.StorageClassName
	}

	provisioners := make(map[string]string, len(classes))
	for _, sc := range classes {
		provisioners[sc.Name] = sc.Provisioner
	}

	for _, pvc := range claims {
		slog.Debug("Generating persistentVolumeClaim", "name", pvc.Name)

		n := d.addNode(kindPersistentVolumeClaim, pvc.Name, k8s.Storage.Pvc)

		if pvc.Spec.VolumeName == "" {
			if class := d.storageClass(ptr.Deref(pvc.Spec.StorageClassName, ""), provisioners); class != nil {
				d.connect(n, class, string(pvc.Status.Phase), dashed)
			}

			continue
		}

		pv, _ := d.node(kindPersistentVolume, "", pvc.Spec.VolumeName, k8s.Storage.Pv)
		d.connect(n, pv, claimSummary(&pvc))

		className := cmp.Or(volumeClasses[pvc.Spec.VolumeName], ptr.Deref(pvc.Spec.StorageClassName, ""))
		if class := d.storageClass(className, provisioners); class != nil {
			d.connect(pv, class, "", dashed)
		}
	}
}

// storageClass returns the node of a storage class, labeled with its provisioner, or nil if name is empty.
func (d *Diagram) storageClass(name string, provisioners map[string]string) *diagram.Node {
	if name == "" {
		return nil
	}

	n, _ := d.node(kindStorageClass, "", name, k8s.Storage.Sc)
	if provisioner := provisioners[name]; provisioner != "" {
		setLabel(n, name+`\n(`+provisioner+")")
	}

	return n
}

// claimSummary returns the capacity and the access modes of a claim, such as "10Gi RWO".
func claimSummary(pvc *corev1.PersistentVolumeClaim) string {
	capacity, ok := pvc.Status.Capacity[corev1.ResourceStorage]
	if !ok {
		capacity = pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	}

	parts := make([]string, 0, 1+len(pvc.Spec.AccessModes))
	parts = append(parts, capacity.String())

	for _, mode := range pvc.Spec.AccessModes {
		parts = append(parts, accessModes[mode])
	}

	return strings.Join(parts, " ")
}

// addPodVolumes draws the volumes of the pods.
func (d *Diagram) addPodVolumes(pods []corev1.Pod) {
	for _, pod := range pods {
		n := d.nodes[objectKey{kind: kindPod, namespace: d.namespace, name: pod.Name}]

		for _, v := range pod.Spec.Volumes {
			d.addPodVolume(&pod, n, &v)
		}
	}
}

func (d *Diagram) addPodVolume(pod *corev1.Pod, n *diagram.Node, v *corev1.Volume) {
	switch {
	case v.PersistentVolumeClaim != nil:
		label := ""
		if v.PersistentVolumeClaim.ReadOnly {
			label = "ro"
		}

		d.connectToClaim(n, v.PersistentVolumeClaim.ClaimName, label)
	case v.Ephemeral != nil:
		// The claim of a generic ephemeral volume is named after the pod and the volume.
		d.connectToClaim(n, pod.Name+"-"+v.Name, "ephemeral")
	case v.ConfigMap != nil:
		d.connect(n, d.namespacedNode(kindConfigMap, v.ConfigMap.Name, k8s.Podconfig.Cm), "")
	case v.Secret != nil:
		d.connect(n, d.namespacedNode(kindSecret, v.Secret.SecretName, k8s.Podconfig.Secret), "")
	case v.Projected != nil:
		d.addProjectedVolume(n, v)
	case v.HostPath != nil:
		// Shared by the pods using the same path, outside of the namespace as it is on the nodes.
		hostPath, _ := d.node(kindHostPath, "", v.HostPath.Path, k8s.Storage.Vol)
		setLabel(hostPath, v.HostPath.Path+`\n(hostPath)`)
		d.connect(n, hostPath, "")
	case v.NFS != nil:
		export := v.NFS.Server + ":" + v.NFS.Path
		nfs, _ := d.node(kindNFS, "", export, k8s.Storage.Vol)
		setLabel(nfs, v.NFS.Server+`\n`+v.NFS.Path+`\n(nfs)`)
		d.connect(n, nfs, "")
	case v.EmptyDir != nil:
		d.addPodLocalVolume(pod, n, v, "emptyDir")
	case v.CSI != nil:
		d.addPodLocalVolume(pod, n, v, v.CSI.Driver)
	case v.Image != nil:
		d.addPodLocalVolume(pod, n, v, "image")
	default:
	}
}

// addProjectedVolume connects a pod to the config maps and secrets projected in one of its volumes.
func (d *Diagram) addProjectedVolume(n *diagram.Node, v *corev1.Volume) {
	if strings.HasPrefix(v.Name, serviceAccountVolumePrefix) {
		return
	}

	for _, source := range v.Projected.Sources {
		switch {
		case source.ConfigMap != nil:
			d.connect(n, d.namespacedNode(kindConfigMap, source.ConfigMap.Name, k8s.Podconfig.Cm), "")
		case source.Secret != nil:
			d.connect(n, d.namespacedNode(kindSecret, source.Secret.Name, k8s.Podconfig.Secret), "")
		default:
		}
	}
}

// connectToClaim connects a pod to a persistent volume claim of the namespace.
func (d *Diagram) connectToClaim(pod *diagram.Node, claim, label string) {
	pvc, ok := d.nodes[objectKey{kind: kindPersistentVolumeClaim, namespace: d.namespace, name: claim}]
	if !ok {
		return
	}

	d.connect(pod, pvc, label)
}

// addPodLocalVolume draws a volume which lives and dies with its pod, next to the pod.
func (d *Diagram) addPodLocalVolume(pod *corev1.Pod, n *diagram.Node, v *corev1.Volume, volumeType string) {
	volume := newNode(k8s.Storage.Vol, v.Name+`\n(`+volumeType+")")
	d.nodes[objectKey{kind: kindVolume, namespace: d.namespace, name: pod.Name + "/" + v.Name}] = volume
	d.podGroups[pod.Name].Add(volume)
	d.connect(n, volume, "")
}

// namespacedNode returns the node of an object of the namespace, drawn on first use.
func (d *Diagram) namespacedNode(kind, name string, icon icon) *diagram.Node {
	if n, ok := d.nodes[objectKey{kind: kind, namespace: d.namespace, name: name}]; ok {
		return n
	}

	return d.addNode(kind, name, icon)
}
