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

func TestGetSpireOIDCDiscoveryProviderService(t *testing.T) {
	t.Run("without custom labels", func(t *testing.T) {
		svc := getSpireOIDCDiscoveryProviderService(nil)

		if svc == nil {
			t.Fatal("Expected Service, got nil")
		}

		if svc.Name != "spire-spiffe-oidc-discovery-provider" {
			t.Errorf("Expected Service name 'spire-spiffe-oidc-discovery-provider', got '%s'", svc.Name)
		}

		if svc.Namespace != utils.GetOperatorNamespace() {
			t.Errorf("Expected Service namespace '%s', got '%s'", utils.GetOperatorNamespace(), svc.Namespace)
		}

		// Check labels
		if len(svc.Labels) == 0 {
			t.Error("Expected Service to have labels, got none")
		}

		// Check for required labels
		if val, ok := svc.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := svc.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentDiscovery {
			t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentDiscovery)
		}

		// Check selectors
		if len(svc.Spec.Selector) == 0 {
			t.Error("Expected Service to have selectors, got none")
		}

		if val, ok := svc.Spec.Selector["app.kubernetes.io/name"]; !ok || val != "spiffe-oidc-discovery-provider" {
			t.Error("Expected selector app.kubernetes.io/name=spiffe-oidc-discovery-provider")
		}

		if val, ok := svc.Spec.Selector["app.kubernetes.io/instance"]; !ok || val != utils.StandardInstance {
			t.Errorf("Expected selector app.kubernetes.io/instance=%s", utils.StandardInstance)
		}
	})

	t.Run("with custom labels", func(t *testing.T) {
		customLabels := map[string]string{
			"discovery-type": "oidc",
			"public":         "true",
		}

		svc := getSpireOIDCDiscoveryProviderService(customLabels)

		if svc == nil {
			t.Fatal("Expected Service, got nil")
		}

		// Check that custom labels are present
		if val, ok := svc.Labels["discovery-type"]; !ok || val != "oidc" {
			t.Errorf("Expected custom label 'discovery-type=oidc', got '%s'", val)
		}

		if val, ok := svc.Labels["public"]; !ok || val != "true" {
			t.Errorf("Expected custom label 'public=true', got '%s'", val)
		}

		// Check that standard labels are still present
		if val, ok := svc.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s to be preserved with custom labels", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := svc.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentDiscovery {
			t.Errorf("Expected label app.kubernetes.io/component=%s to be preserved with custom labels", utils.ComponentDiscovery)
		}
	})

	t.Run("preserves all asset labels", func(t *testing.T) {
		// Get labels without custom labels (these come from asset file)
		svcWithoutCustom := getSpireOIDCDiscoveryProviderService(nil)
		assetLabels := make(map[string]string)
		for k, v := range svcWithoutCustom.Labels {
			assetLabels[k] = v
		}

		// Get labels with custom labels
		customLabels := map[string]string{
			"endpoint": "/.well-known/openid-configuration",
		}
		svcWithCustom := getSpireOIDCDiscoveryProviderService(customLabels)

		// All asset labels should still be present
		for k, v := range assetLabels {
			if svcWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, svcWithCustom.Labels[k])
			}
		}

		// Custom label should also be present
		if val, ok := svcWithCustom.Labels["endpoint"]; !ok || val != "/.well-known/openid-configuration" {
			t.Errorf("Custom label was not added")
		}
	})
}

// newSvcTestReconciler creates a reconciler for Service tests
func newSvcTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireOidcDiscoveryProviderReconciler {
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

// TestReconcileService tests the reconcileService function
func TestReconcileService(t *testing.T) {
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
				existingSvc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-spiffe-oidc-discovery-provider",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if svc, ok := obj.(*corev1.Service); ok {
						*svc = *existingSvc
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
				existingSvc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-spiffe-oidc-discovery-provider",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
					Spec: corev1.ServiceSpec{ClusterIP: "10.0.0.1"},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if svc, ok := obj.(*corev1.Service); ok {
						*svc = *existingSvc
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
				existingSvc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-spiffe-oidc-discovery-provider",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
					Spec: corev1.ServiceSpec{ClusterIP: "10.0.0.1"},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if svc, ok := obj.(*corev1.Service); ok {
						*svc = *existingSvc
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
				reconciler = newSvcTestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileService(context.Background(), tt.oidc, statusMgr, tt.createOnlyMode)

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
