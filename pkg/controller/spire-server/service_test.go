package spire_server

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

func TestGetSpireServerService(t *testing.T) {
	tests := []struct {
		name                      string
		config                    *v1alpha1.SpireServerSpec
		expectServiceName         string
		expectFederationPort      bool
		expectServiceCAAnnotation bool
		customLabels              map[string]string
	}{
		{
			name:                      "without custom labels and without federation",
			config:                    &v1alpha1.SpireServerSpec{},
			expectServiceName:         "spire-server",
			expectFederationPort:      false,
			expectServiceCAAnnotation: false,
		},
		{
			name: "with federation enabled",
			config: &v1alpha1.SpireServerSpec{
				Federation: &v1alpha1.FederationConfig{
					BundleEndpoint: v1alpha1.BundleEndpointConfig{
						Profile: v1alpha1.HttpsSpiffeProfile,
					},
				},
			},
			expectServiceName:         "spire-server",
			expectFederationPort:      true,
			expectServiceCAAnnotation: true,
		},
		{
			name: "with custom labels",
			config: &v1alpha1.SpireServerSpec{
				CommonConfig: v1alpha1.CommonConfig{
					Labels: map[string]string{
						"service-type": "control-plane",
						"priority":     "critical",
					},
				},
			},
			expectServiceName:         "spire-server",
			expectFederationPort:      false,
			expectServiceCAAnnotation: false,
			customLabels: map[string]string{
				"service-type": "control-plane",
				"priority":     "critical",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := getSpireServerService(tt.config)

			if svc == nil {
				t.Fatal("Expected Service, got nil")
			}

			if svc.Name != tt.expectServiceName {
				t.Errorf("Expected Service name '%s', got '%s'", tt.expectServiceName, svc.Name)
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

			if val, ok := svc.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentControlPlane {
				t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentControlPlane)
			}

			// Check selectors
			if len(svc.Spec.Selector) == 0 {
				t.Error("Expected Service to have selectors, got none")
			}

			if val, ok := svc.Spec.Selector["app.kubernetes.io/name"]; !ok || val != "spire-server" {
				t.Error("Expected selector app.kubernetes.io/name=spire-server")
			}

			if val, ok := svc.Spec.Selector["app.kubernetes.io/instance"]; !ok || val != utils.StandardInstance {
				t.Errorf("Expected selector app.kubernetes.io/instance=%s", utils.StandardInstance)
			}

			// Check service CA annotation
			_, hasAnnotation := svc.Annotations[utils.ServiceCAAnnotationKey]
			if tt.expectServiceCAAnnotation && !hasAnnotation {
				t.Error("Expected service CA annotation to be present")
			}
			if !tt.expectServiceCAAnnotation && hasAnnotation {
				t.Error("Expected service CA annotation to be absent")
			}

			// Check federation port
			federationPortFound := false
			for _, port := range svc.Spec.Ports {
				if port.Name == "federation" {
					federationPortFound = true
					if port.Port != 8443 {
						t.Errorf("Expected federation port 8443, got %d", port.Port)
					}
					break
				}
			}
			if tt.expectFederationPort && !federationPortFound {
				t.Error("Expected federation port to be present")
			}
			if !tt.expectFederationPort && federationPortFound {
				t.Error("Expected federation port to be absent")
			}

			// Check custom labels if specified
			for key, expectedValue := range tt.customLabels {
				if val, ok := svc.Labels[key]; !ok || val != expectedValue {
					t.Errorf("Expected custom label '%s=%s', got '%s'", key, expectedValue, val)
				}
			}
		})
	}

	// Test that asset labels are preserved when custom labels are added
	t.Run("preserves all asset labels", func(t *testing.T) {
		configWithoutCustom := &v1alpha1.SpireServerSpec{}
		svcWithoutCustom := getSpireServerService(configWithoutCustom)
		assetLabels := make(map[string]string)
		for k, v := range svcWithoutCustom.Labels {
			assetLabels[k] = v
		}

		customLabels := map[string]string{"cluster": "production"}
		configWithCustom := &v1alpha1.SpireServerSpec{
			CommonConfig: v1alpha1.CommonConfig{
				Labels: customLabels,
			},
		}
		svcWithCustom := getSpireServerService(configWithCustom)

		for k, v := range assetLabels {
			if svcWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, svcWithCustom.Labels[k])
			}
		}

		if val, ok := svcWithCustom.Labels["cluster"]; !ok || val != "production" {
			t.Errorf("Custom label was not added")
		}
	})
}

func TestGetSpireControllerManagerWebhookService(t *testing.T) {
	tests := []struct {
		name         string
		customLabels map[string]string
	}{
		{
			name:         "without custom labels",
			customLabels: nil,
		},
		{
			name: "with custom labels",
			customLabels: map[string]string{
				"webhook-type": "validating",
				"component":    "admission-control",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := getSpireControllerManagerWebhookService(tt.customLabels)

			if svc == nil {
				t.Fatal("Expected Service, got nil")
			}

			if svc.Name != "spire-controller-manager-webhook" {
				t.Errorf("Expected Service name 'spire-controller-manager-webhook', got '%s'", svc.Name)
			}

			// Check selectors
			if val, ok := svc.Spec.Selector["app.kubernetes.io/name"]; !ok || val != "spire-controller-manager" {
				t.Error("Expected selector app.kubernetes.io/name=spire-controller-manager")
			}

			// Check labels
			if len(svc.Labels) == 0 {
				t.Error("Expected Service to have labels, got none")
			}

			// Check custom labels if specified
			for key, expectedValue := range tt.customLabels {
				if val, ok := svc.Labels[key]; !ok || val != expectedValue {
					t.Errorf("Expected custom label '%s=%s', got '%s'", key, expectedValue, val)
				}
			}

			// Check that standard labels are present
			if val, ok := svc.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
				t.Errorf("Expected label %s=%s to be preserved", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
			}
		})
	}

	t.Run("preserves all asset labels", func(t *testing.T) {
		svcWithoutCustom := getSpireControllerManagerWebhookService(nil)
		assetLabels := make(map[string]string)
		for k, v := range svcWithoutCustom.Labels {
			assetLabels[k] = v
		}

		customLabels := map[string]string{"test": "value"}
		svcWithCustom := getSpireControllerManagerWebhookService(customLabels)

		for k, v := range assetLabels {
			if svcWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, svcWithCustom.Labels[k])
			}
		}

		if val, ok := svcWithCustom.Labels["test"]; !ok || val != "value" {
			t.Errorf("Custom label was not added")
		}
	})
}

// newServiceTestReconciler creates a reconciler for Service tests
func newServiceTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireServerReconciler {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	return &SpireServerReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}
}

func TestReconcileSpireServerService(t *testing.T) {
	tests := []struct {
		name           string
		server         *v1alpha1.SpireServer
		setupClient    func(*fakes.FakeCustomCtrlClient)
		createOnlyMode bool
		useEmptyScheme bool
		expectError    bool
		expectCreate   bool
		expectUpdate   bool
	}{
		{
			name: "create success",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-server"))
				fc.CreateReturns(nil)
			},
			expectError:  false,
			expectCreate: true,
		},
		{
			name: "create error",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-server"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name: "get error",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(errors.New("connection refused"))
			},
			expectError: true,
		},
		{
			name: "update success",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireServerSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingService := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-server",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
					Spec: corev1.ServiceSpec{ClusterIP: "10.0.0.1"},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if svc, ok := obj.(*corev1.Service); ok {
						*svc = *existingService
					}
					return nil
				}
				fc.UpdateReturns(nil)
			},
			expectError:  false,
			expectUpdate: true,
		},
		{
			name: "update error",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireServerSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingService := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-server",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
					Spec: corev1.ServiceSpec{ClusterIP: "10.0.0.1"},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if svc, ok := obj.(*corev1.Service); ok {
						*svc = *existingService
					}
					return nil
				}
				fc.UpdateReturns(errors.New("update conflict"))
			},
			expectError:  true,
			expectUpdate: true,
		},
		{
			name: "create only mode skips update",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireServerSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingService := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-server",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
					Spec: corev1.ServiceSpec{ClusterIP: "10.0.0.1"},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if svc, ok := obj.(*corev1.Service); ok {
						*svc = *existingService
					}
					return nil
				}
			},
			createOnlyMode: true,
			expectError:    false,
			expectUpdate:   false,
		},
		{
			name: "set controller ref error",
			server: &v1alpha1.SpireServer{
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
			var reconciler *SpireServerReconciler
			if tt.useEmptyScheme {
				reconciler = &SpireServerReconciler{
					ctrlClient:    fakeClient,
					ctx:           context.Background(),
					log:           logr.Discard(),
					scheme:        runtime.NewScheme(),
					eventRecorder: record.NewFakeRecorder(100),
				}
			} else {
				reconciler = newServiceTestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileSpireServerService(context.Background(), tt.server, statusMgr, tt.createOnlyMode)

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
			if !tt.expectUpdate && fakeClient.UpdateCallCount() != 0 {
				t.Error("Expected Update not to be called")
			}
		})
	}
}

func TestReconcileSpireControllerManagerService(t *testing.T) {
	tests := []struct {
		name           string
		server         *v1alpha1.SpireServer
		setupClient    func(*fakes.FakeCustomCtrlClient)
		createOnlyMode bool
		useEmptyScheme bool
		expectError    bool
		expectCreate   bool
		expectUpdate   bool
	}{
		{
			name: "create success",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-controller-manager-webhook"))
				fc.CreateReturns(nil)
			},
			expectError:  false,
			expectCreate: true,
		},
		{
			name: "get error",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(errors.New("connection refused"))
			},
			expectError: true,
		},
		{
			name: "create error",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-controller-manager-webhook"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name: "update success",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireServerSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingService := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-controller-manager-webhook",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
					Spec: corev1.ServiceSpec{ClusterIP: "10.0.0.2"},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if svc, ok := obj.(*corev1.Service); ok {
						*svc = *existingService
					}
					return nil
				}
				fc.UpdateReturns(nil)
			},
			expectError:  false,
			expectUpdate: true,
		},
		{
			name: "create only mode skips update",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireServerSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingService := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-controller-manager-webhook",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
					Spec: corev1.ServiceSpec{ClusterIP: "10.0.0.2"},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if svc, ok := obj.(*corev1.Service); ok {
						*svc = *existingService
					}
					return nil
				}
			},
			createOnlyMode: true,
			expectError:    false,
			expectUpdate:   false,
		},
		{
			name: "set controller ref error",
			server: &v1alpha1.SpireServer{
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
			var reconciler *SpireServerReconciler
			if tt.useEmptyScheme {
				reconciler = &SpireServerReconciler{
					ctrlClient:    fakeClient,
					ctx:           context.Background(),
					log:           logr.Discard(),
					scheme:        runtime.NewScheme(),
					eventRecorder: record.NewFakeRecorder(100),
				}
			} else {
				reconciler = newServiceTestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileSpireControllerManagerService(context.Background(), tt.server, statusMgr, tt.createOnlyMode)

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
			if !tt.expectUpdate && fakeClient.UpdateCallCount() != 0 {
				t.Error("Expected Update not to be called")
			}
		})
	}
}
