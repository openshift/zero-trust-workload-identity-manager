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
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetRBACResources(t *testing.T) {
	tests := []struct {
		name           string
		resourceType   string
		customLabels   map[string]string
		expectedName   string
		expectedNS     string
		checkComponent string
	}{
		{
			name:           "spire server cluster role without custom labels",
			resourceType:   "spireServerClusterRole",
			customLabels:   nil,
			expectedName:   "spire-server",
			checkComponent: utils.ComponentControlPlane,
		},
		{
			name:           "spire server cluster role binding without custom labels",
			resourceType:   "spireServerClusterRoleBinding",
			customLabels:   nil,
			expectedName:   "spire-server",
			checkComponent: utils.ComponentControlPlane,
		},
		{
			name:           "spire bundle role without custom labels",
			resourceType:   "spireBundleRole",
			customLabels:   nil,
			expectedName:   "spire-bundle",
			expectedNS:     utils.GetOperatorNamespace(),
			checkComponent: utils.ComponentControlPlane,
		},
		{
			name:           "spire bundle role binding without custom labels",
			resourceType:   "spireBundleRoleBinding",
			customLabels:   nil,
			expectedName:   "spire-bundle",
			expectedNS:     utils.GetOperatorNamespace(),
			checkComponent: utils.ComponentControlPlane,
		},
		{
			name:           "controller manager cluster role without custom labels",
			resourceType:   "controllerManagerClusterRole",
			customLabels:   nil,
			expectedName:   "spire-controller-manager",
			checkComponent: utils.ComponentControlPlane,
		},
		{
			name:           "controller manager cluster role binding without custom labels",
			resourceType:   "controllerManagerClusterRoleBinding",
			customLabels:   nil,
			expectedName:   "spire-controller-manager",
			checkComponent: utils.ComponentControlPlane,
		},
		{
			name:           "leader election role without custom labels",
			resourceType:   "leaderElectionRole",
			customLabels:   nil,
			expectedName:   "spire-controller-manager-leader-election",
			expectedNS:     utils.GetOperatorNamespace(),
			checkComponent: utils.ComponentControlPlane,
		},
		{
			name:           "leader election role binding without custom labels",
			resourceType:   "leaderElectionRoleBinding",
			customLabels:   nil,
			expectedName:   "spire-controller-manager-leader-election",
			expectedNS:     utils.GetOperatorNamespace(),
			checkComponent: utils.ComponentControlPlane,
		},
		{
			name:         "spire server cluster role with custom labels",
			resourceType: "spireServerClusterRole",
			customLabels: map[string]string{"team": "platform", "region": "us-west"},
			expectedName: "spire-server",
		},
		{
			name:         "spire bundle role with custom labels",
			resourceType: "spireBundleRole",
			customLabels: map[string]string{"bundle-type": "ca-certificates"},
			expectedName: "spire-bundle",
			expectedNS:   utils.GetOperatorNamespace(),
		},
		{
			name:         "controller manager cluster role with custom labels",
			resourceType: "controllerManagerClusterRole",
			customLabels: map[string]string{"controller": "spire-manager"},
			expectedName: "spire-controller-manager",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var name, namespace string
			var labels map[string]string

			switch tt.resourceType {
			case "spireServerClusterRole":
				cr := getSpireServerClusterRole(tt.customLabels)
				if cr == nil {
					t.Fatal("Expected ClusterRole, got nil")
				}
				name = cr.Name
				labels = cr.Labels
			case "spireServerClusterRoleBinding":
				crb := getSpireServerClusterRoleBinding(tt.customLabels)
				if crb == nil {
					t.Fatal("Expected ClusterRoleBinding, got nil")
				}
				name = crb.Name
				labels = crb.Labels
			case "spireBundleRole":
				role := getSpireBundleRole(tt.customLabels)
				if role == nil {
					t.Fatal("Expected Role, got nil")
				}
				name = role.Name
				namespace = role.Namespace
				labels = role.Labels
			case "spireBundleRoleBinding":
				rb := getSpireBundleRoleBinding(tt.customLabels)
				if rb == nil {
					t.Fatal("Expected RoleBinding, got nil")
				}
				name = rb.Name
				namespace = rb.Namespace
				labels = rb.Labels
			case "controllerManagerClusterRole":
				cr := getSpireControllerManagerClusterRole(tt.customLabels)
				if cr == nil {
					t.Fatal("Expected ClusterRole, got nil")
				}
				name = cr.Name
				labels = cr.Labels
			case "controllerManagerClusterRoleBinding":
				crb := getSpireControllerManagerClusterRoleBinding(tt.customLabels)
				if crb == nil {
					t.Fatal("Expected ClusterRoleBinding, got nil")
				}
				name = crb.Name
				labels = crb.Labels
			case "leaderElectionRole":
				role := getSpireControllerManagerLeaderElectionRole(tt.customLabels)
				if role == nil {
					t.Fatal("Expected Role, got nil")
				}
				name = role.Name
				namespace = role.Namespace
				labels = role.Labels
			case "leaderElectionRoleBinding":
				rb := getSpireControllerManagerLeaderElectionRoleBinding(tt.customLabels)
				if rb == nil {
					t.Fatal("Expected RoleBinding, got nil")
				}
				name = rb.Name
				namespace = rb.Namespace
				labels = rb.Labels
			}

			if name != tt.expectedName {
				t.Errorf("Expected name '%s', got '%s'", tt.expectedName, name)
			}

			if tt.expectedNS != "" && namespace != tt.expectedNS {
				t.Errorf("Expected namespace '%s', got '%s'", tt.expectedNS, namespace)
			}

			// Check managed-by label
			if val, ok := labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
				t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
			}

			// Check component label if specified
			if tt.checkComponent != "" {
				if val, ok := labels["app.kubernetes.io/component"]; !ok || val != tt.checkComponent {
					t.Errorf("Expected label app.kubernetes.io/component=%s", tt.checkComponent)
				}
			}

			// Check custom labels if specified
			for key, expectedValue := range tt.customLabels {
				if val, ok := labels[key]; !ok || val != expectedValue {
					t.Errorf("Expected custom label '%s=%s', got '%s'", key, expectedValue, val)
				}
			}
		})
	}
}

func TestLabelPreservation(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
	}{
		{"spire server cluster role", "spireServerClusterRole"},
		{"spire bundle role", "spireBundleRole"},
		{"controller manager cluster role", "controllerManagerClusterRole"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var labelsWithoutCustom, labelsWithCustom map[string]string

			customLabels := map[string]string{"test": "value"}

			switch tt.resourceType {
			case "spireServerClusterRole":
				labelsWithoutCustom = getSpireServerClusterRole(nil).Labels
				labelsWithCustom = getSpireServerClusterRole(customLabels).Labels
			case "spireBundleRole":
				labelsWithoutCustom = getSpireBundleRole(nil).Labels
				labelsWithCustom = getSpireBundleRole(customLabels).Labels
			case "controllerManagerClusterRole":
				labelsWithoutCustom = getSpireControllerManagerClusterRole(nil).Labels
				labelsWithCustom = getSpireControllerManagerClusterRole(customLabels).Labels
			}

			// Verify all asset labels are preserved
			for k, v := range labelsWithoutCustom {
				if labelsWithCustom[k] != v {
					t.Errorf("Asset label '%s=%s' was not preserved", k, v)
				}
			}

			// Verify custom label was added
			if val, ok := labelsWithCustom["test"]; !ok || val != "value" {
				t.Error("Custom label was not added")
			}
		})
	}
}

// newRBACTestReconciler creates a reconciler for RBAC tests
func newRBACTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireServerReconciler {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	return &SpireServerReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}
}

// createRBACTestServer creates a test server for RBAC tests
func createRBACTestServer() *v1alpha1.SpireServer {
	return &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
			UID:  "test-uid",
		},
	}
}

func TestReconcileClusterRole(t *testing.T) {
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
			name:   "create success",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-server"))
				fc.CreateReturns(nil)
			},
			expectError:  false,
			expectCreate: true,
		},
		{
			name:   "create error",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-server"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name:   "get error",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(errors.New("connection refused"))
			},
			expectError: true,
		},
		{
			name:   "create only mode skips update",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingCR := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "spire-server", ResourceVersion: "123"},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if cr, ok := obj.(*rbacv1.ClusterRole); ok {
						*cr = *existingCR
					}
					return nil
				}
			},
			createOnlyMode: true,
			expectError:    false,
			expectUpdate:   false,
		},
		{
			name: "update error",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireServerSpec{
					CommonConfig: v1alpha1.CommonConfig{Labels: map[string]string{"new-label": "new-value"}},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingCR := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-server",
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if cr, ok := obj.(*rbacv1.ClusterRole); ok {
						*cr = *existingCR
					}
					return nil
				}
				fc.UpdateReturns(errors.New("update conflict"))
			},
			expectError:  true,
			expectUpdate: true,
		},
		{
			name:           "set controller ref error",
			server:         createRBACTestServer(),
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
				reconciler = newRBACTestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileClusterRole(context.Background(), tt.server, statusMgr, tt.createOnlyMode)

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

func TestReconcileClusterRoleBinding(t *testing.T) {
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
			name:   "create success",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-server"))
				fc.CreateReturns(nil)
			},
			expectError:  false,
			expectCreate: true,
		},
		{
			name:   "create error",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-server"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name:   "get error",
			server: createRBACTestServer(),
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
					CommonConfig: v1alpha1.CommonConfig{Labels: map[string]string{"new": "label"}},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingCRB := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-server",
						ResourceVersion: "123",
						Labels:          map[string]string{"old": "label"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if crb, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
						*crb = *existingCRB
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
					CommonConfig: v1alpha1.CommonConfig{Labels: map[string]string{"new": "label"}},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingCRB := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-server",
						ResourceVersion: "123",
						Labels:          map[string]string{"old": "label"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if crb, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
						*crb = *existingCRB
					}
					return nil
				}
				fc.UpdateReturns(errors.New("update failed"))
			},
			expectError:  true,
			expectUpdate: true,
		},
		{
			name: "create only mode skips update",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireServerSpec{
					CommonConfig: v1alpha1.CommonConfig{Labels: map[string]string{"new": "label"}},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingCRB := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-server",
						ResourceVersion: "123",
						Labels:          map[string]string{"old": "label"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if crb, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
						*crb = *existingCRB
					}
					return nil
				}
			},
			createOnlyMode: true,
			expectError:    false,
			expectUpdate:   false,
		},
		{
			name:           "set controller ref error",
			server:         createRBACTestServer(),
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
				reconciler = newRBACTestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileClusterRoleBinding(context.Background(), tt.server, statusMgr, tt.createOnlyMode)

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

func TestReconcileSpireBundleRole(t *testing.T) {
	tests := []struct {
		name           string
		server         *v1alpha1.SpireServer
		setupClient    func(*fakes.FakeCustomCtrlClient)
		useEmptyScheme bool
		expectError    bool
		expectCreate   bool
		expectUpdate   bool
	}{
		{
			name:   "create success",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-bundle"))
				fc.CreateReturns(nil)
			},
			expectError:  false,
			expectCreate: true,
		},
		{
			name:   "create error",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-bundle"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name:   "get error",
			server: createRBACTestServer(),
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
					CommonConfig: v1alpha1.CommonConfig{Labels: map[string]string{"new": "label"}},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingRole := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-bundle",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old": "label"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if role, ok := obj.(*rbacv1.Role); ok {
						*role = *existingRole
					}
					return nil
				}
				fc.UpdateReturns(nil)
			},
			expectError:  false,
			expectUpdate: true,
		},
		{
			name:           "set controller ref error",
			server:         createRBACTestServer(),
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
				reconciler = newRBACTestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileSpireBundleRole(context.Background(), tt.server, statusMgr, false)

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
		})
	}
}

func TestReconcileSpireBundleRoleBinding(t *testing.T) {
	tests := []struct {
		name           string
		server         *v1alpha1.SpireServer
		setupClient    func(*fakes.FakeCustomCtrlClient)
		useEmptyScheme bool
		expectError    bool
		expectCreate   bool
		expectUpdate   bool
	}{
		{
			name:   "create success",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-bundle"))
				fc.CreateReturns(nil)
			},
			expectError:  false,
			expectCreate: true,
		},
		{
			name:   "create error",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-bundle"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name: "update success",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireServerSpec{
					CommonConfig: v1alpha1.CommonConfig{Labels: map[string]string{"new": "label"}},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingRB := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-bundle",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old": "label"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if rb, ok := obj.(*rbacv1.RoleBinding); ok {
						*rb = *existingRB
					}
					return nil
				}
				fc.UpdateReturns(nil)
			},
			expectError:  false,
			expectUpdate: true,
		},
		{
			name:           "set controller ref error",
			server:         createRBACTestServer(),
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
				reconciler = newRBACTestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileSpireBundleRoleBinding(context.Background(), tt.server, statusMgr, false)

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
		})
	}
}

func TestReconcileRBAC(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(*fakes.FakeCustomCtrlClient)
		expectError bool
	}{
		{
			name: "success",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, ""))
				fc.CreateReturns(nil)
			},
			expectError: false,
		},
		{
			name: "cluster role error",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(errors.New("cluster role error"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newRBACTestReconciler(fakeClient)
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileRBAC(context.Background(), createRBACTestServer(), statusMgr, false)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

func TestReconcileControllerManagerClusterRole(t *testing.T) {
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
			name:   "create success",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-controller-manager"))
				fc.CreateReturns(nil)
			},
			expectError:  false,
			expectCreate: true,
		},
		{
			name:   "create error",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-controller-manager"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name:   "get error",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(errors.New("connection refused"))
			},
			expectError: true,
		},
		{
			name:   "create only mode",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingCR := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{Name: "spire-controller-manager", ResourceVersion: "123"},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if cr, ok := obj.(*rbacv1.ClusterRole); ok {
						*cr = *existingCR
					}
					return nil
				}
			},
			createOnlyMode: true,
			expectError:    false,
			expectUpdate:   false,
		},
		{
			name:           "set controller ref error",
			server:         createRBACTestServer(),
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
				reconciler = newRBACTestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileControllerManagerClusterRole(context.Background(), tt.server, statusMgr, tt.createOnlyMode)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if tt.expectCreate && fakeClient.CreateCallCount() != 1 {
				t.Errorf("Expected Create to be called once, got %d", fakeClient.CreateCallCount())
			}
			if !tt.expectUpdate && fakeClient.UpdateCallCount() != 0 {
				t.Error("Expected Update not to be called")
			}
		})
	}
}

func TestReconcileControllerManagerClusterRoleBinding(t *testing.T) {
	tests := []struct {
		name           string
		server         *v1alpha1.SpireServer
		setupClient    func(*fakes.FakeCustomCtrlClient)
		useEmptyScheme bool
		expectError    bool
		expectCreate   bool
		expectUpdate   bool
	}{
		{
			name:   "create success",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-controller-manager"))
				fc.CreateReturns(nil)
			},
			expectError:  false,
			expectCreate: true,
		},
		{
			name:   "create error",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-controller-manager"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name:   "get error",
			server: createRBACTestServer(),
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
					CommonConfig: v1alpha1.CommonConfig{Labels: map[string]string{"new": "label"}},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingCRB := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-controller-manager",
						ResourceVersion: "123",
						Labels:          map[string]string{"old": "label"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if crb, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
						*crb = *existingCRB
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
					CommonConfig: v1alpha1.CommonConfig{Labels: map[string]string{"new": "label"}},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingCRB := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-controller-manager",
						ResourceVersion: "123",
						Labels:          map[string]string{"old": "label"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if crb, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
						*crb = *existingCRB
					}
					return nil
				}
				fc.UpdateReturns(errors.New("update failed"))
			},
			expectError: true,
		},
		{
			name:           "set controller ref error",
			server:         createRBACTestServer(),
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
				reconciler = newRBACTestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileControllerManagerClusterRoleBinding(context.Background(), tt.server, statusMgr, false)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if tt.expectCreate && fakeClient.CreateCallCount() != 1 {
				t.Errorf("Expected Create to be called once, got %d", fakeClient.CreateCallCount())
			}
			if tt.expectUpdate && fakeClient.UpdateCallCount() != 1 {
				t.Errorf("Expected Update to be called once, got %d", fakeClient.UpdateCallCount())
			}
		})
	}
}

func TestReconcileLeaderElectionRole(t *testing.T) {
	tests := []struct {
		name           string
		server         *v1alpha1.SpireServer
		setupClient    func(*fakes.FakeCustomCtrlClient)
		useEmptyScheme bool
		expectError    bool
		expectCreate   bool
		expectUpdate   bool
	}{
		{
			name:   "create success",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-controller-manager-leader-election"))
				fc.CreateReturns(nil)
			},
			expectError:  false,
			expectCreate: true,
		},
		{
			name:   "create error",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-controller-manager-leader-election"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name:   "get error",
			server: createRBACTestServer(),
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
					CommonConfig: v1alpha1.CommonConfig{Labels: map[string]string{"new": "label"}},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingRole := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-controller-manager-leader-election",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old": "label"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if role, ok := obj.(*rbacv1.Role); ok {
						*role = *existingRole
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
					CommonConfig: v1alpha1.CommonConfig{Labels: map[string]string{"new": "label"}},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingRole := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-controller-manager-leader-election",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old": "label"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if role, ok := obj.(*rbacv1.Role); ok {
						*role = *existingRole
					}
					return nil
				}
				fc.UpdateReturns(errors.New("update failed"))
			},
			expectError: true,
		},
		{
			name:           "set controller ref error",
			server:         createRBACTestServer(),
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
				reconciler = newRBACTestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileLeaderElectionRole(context.Background(), tt.server, statusMgr, false)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if tt.expectCreate && fakeClient.CreateCallCount() != 1 {
				t.Errorf("Expected Create to be called once, got %d", fakeClient.CreateCallCount())
			}
			if tt.expectUpdate && fakeClient.UpdateCallCount() != 1 {
				t.Errorf("Expected Update to be called once, got %d", fakeClient.UpdateCallCount())
			}
		})
	}
}

func TestReconcileLeaderElectionRoleBinding(t *testing.T) {
	tests := []struct {
		name           string
		server         *v1alpha1.SpireServer
		setupClient    func(*fakes.FakeCustomCtrlClient)
		useEmptyScheme bool
		expectError    bool
		expectCreate   bool
		expectUpdate   bool
	}{
		{
			name:   "create success",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-controller-manager-leader-election"))
				fc.CreateReturns(nil)
			},
			expectError:  false,
			expectCreate: true,
		},
		{
			name:   "create error",
			server: createRBACTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-controller-manager-leader-election"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name:   "get error",
			server: createRBACTestServer(),
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
					CommonConfig: v1alpha1.CommonConfig{Labels: map[string]string{"new": "label"}},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingRB := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-controller-manager-leader-election",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old": "label"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if rb, ok := obj.(*rbacv1.RoleBinding); ok {
						*rb = *existingRB
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
					CommonConfig: v1alpha1.CommonConfig{Labels: map[string]string{"new": "label"}},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingRB := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-controller-manager-leader-election",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old": "label"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if rb, ok := obj.(*rbacv1.RoleBinding); ok {
						*rb = *existingRB
					}
					return nil
				}
				fc.UpdateReturns(errors.New("update failed"))
			},
			expectError: true,
		},
		{
			name:           "set controller ref error",
			server:         createRBACTestServer(),
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
				reconciler = newRBACTestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileLeaderElectionRoleBinding(context.Background(), tt.server, statusMgr, false)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if tt.expectCreate && fakeClient.CreateCallCount() != 1 {
				t.Errorf("Expected Create to be called once, got %d", fakeClient.CreateCallCount())
			}
			if tt.expectUpdate && fakeClient.UpdateCallCount() != 1 {
				t.Errorf("Expected Update to be called once, got %d", fakeClient.UpdateCallCount())
			}
		})
	}
}
