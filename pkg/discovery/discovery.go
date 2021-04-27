package discovery

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Objects struct {
	ConfigMaps             *corev1.ConfigMapList
	Endpoints              *corev1.EndpointsList
	Namespaces             *corev1.NamespaceList
	Pods                   *corev1.PodList
	PersistentVolumes      *corev1.PersistentVolumeList
	PersistentVolumeClaims *corev1.PersistentVolumeClaimList
	Secrets                *corev1.SecretList
	Services               *corev1.ServiceList
	DaemonSets             *appsv1.DaemonSetList
	Deployments            *appsv1.DeploymentList
	ReplicaSets            *appsv1.ReplicaSetList
	StatefulSets           *appsv1.StatefulSetList
	Ingresses              *networkingv1.IngressList
}

type Discovery struct {
	client  *kubernetes.Clientset
	ctx     context.Context
	objects *Objects
}

// NewDiscovery initialize a discovery of k8s objects.
func NewDiscovery(ctx context.Context, config *rest.Config) (Discovery, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		err = fmt.Errorf("creating kubernetes client: %w", err)
	}

	return Discovery{
		client:  clientset,
		ctx:     ctx,
		objects: &Objects{},
	}, err
}

// func (k *Discovery) generateSecrets(namespace string) error {
// 	secrets, err := k.client.CoreV1().Secrets(namespace).List(k.ctx, metav1.ListOptions{})
// 	if err != nil {
// 		return fmt.Errorf("getting secrets: %w", err)
// 	}

// 	filteredSecrets := &corev1.SecretList{}

// 	for _, secret := range secrets.Items {
// 		if !strings.HasPrefix(secret.ObjectMeta.Name, "sh.helm.release") {
// 			if secret.ObjectMeta.Annotations["kubectl.kubernetes.io/last-applied-configuration"] != "" {
// 				delete(secret.ObjectMeta.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
// 			}

// 			secret.Data = make(map[string][]byte)

// 			filteredSecrets.Items = append(filteredSecrets.Items, secret)
// 		}
// 	}

// 	k.objects.Secrets = filteredSecrets

// 	return nil
// }

func (k *Discovery) generateCore(namespace string) error {
	ns, err := k.client.CoreV1().Namespaces().List(k.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("getting namespaces: %w", err)
	}

	k.objects.Namespaces = ns

	// cm, err := k.client.CoreV1().ConfigMaps(namespace).List(k.ctx, metav1.ListOptions{})
	// if err != nil {
	// 	return fmt.Errorf("getting configmaps: %w", err)
	// }

	// k.objects.ConfigMaps = cm

	ep, err := k.client.CoreV1().Endpoints(namespace).List(k.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("getting endpoints: %w", err)
	}

	k.objects.Endpoints = ep

	po, err := k.client.CoreV1().Pods(namespace).List(k.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("getting pods: %w", err)
	}

	k.objects.Pods = po

	// pv, err := k.client.CoreV1().PersistentVolumes().List(k.ctx, metav1.ListOptions{})
	// if err != nil {
	// 	return fmt.Errorf("getting persitent volumes: %w", err)
	// }

	// k.objects.PersistentVolumes = pv

	// pvc, err := k.client.CoreV1().PersistentVolumeClaims(namespace).List(k.ctx, metav1.ListOptions{})
	// if err != nil {
	// 	return fmt.Errorf("getting persitent volumes claims: %w", err)
	// }

	// k.objects.PersistentVolumeClaims = pvc

	// if err = k.generateSecrets(namespace); err != nil {
	// 	return err
	// }

	svc, err := k.client.CoreV1().Services(namespace).List(k.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("getting services: %w", err)
	}

	k.objects.Services = svc

	return nil
}

func (k *Discovery) generateApps(namespace string) error {
	ds, err := k.client.AppsV1().DaemonSets(namespace).List(k.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("getting daemonsets: %w", err)
	}

	k.objects.DaemonSets = ds

	deploy, err := k.client.AppsV1().Deployments(namespace).List(k.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("getting deployments: %w", err)
	}

	k.objects.Deployments = deploy

	rs, err := k.client.AppsV1().ReplicaSets(namespace).List(k.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("getting replicasets: %w", err)
	}

	k.objects.ReplicaSets = rs

	sts, err := k.client.AppsV1().StatefulSets(namespace).List(k.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("getting statefulsets: %w", err)
	}

	k.objects.StatefulSets = sts

	return nil
}

func (k *Discovery) generateNetworking(namespace string) error {
	ing, err := k.client.NetworkingV1().Ingresses(namespace).List(k.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("getting ingresses: %w", err)
	}

	k.objects.Ingresses = ing

	return nil
}

// GenerateAll gets all kubernetes objects.
func (k *Discovery) GenerateAll(namespace string) (*Objects, error) {
	if err := k.generateCore(namespace); err != nil {
		return nil, err
	}

	if err := k.generateApps(namespace); err != nil {
		return nil, err
	}

	if err := k.generateNetworking(namespace); err != nil {
		return nil, err
	}

	return k.objects, nil
}
