package utils

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// PortEgressNetworkPolicy builds a port-only egress NetworkPolicy for an operand pod selector.
func PortEgressNetworkPolicy(name, namespace string, podSelectorName string, port int32, labels map[string]string) *networkingv1.NetworkPolicy {
	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				AppManagedByLabelKey: AppManagedByLabelValue,
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": podSelectorName,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: protocolTCP(),
							Port:     intstrPort(port),
						},
					},
				},
			},
		},
	}
	for k, v := range labels {
		np.Labels[k] = v
	}
	return np
}

// MultiPortEgressNetworkPolicy builds egress allowing multiple TCP ports (e.g. HTTP and HTTPS proxy).
func MultiPortEgressNetworkPolicy(name, namespace string, podSelectorName string, ports []int32, labels map[string]string) *networkingv1.NetworkPolicy {
	npPorts := make([]networkingv1.NetworkPolicyPort, 0, len(ports))
	tcp := protocolTCP()
	for _, p := range ports {
		npPorts = append(npPorts, networkingv1.NetworkPolicyPort{
			Protocol: tcp,
			Port:     intstrPort(p),
		})
	}
	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				AppManagedByLabelKey: AppManagedByLabelValue,
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": podSelectorName,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{Ports: npPorts},
			},
		},
	}
	for k, v := range labels {
		np.Labels[k] = v
	}
	return np
}

func protocolTCP() *corev1.Protocol {
	p := corev1.ProtocolTCP
	return &p
}

func intstrPort(port int32) *intstr.IntOrString {
	p := intstr.FromInt32(port)
	return &p
}
