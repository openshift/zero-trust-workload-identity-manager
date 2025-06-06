package spire_oidc_discovery_provider

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestBuildDeployment(t *testing.T) {
	tests := []struct {
		name     string
		config   *v1alpha1.SpireOIDCDiscoveryProviderConfig
		hash     string
		expected func(*appsv1.Deployment)
	}{
		{
			name: "basic deployment with default settings",
			config: &v1alpha1.SpireOIDCDiscoveryProviderConfig{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{},
			},
			hash: "test-hash-123",
			expected: func(deployment *appsv1.Deployment) {
				assert.Equal(t, "spire-spiffe-oidc-discovery-provider", deployment.Name)
				assert.Equal(t, utils.OperatorNamespace, deployment.Namespace)
				assert.Equal(t, int32(1), *deployment.Spec.Replicas)
				assert.Equal(t, "test-hash-123", deployment.Annotations[spireOidcDeploymentSpireOidcConfigHashAnnotationKey])
			},
		},
		{
			name: "deployment with custom replica count",
			config: &v1alpha1.SpireOIDCDiscoveryProviderConfig{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
					ReplicaCount: 3,
				},
			},
			hash: "test-hash-456",
			expected: func(deployment *appsv1.Deployment) {
				assert.Equal(t, int32(3), *deployment.Spec.Replicas)
			},
		},
		{
			name: "deployment with custom labels",
			config: &v1alpha1.SpireOIDCDiscoveryProviderConfig{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{
							"custom-label":           "custom-value",
							"environment":            "test",
							"app.kubernetes.io/name": "override-name", // This should override the default
						},
					},
				},
			},
			hash: "test-hash-789",
			expected: func(deployment *appsv1.Deployment) {
				labels := deployment.Labels
				assert.Equal(t, "custom-value", labels["custom-label"])
				assert.Equal(t, "test", labels["environment"])
				assert.Equal(t, "override-name", labels["app.kubernetes.io/name"])
				assert.Equal(t, "spire", labels["app.kubernetes.io/instance"])
				assert.Equal(t, utils.AppManagedByLabelValue, labels[utils.AppManagedByLabelKey])
			},
		},
		{
			name: "deployment with node selector",
			config: &v1alpha1.SpireOIDCDiscoveryProviderConfig{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
					CommonConfig: v1alpha1.CommonConfig{
						NodeSelector: map[string]string{
							"node-type": "compute",
							"zone":      "us-west-1a",
						},
					},
				},
			},
			hash: "test-hash-node",
			expected: func(deployment *appsv1.Deployment) {
				assert.Equal(t, map[string]string{
					"node-type": "compute",
					"zone":      "us-west-1a",
				}, deployment.Spec.Template.Spec.NodeSelector)
			},
		},
		{
			name: "deployment with affinity",
			config: &v1alpha1.SpireOIDCDiscoveryProviderConfig{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Affinity: &corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "node-type",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{"compute"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			hash: "test-hash-affinity",
			expected: func(deployment *appsv1.Deployment) {
				require.NotNil(t, deployment.Spec.Template.Spec.Affinity)
				require.NotNil(t, deployment.Spec.Template.Spec.Affinity.NodeAffinity)
			},
		},
		{
			name: "deployment with tolerations",
			config: &v1alpha1.SpireOIDCDiscoveryProviderConfig{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Tolerations: []*corev1.Toleration{
							{
								Key:      "node-role",
								Operator: corev1.TolerationOpEqual,
								Value:    "master",
								Effect:   corev1.TaintEffectNoSchedule,
							},
							{
								Key:      "dedicated",
								Operator: corev1.TolerationOpExists,
								Effect:   corev1.TaintEffectNoExecute,
							},
						},
					},
				},
			},
			hash: "test-hash-tolerations",
			expected: func(deployment *appsv1.Deployment) {
				tolerations := deployment.Spec.Template.Spec.Tolerations
				require.Len(t, tolerations, 2)
				assert.Equal(t, "node-role", tolerations[0].Key)
				assert.Equal(t, corev1.TolerationOpEqual, tolerations[0].Operator)
				assert.Equal(t, "master", tolerations[0].Value)
				assert.Equal(t, corev1.TaintEffectNoSchedule, tolerations[0].Effect)
				assert.Equal(t, "dedicated", tolerations[1].Key)
				assert.Equal(t, corev1.TolerationOpExists, tolerations[1].Operator)
			},
		},
		{
			name: "deployment with resource requirements",
			config: &v1alpha1.SpireOIDCDiscoveryProviderConfig{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
					},
				},
			},
			hash: "test-hash-resources",
			expected: func(deployment *appsv1.Deployment) {
				assert.Equal(t, "spire-spiffe-oidc-discovery-provider", deployment.Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment := buildDeployment(tt.config, tt.hash)

			// Common assertions for all tests
			require.NotNil(t, deployment)
			assert.Equal(t, "spire-spiffe-oidc-discovery-provider", deployment.Name)
			assert.Equal(t, utils.OperatorNamespace, deployment.Namespace)

			// Check default labels are always present
			expectedLabels := map[string]string{
				"app.kubernetes.io/name":     "spiffe-oidc-discovery-provider",
				"app.kubernetes.io/instance": "spire",
				"component":                  "oidc-discovery-provider",
				"release":                    "spire",
				"release-namespace":          "zero-trust-workload-identity-manager",
				utils.AppManagedByLabelKey:   utils.AppManagedByLabelValue,
			}

			for k, v := range expectedLabels {
				if tt.config.Spec.Labels == nil || tt.config.Spec.Labels[k] == "" {
					assert.Equal(t, v, deployment.Labels[k], "Expected default label %s=%s", k, v)
				}
			}

			// Check selector
			assert.Equal(t, "spiffe-oidc-discovery-provider", deployment.Spec.Selector.MatchLabels["app.kubernetes.io/name"])
			assert.Equal(t, "spire", deployment.Spec.Selector.MatchLabels["app.kubernetes.io/instance"])

			// Check service account
			assert.Equal(t, "spire-spiffe-oidc-discovery-provider", deployment.Spec.Template.Spec.ServiceAccountName)

			// Check containers
			require.Len(t, deployment.Spec.Template.Spec.Containers, 2)

			oidcContainer := deployment.Spec.Template.Spec.Containers[0]
			assert.Equal(t, "spiffe-oidc-discovery-provider", oidcContainer.Name)
			assert.Equal(t, utils.GetSpireOIDCDiscoveryProviderImage(), oidcContainer.Image)
			assert.Contains(t, oidcContainer.Args, "-config")
			assert.Contains(t, oidcContainer.Args, "/run/spire/oidc/config/oidc-discovery-provider.conf")

			helperContainer := deployment.Spec.Template.Spec.Containers[1]
			assert.Equal(t, "spiffe-helper", helperContainer.Name)
			assert.Equal(t, utils.GetSpiffeHelperImage(), helperContainer.Image)

			// Check init container
			require.Len(t, deployment.Spec.Template.Spec.InitContainers, 1)
			initContainer := deployment.Spec.Template.Spec.InitContainers[0]
			assert.Equal(t, "init", initContainer.Name)
			assert.Equal(t, utils.GetSpiffeHelperImage(), initContainer.Image)
			assert.Contains(t, initContainer.Args, "-daemon-mode=false")

			// Check volumes
			volumeNames := make([]string, len(deployment.Spec.Template.Spec.Volumes))
			for i, vol := range deployment.Spec.Template.Spec.Volumes {
				volumeNames[i] = vol.Name
			}
			expectedVolumes := []string{
				"spiffe-workload-api",
				"spire-oidc-sockets",
				"spire-oidc-config",
				"certdir",
				"ngnix-tmp",
			}
			for _, expectedVol := range expectedVolumes {
				assert.Contains(t, volumeNames, expectedVol)
			}

			// Check probes
			assert.NotNil(t, oidcContainer.ReadinessProbe)
			assert.NotNil(t, oidcContainer.LivenessProbe)
			assert.Equal(t, "/ready", oidcContainer.ReadinessProbe.HTTPGet.Path)
			assert.Equal(t, "/live", oidcContainer.LivenessProbe.HTTPGet.Path)
			assert.Equal(t, intstr.FromString("healthz"), oidcContainer.ReadinessProbe.HTTPGet.Port)

			// Run test-specific assertions
			if tt.expected != nil {
				tt.expected(deployment)
			}
		})
	}
}

func TestBoolPtr(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		expected bool
	}{
		{
			name:     "true value",
			input:    true,
			expected: true,
		},
		{
			name:     "false value",
			input:    false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := boolPtr(tt.input)
			require.NotNil(t, result)
			assert.Equal(t, tt.expected, *result)
		})
	}
}
