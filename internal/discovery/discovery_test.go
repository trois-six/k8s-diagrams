package discovery

import (
	"slices"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakediscovery "k8s.io/client-go/discovery/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayfake "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/fake"
)

const testNamespace = "app"

func meta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: testNamespace}
}

func newTestClient(t *testing.T, gatewayResources []string) (*Client, *kubefake.Clientset) {
	t.Helper()

	kube := kubefake.NewClientset(
		&corev1.Namespace{Name: testNamespace},
		&corev1.Pod{ObjectMeta: meta("web-1")},
		&corev1.Pod{Name: "other", Namespace: "other"},
		&corev1.Service{ObjectMeta: meta("web")},
		&discoveryv1.EndpointSlice{ObjectMeta: meta("web-abcde")},
		&appsv1.Deployment{ObjectMeta: meta("web")},
		&networkingv1.Ingress{ObjectMeta: meta("web")},
		&networkingv1.IngressClass{Name: "traefik"},
		&corev1.PersistentVolumeClaim{ObjectMeta: meta("data")},
		&corev1.PersistentVolume{Name: "pv-1"},
		&storagev1.StorageClass{Name: "fast"},
	)

	if gatewayResources != nil {
		list := &metav1.APIResourceList{GroupVersion: gatewayv1.GroupVersion.String()}
		for _, r := range gatewayResources {
			list.APIResources = append(list.APIResources, metav1.APIResource{Name: r})
		}

		fake, ok := kube.Discovery().(*fakediscovery.FakeDiscovery)
		if !ok {
			t.Fatalf("unexpected discovery client %T", kube.Discovery())
		}

		fake.Resources = []*metav1.APIResourceList{list}
	}

	gateway := gatewayfake.NewSimpleClientset(
		&gatewayv1.GatewayClass{Name: "envoy"},
		&gatewayv1.HTTPRoute{
			ObjectMeta: meta("web"),
			Spec: gatewayv1.HTTPRouteSpec{
				CommonRouteSpec: gatewayv1.CommonRouteSpec{ParentRefs: []gatewayv1.ParentReference{{Name: "gw"}}},
				Rules: []gatewayv1.HTTPRouteRule{{
					BackendRefs: []gatewayv1.HTTPBackendRef{{
						Name: "web",
					}},
				}},
			},
		},
		&gatewayv1.TCPRoute{ObjectMeta: meta("db")},
	)

	// Created through the typed client, as the fake object tracker guesses "gatewaies" as the resource of a Gateway.
	_, err := gateway.GatewayV1().Gateways(testNamespace).
		Create(t.Context(), &gatewayv1.Gateway{ObjectMeta: meta("gw")}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("creating gateway: %v", err)
	}

	return &Client{kube: kube, gateway: gateway}, kube
}

func TestDiscover(t *testing.T) {
	client, _ := newTestClient(t, []string{"gatewayclasses", "gateways", "httproutes", "tcproutes"})

	o, err := client.Discover(t.Context(), testNamespace, ModeNetwork)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	counts := map[string]int{
		"pods":           len(o.Pods),
		"services":       len(o.Services),
		"endpointslices": len(o.EndpointSlices),
		"deployments":    len(o.Deployments),
		"ingresses":      len(o.Ingresses),
		"ingressclasses": len(o.IngressClasses),
		"gatewayclasses": len(o.GatewayClasses),
		"gateways":       len(o.Gateways),
		"routes":         len(o.Routes),
	}
	want := map[string]int{
		"pods":           1,
		"services":       1,
		"endpointslices": 1,
		"deployments":    1,
		"ingresses":      1,
		"ingressclasses": 1,
		"gatewayclasses": 1,
		"gateways":       1,
		"routes":         2,
	}

	for resource, n := range want {
		if counts[resource] != n {
			t.Errorf("got %d %s, want %d", counts[resource], resource, n)
		}
	}

	i := slices.IndexFunc(o.Routes, func(r Route) bool { return r.Kind == KindHTTPRoute })
	if i < 0 {
		t.Fatalf("no %s in %+v", KindHTTPRoute, o.Routes)
	}

	if r := o.Routes[i]; len(r.ParentRefs) != 1 || len(r.BackendRefs) != 1 || r.BackendRefs[0].Name != "web" {
		t.Errorf("unexpected HTTPRoute: %+v", r)
	}
}

func TestDiscoverStorage(t *testing.T) {
	client, _ := newTestClient(t, []string{"gateways", "httproutes"})

	o, err := client.Discover(t.Context(), testNamespace, ModeStorage)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(o.Pods) != 1 || len(o.Deployments) != 1 {
		t.Errorf("got %d pods and %d deployments, want 1 and 1", len(o.Pods), len(o.Deployments))
	}

	if len(o.PersistentVolumeClaims) != 1 || len(o.PersistentVolumes) != 1 || len(o.StorageClasses) != 1 {
		t.Errorf("got %d persistentvolumeclaims, %d persistentvolumes and %d storageclasses, want 1 of each",
			len(o.PersistentVolumeClaims), len(o.PersistentVolumes), len(o.StorageClasses))
	}

	if len(o.Services) != 0 || len(o.Ingresses) != 0 || len(o.Gateways) != 0 || len(o.Routes) != 0 {
		t.Errorf("got network objects in storage mode: %+v", o)
	}
}

func TestDiscoverWithoutGatewayAPI(t *testing.T) {
	client, _ := newTestClient(t, nil)

	o, err := client.Discover(t.Context(), testNamespace, ModeNetwork)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(o.Gateways) != 0 || len(o.Routes) != 0 {
		t.Errorf("got gateway API objects without the gateway API: %+v, %+v", o.Gateways, o.Routes)
	}
}

func TestDiscoverForbiddenClusterScoped(t *testing.T) {
	client, kube := newTestClient(t, nil)
	kube.PrependReactor("list", "ingressclasses", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "ingressclasses"}, "", nil)
	})

	o, err := client.Discover(t.Context(), testNamespace, ModeNetwork)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(o.IngressClasses) != 0 || len(o.Ingresses) != 1 {
		t.Errorf("got %d ingressclasses and %d ingresses, want 0 and 1", len(o.IngressClasses), len(o.Ingresses))
	}
}

func TestDiscoverUnknownNamespace(t *testing.T) {
	client, _ := newTestClient(t, nil)

	if _, err := client.Discover(t.Context(), "unknown", ModeNetwork); !apierrors.IsNotFound(err) {
		t.Errorf("Discover() error = %v, want a not found error", err)
	}
}
