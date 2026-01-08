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
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetSpireOIDCDiscoveryProviderServiceAccount(t *testing.T) {
	t.Run("without custom labels", func(t *testing.T) {
		sa := getSpireOIDCDiscoveryProviderServiceAccount(nil)

		if sa == nil {
			t.Fatal("Expected ServiceAccount, got nil")
		}

		if sa.Name != "spire-spiffe-oidc-discovery-provider" {
			t.Errorf("Expected ServiceAccount name 'spire-spiffe-oidc-discovery-provider', got '%s'", sa.Name)
		}

		if sa.Namespace != utils.GetOperatorNamespace() {
			t.Errorf("Expected ServiceAccount namespace '%s', got '%s'", utils.GetOperatorNamespace(), sa.Namespace)
		}

		// Check labels
		if len(sa.Labels) == 0 {
			t.Error("Expected ServiceAccount to have labels, got none")
		}

		// Check for required labels
		if val, ok := sa.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := sa.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentDiscovery {
			t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentDiscovery)
		}
	})

	t.Run("with custom labels", func(t *testing.T) {
		customLabels := map[string]string{
			"service-tier": "discovery",
			"zone":         "global",
		}

		sa := getSpireOIDCDiscoveryProviderServiceAccount(customLabels)

		if sa == nil {
			t.Fatal("Expected ServiceAccount, got nil")
		}

		// Check that custom labels are present
		if val, ok := sa.Labels["service-tier"]; !ok || val != "discovery" {
			t.Errorf("Expected custom label 'service-tier=discovery', got '%s'", val)
		}

		if val, ok := sa.Labels["zone"]; !ok || val != "global" {
			t.Errorf("Expected custom label 'zone=global', got '%s'", val)
		}

		// Check that standard labels are still present
		if val, ok := sa.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s to be preserved with custom labels", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := sa.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentDiscovery {
			t.Errorf("Expected label app.kubernetes.io/component=%s to be preserved with custom labels", utils.ComponentDiscovery)
		}
	})

	t.Run("preserves all asset labels", func(t *testing.T) {
		// Get labels without custom labels (these come from asset file)
		saWithoutCustom := getSpireOIDCDiscoveryProviderServiceAccount(nil)
		assetLabels := make(map[string]string)
		for k, v := range saWithoutCustom.Labels {
			assetLabels[k] = v
		}

		// Get labels with custom labels
		customLabels := map[string]string{
			"release": "v2.5.0",
		}
		saWithCustom := getSpireOIDCDiscoveryProviderServiceAccount(customLabels)

		// All asset labels should still be present
		for k, v := range assetLabels {
			if saWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, saWithCustom.Labels[k])
			}
		}

		// Custom label should also be present
		if val, ok := saWithCustom.Labels["release"]; !ok || val != "v2.5.0" {
			t.Errorf("Custom label was not added")
		}
	})
}

// newSATestReconciler creates a reconciler for ServiceAccount tests
func newSATestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireOidcDiscoveryProviderReconciler {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	return &SpireOidcDiscoveryProviderReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}
}

// TestReconcileServiceAccount tests the reconcileServiceAccount function
func TestReconcileServiceAccount(t *testing.T) {
	tests := []struct {
		name           string
		oidc           *v1alpha1.SpireOIDCDiscoveryProvider
		setupClient    func(*fakes.FakeCustomCtrlClient)
		createOnlyMode bool
		useEmptyScheme bool
		expectError    bool
		expectCreate   bool
		expectUpdate   bool
	}{
		{
			name: "create success",
			oidc: &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-spiffe-oidc-discovery-provider"))
				fc.CreateReturns(nil)
			},
			expectCreate: true,
		},
		{
			name: "create error",
			oidc: &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-spiffe-oidc-discovery-provider"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name: "get error",
			oidc: &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(errors.New("connection refused"))
			},
			expectError: true,
		},
		{
			name: "create only mode skips update",
			oidc: &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-spiffe-oidc-discovery-provider",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if sa, ok := obj.(*corev1.ServiceAccount); ok {
						*sa = *existingSA
					}
					return nil
				}
			},
			createOnlyMode: true,
		},
		{
			name: "update success",
			oidc: &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-spiffe-oidc-discovery-provider",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if sa, ok := obj.(*corev1.ServiceAccount); ok {
						*sa = *existingSA
					}
					return nil
				}
				fc.UpdateReturns(nil)
			},
			expectUpdate: true,
		},
		{
			name: "update error",
			oidc: &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-spiffe-oidc-discovery-provider",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if sa, ok := obj.(*corev1.ServiceAccount); ok {
						*sa = *existingSA
					}
					return nil
				}
				fc.UpdateReturns(errors.New("update conflict"))
			},
			expectError:  true,
			expectUpdate: true,
		},
		{
			name: "set controller ref error",
			oidc: &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient:    func(fc *fakes.FakeCustomCtrlClient) {},
			useEmptyScheme: true,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			var reconciler *SpireOidcDiscoveryProviderReconciler
			if tt.useEmptyScheme {
				reconciler = &SpireOidcDiscoveryProviderReconciler{
					ctrlClient:    fakeClient,
					ctx:           context.Background(),
					log:           logr.Discard(),
					scheme:        runtime.NewScheme(),
					eventRecorder: record.NewFakeRecorder(100),
				}
			} else {
				reconciler = newSATestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileServiceAccount(context.Background(), tt.oidc, statusMgr, tt.createOnlyMode)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if tt.expectCreate && fakeClient.CreateCallCount() != 1 {
				t.Errorf("Expected Create to be called once, called %d times", fakeClient.CreateCallCount())
			}
			if tt.expectUpdate && fakeClient.UpdateCallCount() != 1 {
				t.Errorf("Expected Update to be called once, called %d times", fakeClient.UpdateCallCount())
			}
			if !tt.expectUpdate && !tt.expectError && fakeClient.UpdateCallCount() != 0 {
				t.Error("Expected Update not to be called")
			}
		})
	}
}
