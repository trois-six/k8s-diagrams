package discovery

import (
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// from https://github.com/traefik/traefik/blob/master/pkg/provider/kubernetes/ingress/client.go
func addServiceFromV1Beta1(ing *networkingv1.Ingress, old networkingv1beta1.Ingress) {
	if old.Spec.Backend != nil {
		port := networkingv1.ServiceBackendPort{}
		if old.Spec.Backend.ServicePort.Type == intstr.Int {
			port.Number = old.Spec.Backend.ServicePort.IntVal
		} else {
			port.Name = old.Spec.Backend.ServicePort.StrVal
		}

		if old.Spec.Backend.ServiceName != "" {
			ing.Spec.DefaultBackend = &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: old.Spec.Backend.ServiceName,
					Port: port,
				},
			}
		}
	}

	for rc, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}

		for pc, path := range rule.HTTP.Paths {
			if path.Backend.Service == nil {
				oldBackend := old.Spec.Rules[rc].HTTP.Paths[pc].Backend

				port := networkingv1.ServiceBackendPort{}
				if oldBackend.ServicePort.Type == intstr.Int {
					port.Number = oldBackend.ServicePort.IntVal
				} else {
					port.Name = oldBackend.ServicePort.StrVal
				}

				svc := networkingv1.IngressServiceBackend{
					Name: oldBackend.ServiceName,
					Port: port,
				}

				ing.Spec.Rules[rc].HTTP.Paths[pc].Backend.Service = &svc
			}
		}
	}
}

func toNetworkingV1(ing networkingv1beta1.Ingress) (*networkingv1.Ingress, error) {
	data, err := ing.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling ingress from v1beta1: %w", err)
	}

	ni := &networkingv1.Ingress{}

	err = ni.Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling ingress to v1: %w", err)
	}

	return ni, nil
}
