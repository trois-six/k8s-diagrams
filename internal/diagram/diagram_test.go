package diagram

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/trois-six/k8s-diagrams/internal/discovery"
)

var output = flag.String("output", "", "Directory where the test diagram is kept, and rendered as PNG.")

const testNamespace = "shop"

func meta(name string, owner ...metav1.OwnerReference) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: testNamespace, OwnerReferences: owner}
}

func controller(kind, name string) metav1.OwnerReference {
	return metav1.OwnerReference{Kind: kind, Name: name, Controller: new(true)}
}

func pods(owner metav1.OwnerReference, names ...string) []corev1.Pod {
	pods := make([]corev1.Pod, 0, len(names))
	for _, name := range names {
		pods = append(pods, corev1.Pod{ObjectMeta: meta(name, owner)})
	}

	return pods
}

func endpointSlice(service string, readyPods ...string) discoveryv1.EndpointSlice {
	slice := discoveryv1.EndpointSlice{ObjectMeta: meta(service + "-x1y2z")}
	slice.Labels = map[string]string{discoveryv1.LabelServiceName: service}

	for _, pod := range readyPods {
		ready := !strings.HasPrefix(pod, "!")
		slice.Endpoints = append(slice.Endpoints, discoveryv1.Endpoint{
			TargetRef:  &corev1.ObjectReference{Kind: kindPod, Name: strings.TrimPrefix(pod, "!")},
			Conditions: discoveryv1.EndpointConditions{Ready: &ready},
		})
	}

	return slice
}

func backend(service string, port int32) gatewayv1.BackendObjectReference {
	return gatewayv1.BackendObjectReference{Name: gatewayv1.ObjectName(service), Port: new(port)}
}

func ingressPath(path, service, port string) networkingv1.HTTPIngressPath {
	return networkingv1.HTTPIngressPath{
		Path: path,
		Backend: networkingv1.IngressBackend{
			Service: &networkingv1.IngressServiceBackend{Name: service, Port: networkingv1.ServiceBackendPort{Name: port}},
		},
	}
}

// testObjects returns a namespace with all the drawn kinds of objects.
func testObjects() *discovery.Objects {
	o := &discovery.Objects{
		Namespace: testNamespace,
		Deployments: []appsv1.Deployment{
			{ObjectMeta: meta("web"), Status: appsv1.DeploymentStatus{AvailableReplicas: 3}},
			{ObjectMeta: meta("api"), Status: appsv1.DeploymentStatus{AvailableReplicas: 2}},
		},
		ReplicaSets: []appsv1.ReplicaSet{
			{ObjectMeta: meta("web-7c9d8f", controller(kindDeployment, "web")), Status: appsv1.ReplicaSetStatus{Replicas: 3}},
			{ObjectMeta: meta("api-5f6b7", controller(kindDeployment, "api")), Status: appsv1.ReplicaSetStatus{Replicas: 2}},
			{ObjectMeta: meta("api-1a2b3", controller(kindDeployment, "api"))},
		},
		DaemonSets: []appsv1.DaemonSet{
			{ObjectMeta: meta("log-agent"), Status: appsv1.DaemonSetStatus{CurrentNumberScheduled: 2}},
		},
		StatefulSets: []appsv1.StatefulSet{
			{ObjectMeta: meta("db"), Status: appsv1.StatefulSetStatus{Replicas: 2}},
		},
		Services: []corev1.Service{
			{ObjectMeta: meta("web")},
			{ObjectMeta: meta("api")},
			{ObjectMeta: meta("db")},
			{
				ObjectMeta: meta("front"),
				Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
				Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{{IP: "203.0.113.10"}},
				}},
			},
			{
				ObjectMeta: meta("admin"),
				Spec: corev1.ServiceSpec{
					Type:  corev1.ServiceTypeNodePort,
					Ports: []corev1.ServicePort{{NodePort: 30080}},
				},
			},
			{
				ObjectMeta: meta("payments"),
				Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName, ExternalName: "payments.example.com"},
			},
		},
		EndpointSlices: []discoveryv1.EndpointSlice{
			endpointSlice("web", "web-7c9d8f-abcde", "web-7c9d8f-fghij", "web-7c9d8f-klmno"),
			endpointSlice("front", "web-7c9d8f-abcde", "web-7c9d8f-fghij", "web-7c9d8f-klmno"),
			endpointSlice("api", "api-5f6b7-pqrst", "api-5f6b7-uvwxy"),
			endpointSlice("admin", "api-5f6b7-pqrst", "api-5f6b7-uvwxy"),
			endpointSlice("db", "db-0", "!db-1"),
		},
		IngressClasses: []networkingv1.IngressClass{{
			Name:        "traefik",
			Annotations: map[string]string{networkingv1.AnnotationIsDefaultIngressClass: "true"},
			Spec:        networkingv1.IngressClassSpec{Controller: "traefik.io/ingress-controller"},
		}},
		Ingresses: []networkingv1.Ingress{{
			ObjectMeta: meta("shop"),
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{{
					Host: "shop.example.com",
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{ingressPath("/", "web", "http"), ingressPath("/api", "api", "http")},
					},
				}, {
					Host: "no-http.example.com",
				}},
			},
			Status: networkingv1.IngressStatus{LoadBalancer: networkingv1.IngressLoadBalancerStatus{
				Ingress: []networkingv1.IngressLoadBalancerIngress{{Hostname: "lb.example.com"}},
			}},
		}},
		GatewayClasses: []gatewayv1.GatewayClass{{
			Name: "eg",
			Spec: gatewayv1.GatewayClassSpec{ControllerName: "gateway.envoyproxy.io/gatewayclass-controller"},
		}},
		Gateways: []gatewayv1.Gateway{{
			ObjectMeta: meta("public"),
			Spec:       gatewayv1.GatewaySpec{GatewayClassName: "eg"},
			Status:     gatewayv1.GatewayStatus{Addresses: []gatewayv1.GatewayStatusAddress{{Value: "198.51.100.7"}}},
		}},
		Routes: []discovery.Route{
			{
				Kind: discovery.KindHTTPRoute, Name: "api", Namespace: testNamespace,
				ParentRefs:  []gatewayv1.ParentReference{{Name: "public", SectionName: ptr.To[gatewayv1.SectionName]("https")}},
				BackendRefs: []gatewayv1.BackendObjectReference{backend("api", 8080)},
			},
			{
				Kind: discovery.KindGRPCRoute, Name: "grpc", Namespace: testNamespace,
				ParentRefs:  []gatewayv1.ParentReference{{Name: "public"}},
				BackendRefs: []gatewayv1.BackendObjectReference{backend("api", 9090)},
			},
			{
				Kind: discovery.KindTCPRoute, Name: "db", Namespace: testNamespace,
				ParentRefs:  []gatewayv1.ParentReference{{Name: "tcp", Namespace: ptr.To[gatewayv1.Namespace]("infra")}},
				BackendRefs: []gatewayv1.BackendObjectReference{backend("db", 5432)},
			},
		},
	}

	o.Pods = append(o.Pods,
		pods(controller(kindReplicaSet, "web-7c9d8f"), "web-7c9d8f-abcde", "web-7c9d8f-fghij", "web-7c9d8f-klmno")...)
	o.Pods = append(o.Pods, pods(controller(kindReplicaSet, "api-5f6b7"), "api-5f6b7-pqrst", "api-5f6b7-uvwxy")...)
	o.Pods = append(o.Pods, pods(controller(kindDaemonSet, "log-agent"), "log-agent-q1w2e", "log-agent-r4t5y")...)
	o.Pods = append(o.Pods, pods(controller(kindStatefulSet, "db"), "db-0", "db-1")...)
	o.Pods = append(o.Pods, pods(controller("Job", "migrate"), "migrate-8h7g6")...)

	return o
}

// render renders a diagram of the objects, checks that graphviz renders it without warnings, and returns it.
func render(t *testing.T, o *discovery.Objects) string {
	t.Helper()

	dir := t.TempDir()
	if *output != "" {
		dir = filepath.Join(*output, t.Name())
	}

	d, err := New(dir, "k8s", "Kubernetes")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	d.Build(o)

	if err = d.Render(); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "k8s.dot"))
	if err != nil {
		t.Fatalf("reading diagram: %v", err)
	}

	if _, err := exec.LookPath("dot"); err != nil {
		t.Log("graphviz is not installed, not rendering the diagram")

		return string(data)
	}

	cmd := exec.CommandContext(t.Context(), "dot", "-Tpng", "-o", "k8s.png", "k8s.dot")
	cmd.Dir = dir

	if out, err := cmd.CombinedOutput(); err != nil || len(out) > 0 {
		t.Errorf("rendering diagram with graphviz: %v: %s", err, out)
	}

	return string(data)
}

func assertContains(t *testing.T, dot string, wants ...string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(dot, want) {
			t.Errorf("diagram does not contain %s", want)
		}
	}
}

func TestRenderNetwork(t *testing.T) {
	dot := render(t, testObjects())

	assertContains(t, dot,
		`"cluster_Namespace-shop"`, `"cluster_ReplicaSet-web-7c9d8f"`, `"cluster_DaemonSet-log-agent"`,
		`"web-\n7c9d8f"`, `"web-\n7c9d8f-\nabcde"`, `"log-agent-\nq1w2e"`, `"db-\n0"`, `migrate-8h7g6`,
		`Internet`, `"203.0.113.10"`, `"lb.example.com"`, `"198.51.100.7"`, `"payments.\nexample.com"`, `"NodePort 30080"`,
		`traefik`, `"shop.example.com/ :http"`, `eg`, `"api\n(HTTPRoute)"`, `":8080"`, `"https"`, `"infra/\ntcp"`,
		`style=dashed`,
	)

	if strings.Contains(dot, "api-\n1a2b3") {
		t.Error("diagram contains a replicaset without replicas")
	}

	if strings.Contains(dot, "pvc.png") {
		t.Error("network diagram contains storage objects")
	}
}

func volume(name string, source corev1.VolumeSource) corev1.Volume {
	return corev1.Volume{Name: name, VolumeSource: source}
}

func claim(name, class, volume, size string) corev1.PersistentVolumeClaim {
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: meta(name),
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &class,
			VolumeName:       volume,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
	}

	if volume == "" {
		pvc.Status.Phase = corev1.ClaimPending
	}

	return pvc
}

// testStorageObjects returns the namespace of testObjects, with all the drawn kinds of volumes.
func testStorageObjects() *discovery.Objects {
	o := testObjects()
	o.Mode = discovery.ModeStorage

	serviceAccount := volume("kube-api-access-x7k2p", corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{
		Sources: []corev1.VolumeProjection{{ConfigMap: &corev1.ConfigMapProjection{
			Name: "kube-root-ca.crt",
		}}},
	}})

	for i := range o.Pods {
		pod := &o.Pods[i]
		pod.Spec.Volumes = []corev1.Volume{serviceAccount}

		switch name := pod.Name; {
		case strings.HasPrefix(name, "web-"):
			pod.Spec.Volumes = append(pod.Spec.Volumes,
				volume("config", corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
					Name: "web-config",
				}}),
				volume("tls", corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "web-tls"}}),
				volume("cache", corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}),
			)
		case strings.HasPrefix(name, "api-"):
			pod.Spec.Volumes = append(pod.Spec.Volumes,
				volume("credentials", corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{{Secret: &corev1.SecretProjection{
						Name: "api-credentials",
					}}},
				}}),
				volume("shared", corev1.VolumeSource{
					NFS: &corev1.NFSVolumeSource{Server: "nfs.example.com", Path: "/exports/shared"},
				}),
			)
		case strings.HasPrefix(name, "log-agent-"):
			pod.Spec.Volumes = append(pod.Spec.Volumes,
				volume("logs", corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/log"}}),
				volume("config", corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
					Name: "log-agent-config",
				}}),
			)
		case strings.HasPrefix(name, "db-"):
			pod.Spec.Volumes = append(pod.Spec.Volumes, volume("data", corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-" + name},
			}))
		case strings.HasPrefix(name, "migrate-"):
			pod.Spec.Volumes = append(pod.Spec.Volumes,
				volume("scratch", corev1.VolumeSource{Ephemeral: &corev1.EphemeralVolumeSource{}}),
				volume("secrets", corev1.VolumeSource{CSI: &corev1.CSIVolumeSource{Driver: "secrets-store.csi.k8s.io"}}),
			)
		}
	}

	const (
		db0     = "pvc-0c1d2e3f-4a5b-6c7d-8e9f-0a1b2c3d4e5f"
		db1     = "pvc-9f8e7d6c-5b4a-3c2d-1e0f-a9b8c7d6e5f4"
		scratch = "pvc-11111111-2222-3333-4444-555555555555"
	)

	o.PersistentVolumeClaims = []corev1.PersistentVolumeClaim{
		claim("data-db-0", "fast", db0, "10Gi"),
		claim("data-db-1", "fast", db1, "10Gi"),
		claim("migrate-8h7g6-scratch", "fast", scratch, "1Gi"),
		claim("uploads", "standard", "", "50Gi"),
	}

	for _, name := range []string{db0, db1, scratch} {
		o.PersistentVolumes = append(o.PersistentVolumes, corev1.PersistentVolume{
			Name: name,
			Spec: corev1.PersistentVolumeSpec{StorageClassName: "fast"},
		})
	}

	o.StorageClasses = []storagev1.StorageClass{
		{Name: "fast", Provisioner: "ebs.csi.aws.com"},
		{Name: "standard", Provisioner: "rancher.io/local-path"},
	}

	return o
}

func TestRenderStorage(t *testing.T) {
	dot := render(t, testStorageObjects())

	assertContains(t, dot,
		`"db-\n0"`, `"data-db-0"`, `"10Gi RWO"`, `"pvc-0c1d2e3f-\n4a5b-6c7d-8e9f-\n0a1b2c3d4e5f"`,
		`"fast\n(ebs.csi.aws.\ncom)"`, `"standard\n(rancher.io/\nlocal-path)"`, `"Pending"`,
		`"web-config"`, `"web-tls"`, `"api-credentials"`, `"cache\n(emptyDir)"`, `"/var/log\n(hostPath)"`,
		`"nfs.example.com\n/exports/shared\n(nfs)"`, `"migrate-8h7g6-\nscratch"`, `"ephemeral"`,
		`"secrets\n(secrets-store.\ncsi.k8s.io)"`,
	)

	for _, unwanted := range []string{"kube-root-ca.crt", "svc.png", "ing.png", "Internet"} {
		if strings.Contains(dot, unwanted) {
			t.Errorf("storage diagram contains %s", unwanted)
		}
	}
}

func TestWrapLine(t *testing.T) {
	tests := []struct {
		line string
		want []string
	}{
		{"web", []string{"web"}},
		{"log-agent-config", []string{"log-agent-config"}},
		{"log-agent-configuration", []string{"log-agent-", "configuration"}},
		{"payments.example.com", []string{"payments.", "example.com"}},
		{"pvc-0c1d2e3f-4a5b-6c7d-8e9f-0a1b2c3d4e5f", []string{"pvc-0c1d2e3f-", "4a5b-6c7d-8e9f-", "0a1b2c3d4e5f"}},
		{"averyveryverylongname", []string{"averyveryverylongname"}},
	}

	for _, tt := range tests {
		if got := wrapLine(tt.line, labelLineLength); !slices.Equal(got, tt.want) {
			t.Errorf("wrapLine(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}
