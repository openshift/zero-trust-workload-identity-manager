package spire_oidc_discovery_provider

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestBuildDeployment(t *testing.T) {
	tests := []struct {
		name     string
		config   *v1alpha1.SpireOIDCDiscoveryProvider
		hash     string
		expected func(*appsv1.Deployment)
	}{
		{
			name: "basic deployment with default settings",
			config: &v1alpha1.SpireOIDCDiscoveryProvider{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{},
			},
			hash: "test-hash-123",
			expected: func(deployment *appsv1.Deployment) {
				assert.Equal(t, "spire-spiffe-oidc-discovery-provider", deployment.Name)
				assert.Equal(t, utils.GetOperatorNamespace(), deployment.Namespace)
				assert.Equal(t, int32(1), *deployment.Spec.Replicas)
				// Verify the annotation is on the pod template, not on the deployment itself
				assert.Equal(t, "test-hash-123", deployment.Spec.Template.Annotations[spireOidcDeploymentSpireOidcConfigHashAnnotationKey])
				// Verify the deployment itself doesn't have this annotation
				_, exists := deployment.Annotations[spireOidcDeploymentSpireOidcConfigHashAnnotationKey]
				assert.False(t, exists, "Deployment annotations should not contain the config hash")
			},
		},
		{
			name: "deployment with custom replica count",
			config: &v1alpha1.SpireOIDCDiscoveryProvider{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
					ReplicaCount: 3,
				},
			},
			hash: "test-hash-456",
			expected: func(deployment *appsv1.Deployment) {
				assert.Equal(t, int32(3), *deployment.Spec.Replicas)
			},
		},
		{
			name: "config hash annotation is placed on pod template only",
			config: &v1alpha1.SpireOIDCDiscoveryProvider{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{},
			},
			hash: "config-hash-xyz",
			expected: func(deployment *appsv1.Deployment) {
				// The config hash annotation should be on the pod template
				podAnnotations := deployment.Spec.Template.Annotations
				require.NotNil(t, podAnnotations, "Pod template annotations should not be nil")
				assert.Equal(t, "config-hash-xyz", podAnnotations[spireOidcDeploymentSpireOidcConfigHashAnnotationKey],
					"Config hash should be in pod template annotations")

				// The config hash annotation should NOT be on the deployment itself
				deploymentAnnotations := deployment.Annotations
				if deploymentAnnotations != nil {
					_, exists := deploymentAnnotations[spireOidcDeploymentSpireOidcConfigHashAnnotationKey]
					assert.False(t, exists, "Config hash should not be in deployment annotations")
				}
			},
		},
		{
			name: "deployment with custom labels",
			config: &v1alpha1.SpireOIDCDiscoveryProvider{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
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
				// Standard labels take priority and cannot be overridden
				assert.Equal(t, "spiffe-oidc-discovery-provider", labels["app.kubernetes.io/name"])
				assert.Equal(t, utils.StandardInstance, labels["app.kubernetes.io/instance"])
				assert.Equal(t, utils.StandardManagedByValue, labels["app.kubernetes.io/managed-by"])
				// Verify standardized labels are applied
				assert.Equal(t, utils.ComponentDiscovery, labels["app.kubernetes.io/component"])
				assert.Equal(t, utils.StandardPartOfValue, labels["app.kubernetes.io/part-of"])
			},
		},
		{
			name: "deployment with node selector",
			config: &v1alpha1.SpireOIDCDiscoveryProvider{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
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
			config: &v1alpha1.SpireOIDCDiscoveryProvider{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
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
			config: &v1alpha1.SpireOIDCDiscoveryProvider{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
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
			config: &v1alpha1.SpireOIDCDiscoveryProvider{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
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
		{
			name: "deployment with custom CSI driver name",
			config: &v1alpha1.SpireOIDCDiscoveryProvider{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
					CSIDriverName: "csi.example.com",
				},
			},
			hash: "test-hash-csi",
			expected: func(deployment *appsv1.Deployment) {
				// Find the spiffe-workload-api volume
				var csiVolume *corev1.Volume
				for i := range deployment.Spec.Template.Spec.Volumes {
					if deployment.Spec.Template.Spec.Volumes[i].Name == "spiffe-workload-api" {
						csiVolume = &deployment.Spec.Template.Spec.Volumes[i]
						break
					}
				}
				require.NotNil(t, csiVolume, "spiffe-workload-api volume should exist")
				require.NotNil(t, csiVolume.CSI, "Volume should use CSI")
				assert.Equal(t, "csi.example.com", csiVolume.CSI.Driver, "CSI driver name should match config")
				assert.True(t, *csiVolume.CSI.ReadOnly, "CSI volume should be read-only")
			},
		},
		{
			name: "deployment uses default CSI driver name when not specified",
			config: &v1alpha1.SpireOIDCDiscoveryProvider{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
					// CSIDriverName not specified, should use default
				},
			},
			hash: "test-hash-default-csi",
			expected: func(deployment *appsv1.Deployment) {
				// Find the spiffe-workload-api volume
				var csiVolume *corev1.Volume
				for i := range deployment.Spec.Template.Spec.Volumes {
					if deployment.Spec.Template.Spec.Volumes[i].Name == "spiffe-workload-api" {
						csiVolume = &deployment.Spec.Template.Spec.Volumes[i]
						break
					}
				}
				require.NotNil(t, csiVolume, "spiffe-workload-api volume should exist")
				require.NotNil(t, csiVolume.CSI, "Volume should use CSI")
				// When CSIDriverName is not set, it should default to "csi.spiffe.io"
				assert.Equal(t, "csi.spiffe.io", csiVolume.CSI.Driver, "CSI driver name should default to csi.spiffe.io")
				assert.True(t, *csiVolume.CSI.ReadOnly, "CSI volume should be read-only")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployment := generateDeployment(tt.config, tt.hash)

			// Common assertions for all tests
			require.NotNil(t, deployment)
			assert.Equal(t, "spire-spiffe-oidc-discovery-provider", deployment.Name)
			assert.Equal(t, utils.GetOperatorNamespace(), deployment.Namespace)

			// Check labels using centralized approach
			expectedLabels := utils.SpireOIDCDiscoveryProviderLabels(tt.config.Spec.Labels)
			assert.Equal(t, expectedLabels, deployment.Labels, "Expected standardized labels")

			// Check selector labels using centralized approach
			expectedSelectorLabels := map[string]string{
				"app.kubernetes.io/name":      expectedLabels["app.kubernetes.io/name"],
				"app.kubernetes.io/instance":  expectedLabels["app.kubernetes.io/instance"],
				"app.kubernetes.io/component": expectedLabels["app.kubernetes.io/component"],
			}
			assert.Equal(t, expectedSelectorLabels, deployment.Spec.Selector.MatchLabels, "Expected standardized selector labels")

			// Check service account
			assert.Equal(t, "spire-spiffe-oidc-discovery-provider", deployment.Spec.Template.Spec.ServiceAccountName)

			// Check containers
			require.Len(t, deployment.Spec.Template.Spec.Containers, 1)

			oidcContainer := deployment.Spec.Template.Spec.Containers[0]
			assert.Equal(t, "spiffe-oidc-discovery-provider", oidcContainer.Name)
			assert.Equal(t, utils.GetSpireOIDCDiscoveryProviderImage(), oidcContainer.Image)
			assert.Contains(t, oidcContainer.Args, "-config")
			assert.Contains(t, oidcContainer.Args, "/run/spire/oidc/config/oidc-discovery-provider.conf")

			// Check that init containers are not present
			require.Len(t, deployment.Spec.Template.Spec.InitContainers, 0)

			// Check volumes
			volumeNames := make([]string, len(deployment.Spec.Template.Spec.Volumes))
			for i, vol := range deployment.Spec.Template.Spec.Volumes {
				volumeNames[i] = vol.Name
			}
			expectedVolumes := []string{
				"spiffe-workload-api",
				"spire-oidc-sockets",
				"spire-oidc-config",
				"tls-certs",
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

// TestReconcileDeployment tests the reconcileDeployment function
func TestReconcileDeployment(t *testing.T) {
	t.Run("create success", func(t *testing.T) {
		fakeClient := &fakes.FakeCustomCtrlClient{}
		reconciler := newDeploymentTestReconciler(fakeClient)

		oidc := createDeploymentTestOIDCCR()
		statusMgr := status.NewManager(fakeClient)

		fakeClient.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-spiffe-oidc-discovery-provider"))
		fakeClient.CreateReturns(nil)

		err := reconciler.reconcileDeployment(context.Background(), oidc, statusMgr, false, "test-hash")

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if fakeClient.CreateCallCount() != 1 {
			t.Errorf("Expected Create to be called once, got %d", fakeClient.CreateCallCount())
		}
	})

	t.Run("create error", func(t *testing.T) {
		fakeClient := &fakes.FakeCustomCtrlClient{}
		reconciler := newDeploymentTestReconciler(fakeClient)

		oidc := createDeploymentTestOIDCCR()
		statusMgr := status.NewManager(fakeClient)

		fakeClient.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-spiffe-oidc-discovery-provider"))
		fakeClient.CreateReturns(errors.New("create failed"))

		err := reconciler.reconcileDeployment(context.Background(), oidc, statusMgr, false, "test-hash")

		if err == nil {
			t.Error("Expected error when Create fails")
		}
	})

	t.Run("get error", func(t *testing.T) {
		fakeClient := &fakes.FakeCustomCtrlClient{}
		reconciler := newDeploymentTestReconciler(fakeClient)

		oidc := createDeploymentTestOIDCCR()
		statusMgr := status.NewManager(fakeClient)

		fakeClient.GetReturns(errors.New("connection refused"))

		err := reconciler.reconcileDeployment(context.Background(), oidc, statusMgr, false, "test-hash")

		if err == nil {
			t.Error("Expected error when Get fails")
		}
	})

	t.Run("update success", func(t *testing.T) {
		fakeClient := &fakes.FakeCustomCtrlClient{}
		reconciler := newDeploymentTestReconciler(fakeClient)

		oidc := createDeploymentTestOIDCCR()
		oidc.Spec.ReplicaCount = 3
		statusMgr := status.NewManager(fakeClient)

		replicas := int32(1)
		existingDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "spire-spiffe-oidc-discovery-provider",
				Namespace:       utils.GetOperatorNamespace(),
				ResourceVersion: "123",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
			},
		}

		fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
			if deploy, ok := obj.(*appsv1.Deployment); ok {
				*deploy = *existingDeployment
			}
			return nil
		}
		fakeClient.UpdateReturns(nil)

		err := reconciler.reconcileDeployment(context.Background(), oidc, statusMgr, false, "new-hash")

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if fakeClient.UpdateCallCount() != 1 {
			t.Errorf("Expected Update to be called once, got %d", fakeClient.UpdateCallCount())
		}
	})

	t.Run("update error", func(t *testing.T) {
		fakeClient := &fakes.FakeCustomCtrlClient{}
		reconciler := newDeploymentTestReconciler(fakeClient)

		oidc := createDeploymentTestOIDCCR()
		oidc.Spec.ReplicaCount = 3
		statusMgr := status.NewManager(fakeClient)

		replicas := int32(1)
		existingDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "spire-spiffe-oidc-discovery-provider",
				Namespace:       utils.GetOperatorNamespace(),
				ResourceVersion: "123",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
			},
		}

		fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
			if deploy, ok := obj.(*appsv1.Deployment); ok {
				*deploy = *existingDeployment
			}
			return nil
		}
		fakeClient.UpdateReturns(errors.New("update conflict"))

		err := reconciler.reconcileDeployment(context.Background(), oidc, statusMgr, false, "new-hash")

		if err == nil {
			t.Error("Expected error when Update fails")
		}
	})

	t.Run("create only mode skips update", func(t *testing.T) {
		fakeClient := &fakes.FakeCustomCtrlClient{}
		reconciler := newDeploymentTestReconciler(fakeClient)

		oidc := createDeploymentTestOIDCCR()
		oidc.Spec.ReplicaCount = 3
		statusMgr := status.NewManager(fakeClient)

		replicas := int32(1)
		existingDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "spire-spiffe-oidc-discovery-provider",
				Namespace:       utils.GetOperatorNamespace(),
				ResourceVersion: "123",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
			},
		}

		fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
			if deploy, ok := obj.(*appsv1.Deployment); ok {
				*deploy = *existingDeployment
			}
			return nil
		}

		err := reconciler.reconcileDeployment(context.Background(), oidc, statusMgr, true, "new-hash")

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if fakeClient.UpdateCallCount() != 0 {
			t.Error("Expected Update not to be called in create-only mode")
		}
	})

	t.Run("set controller reference error", func(t *testing.T) {
		fakeClient := &fakes.FakeCustomCtrlClient{}
		reconciler := &SpireOidcDiscoveryProviderReconciler{
			ctrlClient:    fakeClient,
			ctx:           context.Background(),
			log:           logr.Discard(),
			scheme:        runtime.NewScheme(), // Empty scheme causes error
			eventRecorder: record.NewFakeRecorder(100),
		}

		oidc := createDeploymentTestOIDCCR()
		statusMgr := status.NewManager(fakeClient)

		err := reconciler.reconcileDeployment(context.Background(), oidc, statusMgr, false, "test-hash")

		if err == nil {
			t.Error("Expected error when SetControllerReference fails")
		}
	})
}

// newDeploymentTestReconciler creates a reconciler for Deployment tests
func newDeploymentTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireOidcDiscoveryProviderReconciler {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	return &SpireOidcDiscoveryProviderReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}
}

// createDeploymentTestOIDCCR creates a test SpireOIDCDiscoveryProvider for Deployment tests
func createDeploymentTestOIDCCR() *v1alpha1.SpireOIDCDiscoveryProvider {
	return &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
			UID:  "test-uid",
		},
		Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
			JwtIssuer:    "https://oidc.example.org",
			ReplicaCount: 1,
		},
	}
}
