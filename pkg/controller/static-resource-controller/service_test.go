package static_resource_controller

import (
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/version"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"testing"
)

func TestStaticResourceReconciler_ListStaticServiceResource(t *testing.T) {
	r := &StaticResourceReconciler{}

	services := r.listStaticServiceResource()
	assert.Len(t, services, 4)

	// Define expected info for each service in order
	expectedServices := []struct {
		name      string
		kind      string
		labels    map[string]string
		ports     []corev1.ServicePort
		selector  map[string]string
		namespace string
	}{
		{
			name:      "spire-server",
			kind:      "Service",
			namespace: "zero-trust-workload-identity-manager",
			labels: map[string]string{
				"app.kubernetes.io/name":       "spire-server",
				"app.kubernetes.io/instance":   "cluster-zero-trust-workload-identity-manager",
				"app.kubernetes.io/component":  "control-plane",
				"app.kubernetes.io/version":    version.SpireServerVersion,
				"app.kubernetes.io/managed-by": "zero-trust-workload-identity-manager",
				"app.kubernetes.io/part-of":    "zero-trust-workload-identity-manager",
			},
			ports: []corev1.ServicePort{
				{
					Name:       "grpc",
					Port:       443,
					TargetPort: intstrFromString("grpc"),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "metrics",
					Port:       9402,
					TargetPort: intstrFromInt(9402),
				},
			},

			selector: map[string]string{
				"app.kubernetes.io/name":     "spire-server",
				"app.kubernetes.io/instance": "cluster-zero-trust-workload-identity-manager",
			},
		},
		{
			name:      "spire-agent",
			kind:      "Service",
			namespace: "zero-trust-workload-identity-manager",
			labels: map[string]string{
				"app.kubernetes.io/name":       "spire-agent",
				"app.kubernetes.io/instance":   "cluster-zero-trust-workload-identity-manager",
				"app.kubernetes.io/component":  "node-agent",
				"app.kubernetes.io/version":    version.SpireAgentVersion,
				"app.kubernetes.io/managed-by": "zero-trust-workload-identity-manager",
				"app.kubernetes.io/part-of":    "zero-trust-workload-identity-manager",
			},
			ports: []corev1.ServicePort{
				{
					Name:       "metrics",
					Port:       9402,
					TargetPort: intstrFromInt(9402),
				},
			},
			selector: map[string]string{
				"app.kubernetes.io/name":     "spire-agent",
				"app.kubernetes.io/instance": "cluster-zero-trust-workload-identity-manager",
			},
		},
		{
			name:      "spire-spiffe-oidc-discovery-provider",
			kind:      "Service",
			namespace: "zero-trust-workload-identity-manager",
			labels: map[string]string{
				"app.kubernetes.io/name":       "spiffe-oidc-discovery-provider",
				"app.kubernetes.io/instance":   "cluster-zero-trust-workload-identity-manager",
				"app.kubernetes.io/component":  "discovery",
				"app.kubernetes.io/version":    version.SpireOIDCDiscoveryProviderVersion,
				"app.kubernetes.io/managed-by": "zero-trust-workload-identity-manager",
				"app.kubernetes.io/part-of":    "zero-trust-workload-identity-manager",
			},
			ports: []corev1.ServicePort{
				{
					Name:       "https",
					Port:       443,
					TargetPort: intstrFromString("https"),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			selector: map[string]string{
				"app.kubernetes.io/name":     "spiffe-oidc-discovery-provider",
				"app.kubernetes.io/instance": "cluster-zero-trust-workload-identity-manager",
			},
		},
		{
			name:      "spire-controller-manager-webhook",
			kind:      "Service",
			namespace: "zero-trust-workload-identity-manager",
			labels: map[string]string{
				"app.kubernetes.io/name":       "spire-controller-manager",
				"app.kubernetes.io/instance":   "cluster-zero-trust-workload-identity-manager",
				"app.kubernetes.io/component":  "control-plane",
				"app.kubernetes.io/version":    version.SpireControllerManagerVersion,
				"app.kubernetes.io/managed-by": "zero-trust-workload-identity-manager",
				"app.kubernetes.io/part-of":    "zero-trust-workload-identity-manager",
			},
			ports: []corev1.ServicePort{
				{
					Name:       "https",
					Port:       443,
					TargetPort: intstrFromString("https"),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			selector: map[string]string{
				"app.kubernetes.io/name":     "spire-controller-manager",
				"app.kubernetes.io/instance": "cluster-zero-trust-workload-identity-manager",
			},
		},
	}

	for i, svc := range services {
		expected := expectedServices[i]
		assert.Equal(t, expected.name, svc.Name)
		assert.Equal(t, expected.kind, svc.Kind)
		assert.Equal(t, expected.namespace, svc.Namespace)
		assert.Equal(t, expected.labels, svc.Labels)
		assert.Equal(t, expected.ports, svc.Spec.Ports)
		assert.Equal(t, expected.selector, svc.Spec.Selector)
	}

	// Also test individual getters similarly (optional)
	t.Run("getSpireServerService", func(t *testing.T) {
		svc := r.getSpireServerService()
		assert.Equal(t, "spire-server", svc.Name)
		assert.Equal(t, "Service", svc.Kind)
		assert.Equal(t, "zero-trust-workload-identity-manager", svc.Namespace)
		assert.Equal(t, expectedServices[0].labels, svc.Labels)
	})

	t.Run("getSpireAgentService", func(t *testing.T) {
		svc := r.getSpireAgentService()
		assert.Equal(t, "spire-agent", svc.Name)
		assert.Equal(t, "Service", svc.Kind)
		assert.Equal(t, "zero-trust-workload-identity-manager", svc.Namespace)
		assert.Equal(t, expectedServices[1].labels, svc.Labels)
	})

	t.Run("getSpireOIDCDiscoveryProviderService", func(t *testing.T) {
		svc := r.getSpireOIDCDiscoveryProviderService()
		assert.Equal(t, "spire-spiffe-oidc-discovery-provider", svc.Name)
		assert.Equal(t, "Service", svc.Kind)
		assert.Equal(t, "zero-trust-workload-identity-manager", svc.Namespace)
		assert.Equal(t, expectedServices[2].labels, svc.Labels)
	})

	t.Run("getSpireControllerMangerWebhookService", func(t *testing.T) {
		svc := r.getSpireControllerMangerWebhookService()
		assert.Equal(t, "spire-controller-manager-webhook", svc.Name)
		assert.Equal(t, "Service", svc.Kind)
		assert.Equal(t, "zero-trust-workload-identity-manager", svc.Namespace)
		assert.Equal(t, expectedServices[3].labels, svc.Labels)
	})
}

func TestGetSpireServerService(t *testing.T) {
	r := &StaticResourceReconciler{}
	svc := r.getSpireServerService()

	assert.Equal(t, "spire-server", svc.Name)
	assert.Equal(t, "Service", svc.Kind)
	assert.Equal(t, "zero-trust-workload-identity-manager", svc.Namespace)

	expectedLabels := map[string]string{
		"app.kubernetes.io/name":       "spire-server",
		"app.kubernetes.io/instance":   "cluster-zero-trust-workload-identity-manager",
		"app.kubernetes.io/component":  "control-plane",
		"app.kubernetes.io/version":    version.SpireServerVersion,
		"app.kubernetes.io/managed-by": "zero-trust-workload-identity-manager",
		"app.kubernetes.io/part-of":    "zero-trust-workload-identity-manager",
	}
	assert.Equal(t, expectedLabels, svc.Labels)

	assert.Len(t, svc.Spec.Ports, 2)
	assert.Equal(t, "grpc", svc.Spec.Ports[0].Name)
	assert.Equal(t, int32(443), svc.Spec.Ports[0].Port)
	assert.Equal(t, "grpc", svc.Spec.Ports[0].TargetPort.String())
	assert.Equal(t, corev1.ProtocolTCP, svc.Spec.Ports[0].Protocol)

	expectedSelector := map[string]string{
		"app.kubernetes.io/name":     "spire-server",
		"app.kubernetes.io/instance": "cluster-zero-trust-workload-identity-manager",
	}
	assert.Equal(t, expectedSelector, svc.Spec.Selector)
}

func TestGetSpireOIDCDiscoveryProviderService(t *testing.T) {
	r := &StaticResourceReconciler{}
	svc := r.getSpireOIDCDiscoveryProviderService()

	assert.Equal(t, "spire-spiffe-oidc-discovery-provider", svc.Name)
	assert.Equal(t, "Service", svc.Kind)
	assert.Equal(t, "zero-trust-workload-identity-manager", svc.Namespace)

	expectedLabels := map[string]string{
		"app.kubernetes.io/name":       "spiffe-oidc-discovery-provider",
		"app.kubernetes.io/instance":   "cluster-zero-trust-workload-identity-manager",
		"app.kubernetes.io/component":  "discovery",
		"app.kubernetes.io/version":    version.SpireOIDCDiscoveryProviderVersion,
		"app.kubernetes.io/managed-by": "zero-trust-workload-identity-manager",
		"app.kubernetes.io/part-of":    "zero-trust-workload-identity-manager",
	}
	assert.Equal(t, expectedLabels, svc.Labels)

	assert.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, "https", svc.Spec.Ports[0].Name)
	assert.Equal(t, int32(443), svc.Spec.Ports[0].Port)
	assert.Equal(t, "https", svc.Spec.Ports[0].TargetPort.String())
	assert.Equal(t, corev1.ProtocolTCP, svc.Spec.Ports[0].Protocol)

	expectedSelector := map[string]string{
		"app.kubernetes.io/name":     "spiffe-oidc-discovery-provider",
		"app.kubernetes.io/instance": "cluster-zero-trust-workload-identity-manager",
	}
	assert.Equal(t, expectedSelector, svc.Spec.Selector)
}

func TestGetSpireControllerMangerWebhookService(t *testing.T) {
	r := &StaticResourceReconciler{}
	svc := r.getSpireControllerMangerWebhookService()

	assert.Equal(t, "spire-controller-manager-webhook", svc.Name)
	assert.Equal(t, "Service", svc.Kind)
	assert.Equal(t, "zero-trust-workload-identity-manager", svc.Namespace)

	expectedLabels := map[string]string{
		"app.kubernetes.io/name":       "spire-controller-manager",
		"app.kubernetes.io/instance":   "cluster-zero-trust-workload-identity-manager",
		"app.kubernetes.io/component":  "control-plane",
		"app.kubernetes.io/version":    version.SpireControllerManagerVersion,
		"app.kubernetes.io/managed-by": "zero-trust-workload-identity-manager",
		"app.kubernetes.io/part-of":    "zero-trust-workload-identity-manager",
	}
	assert.Equal(t, expectedLabels, svc.Labels)

	assert.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, "https", svc.Spec.Ports[0].Name)
	assert.Equal(t, int32(443), svc.Spec.Ports[0].Port)
	assert.Equal(t, "https", svc.Spec.Ports[0].TargetPort.String())
	assert.Equal(t, corev1.ProtocolTCP, svc.Spec.Ports[0].Protocol)

	expectedSelector := map[string]string{
		"app.kubernetes.io/name":     "spire-controller-manager",
		"app.kubernetes.io/instance": "cluster-zero-trust-workload-identity-manager",
	}
	assert.Equal(t, expectedSelector, svc.Spec.Selector)
}

// helper to get intstr.IntOrString from string
func intstrFromString(s string) intstr.IntOrString {
	return intstr.IntOrString{Type: intstr.String, StrVal: s}
}

func intstrFromInt(i int32) intstr.IntOrString {
	return intstr.IntOrString{Type: intstr.Int, IntVal: i}
}
