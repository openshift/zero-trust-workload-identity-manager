package spire_server

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	customClient "github.com/openshift/zero-trust-workload-identity-manager/pkg/client"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
)

var (
	testError = errors.New("test error")
)

// testStore is a simple in-memory store for testing
type testStore struct {
	objects map[string]client.Object
}

func newTestStore() *testStore {
	return &testStore{
		objects: make(map[string]client.Object),
	}
}

func (s *testStore) key(obj client.Object) string {
	ns := obj.GetNamespace()
	if ns == "" {
		// For cluster-scoped resources
		return obj.GetName()
	}
	return ns + "/" + obj.GetName()
}

func (s *testStore) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	ns := key.Namespace
	if ns == "" {
		ns = key.Name // For cluster-scoped resources
	}
	k := ns
	if key.Namespace != "" {
		k = key.Namespace + "/" + key.Name
	}

	stored, ok := s.objects[k]
	if !ok {
		return kerrors.NewNotFound(rbacv1.Resource(""), key.Name)
	}

	storedCopy := stored.DeepCopyObject()
	switch v := storedCopy.(type) {
	case *rbacv1.Role:
		if target, ok := obj.(*rbacv1.Role); ok {
			*target = *v
		}
	case *rbacv1.RoleBinding:
		if target, ok := obj.(*rbacv1.RoleBinding); ok {
			*target = *v
		}
	case *rbacv1.ClusterRole:
		if target, ok := obj.(*rbacv1.ClusterRole); ok {
			*target = *v
		}
	case *rbacv1.ClusterRoleBinding:
		if target, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
			*target = *v
		}
	}
	return nil
}

func (s *testStore) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	k := s.key(obj)
	if _, exists := s.objects[k]; exists {
		return kerrors.NewAlreadyExists(rbacv1.Resource(""), obj.GetName())
	}
	s.objects[k] = obj.DeepCopyObject().(client.Object)
	return nil
}

func (s *testStore) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	k := s.key(obj)
	s.objects[k] = obj.DeepCopyObject().(client.Object)
	return nil
}

func (s *testStore) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	k := s.key(obj)
	delete(s.objects, k)
	return nil
}

func (s *testStore) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return nil
}

// newFakeClient creates a new fake CustomCtrlClient for testing
func newFakeClient(store *testStore) customClient.CustomCtrlClient {
	fake := &fakes.FakeCustomCtrlClient{}

	fake.GetStub = store.Get
	fake.CreateStub = store.Create
	fake.UpdateStub = store.Update
	fake.DeleteStub = store.Delete
	fake.ListStub = store.List

	fake.StatusUpdateStub = func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
		return store.Update(ctx, obj)
	}

	fake.UpdateWithRetryStub = func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
		return store.Update(ctx, obj, opts...)
	}

	fake.ExistsStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) (bool, error) {
		err := store.Get(ctx, key, obj)
		if err != nil {
			if kerrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}

	fake.CreateOrUpdateObjectStub = func(ctx context.Context, obj client.Object) error {
		err := store.Create(ctx, obj)
		if err != nil && kerrors.IsAlreadyExists(err) {
			return store.Update(ctx, obj)
		}
		return err
	}

	fake.StatusUpdateWithRetryStub = func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
		return store.Update(ctx, obj)
	}

	return fake
}

func TestGetSpireServerClusterRole(t *testing.T) {
	cr := getSpireServerClusterRole(nil)

	if cr == nil {
		t.Fatal("Expected ClusterRole, got nil")
	}

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

// Tests for newly added external cert RBAC functions

func TestGetSpireServerExternalCertRole(t *testing.T) {
	tests := []struct {
		name         string
		customLabels map[string]string
		checkFunc    func(t *testing.T, role *rbacv1.Role)
	}{
		{
			name:         "without custom labels",
			customLabels: nil,
			checkFunc: func(t *testing.T, role *rbacv1.Role) {
				if role.Name != utils.SpireServerExternalCertRoleName {
					t.Errorf("Expected Role name '%s', got '%s'", utils.SpireServerExternalCertRoleName, role.Name)
				}
				if role.Namespace != utils.GetOperatorNamespace() {
					t.Errorf("Expected Role namespace '%s', got '%s'", utils.GetOperatorNamespace(), role.Namespace)
				}
				if val, ok := role.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
					t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
				}
				if val, ok := role.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentControlPlane {
					t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentControlPlane)
				}
				if len(role.Rules) == 0 {
					t.Error("Expected role to have rules")
				}
			},
		},
		{
			name: "with custom labels",
			customLabels: map[string]string{
				"team": "platform",
				"env":  "prod",
			},
			checkFunc: func(t *testing.T, role *rbacv1.Role) {
				if val, ok := role.Labels["team"]; !ok || val != "platform" {
					t.Errorf("Expected custom label 'team=platform'")
				}
				if val, ok := role.Labels["env"]; !ok || val != "prod" {
					t.Errorf("Expected custom label 'env=prod'")
				}
				if val, ok := role.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
					t.Errorf("Expected standard label to be preserved")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role := getSpireServerExternalCertRole(tt.customLabels)
			if role == nil {
				t.Fatal("Expected Role, got nil")
			}
			tt.checkFunc(t, role)
		})
	}
}

func TestGetSpireServerExternalCertRoleBinding(t *testing.T) {
	tests := []struct {
		name         string
		customLabels map[string]string
		checkFunc    func(t *testing.T, rb *rbacv1.RoleBinding)
	}{
		{
			name:         "without custom labels",
			customLabels: nil,
			checkFunc: func(t *testing.T, rb *rbacv1.RoleBinding) {
				if rb.Name != utils.SpireServerExternalCertRoleBindingName {
					t.Errorf("Expected RoleBinding name '%s', got '%s'", utils.SpireServerExternalCertRoleBindingName, rb.Name)
				}
				if rb.Namespace != utils.GetOperatorNamespace() {
					t.Errorf("Expected RoleBinding namespace '%s', got '%s'", utils.GetOperatorNamespace(), rb.Namespace)
				}
				if val, ok := rb.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
					t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
				}
				if len(rb.Subjects) == 0 {
					t.Error("Expected rolebinding to have subjects")
				}
				if rb.RoleRef.Name == "" {
					t.Error("Expected rolebinding to have roleRef")
				}
			},
		},
		{
			name: "with custom labels",
			customLabels: map[string]string{
				"team": "security",
			},
			checkFunc: func(t *testing.T, rb *rbacv1.RoleBinding) {
				if val, ok := rb.Labels["team"]; !ok || val != "security" {
					t.Errorf("Expected custom label 'team=security'")
				}
				if val, ok := rb.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
					t.Errorf("Expected standard label to be preserved")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := getSpireServerExternalCertRoleBinding(tt.customLabels)
			if rb == nil {
				t.Fatal("Expected RoleBinding, got nil")
			}
			tt.checkFunc(t, rb)
		})
	}
}

func TestGetExternalSecretRefFromServer(t *testing.T) {
	tests := []struct {
		name           string
		server         *v1alpha1.SpireServer
		expectedSecret string
	}{
		{
			name: "returns secret when federation with https_web and externalSecretRef is configured",
			server: &v1alpha1.SpireServer{
				Spec: v1alpha1.SpireServerSpec{
					Federation: &v1alpha1.FederationConfig{
						BundleEndpoint: v1alpha1.BundleEndpointConfig{
							HttpsWeb: &v1alpha1.HttpsWebConfig{
								ServingCert: &v1alpha1.ServingCertConfig{
									ExternalSecretRef: "test-secret",
								},
							},
						},
					},
				},
			},
			expectedSecret: "test-secret",
		},
		{
			name: "returns empty string when federation is nil",
			server: &v1alpha1.SpireServer{
				Spec: v1alpha1.SpireServerSpec{
					Federation: nil,
				},
			},
			expectedSecret: "",
		},
		{
			name: "returns empty string when HttpsWeb is nil",
			server: &v1alpha1.SpireServer{
				Spec: v1alpha1.SpireServerSpec{
					Federation: &v1alpha1.FederationConfig{
						BundleEndpoint: v1alpha1.BundleEndpointConfig{
							HttpsWeb: nil,
						},
					},
				},
			},
			expectedSecret: "",
		},
		{
			name: "returns empty string when ServingCert is nil",
			server: &v1alpha1.SpireServer{
				Spec: v1alpha1.SpireServerSpec{
					Federation: &v1alpha1.FederationConfig{
						BundleEndpoint: v1alpha1.BundleEndpointConfig{
							HttpsWeb: &v1alpha1.HttpsWebConfig{
								ServingCert: nil,
							},
						},
					},
				},
			},
			expectedSecret: "",
		},
		{
			name: "returns empty string when ExternalSecretRef is empty",
			server: &v1alpha1.SpireServer{
				Spec: v1alpha1.SpireServerSpec{
					Federation: &v1alpha1.FederationConfig{
						BundleEndpoint: v1alpha1.BundleEndpointConfig{
							HttpsWeb: &v1alpha1.HttpsWebConfig{
								ServingCert: &v1alpha1.ServingCertConfig{
									ExternalSecretRef: "",
								},
							},
						},
					},
				},
			},
			expectedSecret: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getExternalSecretRefFromServer(tt.server)
			if result != tt.expectedSecret {
				t.Errorf("Expected '%s', got '%s'", tt.expectedSecret, result)
			}
		})
	}
}

// Reconcile function tests for external cert RBAC

func TestReconcileExternalCertRole(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name              string
		setupObjects      func() []client.Object
		setupScheme       func() *runtime.Scheme
		setupClient       func(store *testStore) customClient.CustomCtrlClient
		externalSecretRef string
		createOnlyMode    bool
		expectError       bool
		postTestChecks    func(t *testing.T, client customClient.CustomCtrlClient)
	}{
		{
			name:              "creates role when it doesn't exist",
			setupObjects:      func() []client.Object { return []client.Object{} },
			setupScheme:       func() *runtime.Scheme { return newTestScheme() },
			setupClient:       func(store *testStore) customClient.CustomCtrlClient { return newFakeClient(store) },
			externalSecretRef: "test-secret",
			createOnlyMode:    false,
			expectError:       false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				role := &rbacv1.Role{}
				err := client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireServerExternalCertRoleName,
					Namespace: utils.GetOperatorNamespace(),
				}, role)
				if err != nil {
					t.Errorf("Expected role to be created, got error: %v", err)
				}
				if len(role.Rules) > 0 && !contains(role.Rules[0].ResourceNames, "test-secret") {
					t.Error("Expected role to have test-secret in resourceNames")
				}
			},
		},
		{
			name: "updates role when it exists and is different",
			setupObjects: func() []client.Object {
				return []client.Object{
					&rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireServerExternalCertRoleName,
							Namespace: utils.GetOperatorNamespace(),
						},
						Rules: []rbacv1.PolicyRule{{
							APIGroups:     []string{""},
							Resources:     []string{"secrets"},
							Verbs:         []string{"get"},
							ResourceNames: []string{"old-secret"},
						}},
					},
				}
			},
			setupScheme:       func() *runtime.Scheme { return newTestScheme() },
			setupClient:       func(store *testStore) customClient.CustomCtrlClient { return newFakeClient(store) },
			externalSecretRef: "new-secret",
			createOnlyMode:    false,
			expectError:       false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				role := &rbacv1.Role{}
				_ = client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireServerExternalCertRoleName,
					Namespace: utils.GetOperatorNamespace(),
				}, role)
				if len(role.Rules) > 0 && !contains(role.Rules[0].ResourceNames, "new-secret") {
					t.Error("Expected role to have new-secret in resourceNames")
				}
			},
		},
		{
			name: "skips update in create-only mode",
			setupObjects: func() []client.Object {
				return []client.Object{
					&rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireServerExternalCertRoleName,
							Namespace: utils.GetOperatorNamespace(),
							OwnerReferences: []metav1.OwnerReference{{
								APIVersion: "ztwim.openshift.io/v1alpha1",
								Kind:       "SpireServer",
								Name:       "test-server",
								UID:        "test-uid",
								Controller: func() *bool { b := true; return &b }(),
							}},
						},
						Rules: []rbacv1.PolicyRule{{
							APIGroups:     []string{""},
							Resources:     []string{"secrets"},
							Verbs:         []string{"get"},
							ResourceNames: []string{"old-secret"},
						}},
					},
				}
			},
			setupScheme:       func() *runtime.Scheme { return newTestScheme() },
			setupClient:       func(store *testStore) customClient.CustomCtrlClient { return newFakeClient(store) },
			externalSecretRef: "new-secret",
			createOnlyMode:    true,
			expectError:       false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				role := &rbacv1.Role{}
				_ = client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireServerExternalCertRoleName,
					Namespace: utils.GetOperatorNamespace(),
				}, role)
				// Should still have old-secret
				if len(role.Rules) > 0 && !contains(role.Rules[0].ResourceNames, "old-secret") {
					t.Error("Expected role to still have old-secret in createOnlyMode")
				}
			},
		},
		{
			name: "no update when role is already up to date",
			setupObjects: func() []client.Object {
				desiredRole := getSpireServerExternalCertRole(nil)
				desiredRole.Rules[0].ResourceNames = []string{"test-secret"}
				desiredRole.OwnerReferences = []metav1.OwnerReference{{
					APIVersion: "ztwim.openshift.io/v1alpha1",
					Kind:       "SpireServer",
					Name:       "test-server",
					UID:        "test-uid",
					Controller: func() *bool { b := true; return &b }(),
				}}
				return []client.Object{desiredRole}
			},
			setupScheme:       func() *runtime.Scheme { return newTestScheme() },
			setupClient:       func(store *testStore) customClient.CustomCtrlClient { return newFakeClient(store) },
			externalSecretRef: "test-secret",
			createOnlyMode:    false,
			expectError:       false,
			postTestChecks:    func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
		{
			name:              "fails when SetControllerReference fails",
			setupObjects:      func() []client.Object { return []client.Object{} },
			setupScheme:       func() *runtime.Scheme { return runtime.NewScheme() },
			setupClient:       func(store *testStore) customClient.CustomCtrlClient { return newFakeClient(store) },
			externalSecretRef: "test-secret",
			createOnlyMode:    false,
			expectError:       true,
			postTestChecks:    func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
		{
			name:         "fails when Get returns unexpected error",
			setupObjects: func() []client.Object { return []client.Object{} },
			setupScheme:  func() *runtime.Scheme { return newTestScheme() },
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				fake.GetReturns(testError)
				return fake
			},
			externalSecretRef: "test-secret",
			createOnlyMode:    false,
			expectError:       true,
			postTestChecks:    func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
		{
			name:         "fails when Create returns error",
			setupObjects: func() []client.Object { return []client.Object{} },
			setupScheme:  func() *runtime.Scheme { return newTestScheme() },
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				fake.GetReturns(kerrors.NewNotFound(rbacv1.Resource("roles"), "test"))
				fake.CreateReturns(testError)
				return fake
			},
			externalSecretRef: "test-secret",
			createOnlyMode:    false,
			expectError:       true,
			postTestChecks:    func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
		{
			name: "fails when Update returns error",
			setupObjects: func() []client.Object {
				return []client.Object{
					&rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireServerExternalCertRoleName,
							Namespace: utils.GetOperatorNamespace(),
						},
						Rules: []rbacv1.PolicyRule{{
							APIGroups:     []string{""},
							Resources:     []string{"secrets"},
							Verbs:         []string{"get"},
							ResourceNames: []string{"old-secret"},
						}},
					},
				}
			},
			setupScheme: func() *runtime.Scheme { return newTestScheme() },
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				existingRole := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      utils.SpireServerExternalCertRoleName,
						Namespace: utils.GetOperatorNamespace(),
					},
					Rules: []rbacv1.PolicyRule{{
						APIGroups:     []string{""},
						Resources:     []string{"secrets"},
						Verbs:         []string{"get"},
						ResourceNames: []string{"old-secret"},
					}},
				}
				fake.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if role, ok := obj.(*rbacv1.Role); ok {
						*role = *existingRole
					}
					return nil
				}
				fake.UpdateReturns(testError)
				return fake
			},
			externalSecretRef: "new-secret",
			createOnlyMode:    false,
			expectError:       true,
			postTestChecks:    func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := tt.setupScheme()
			store := newTestStore()
			for _, obj := range tt.setupObjects() {
				_ = store.Create(ctx, obj)
			}
			client := tt.setupClient(store)

			r := &SpireServerReconciler{
				ctrlClient: client,
				scheme:     scheme,
				log:        logr.Discard(),
			}

			server := &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "test-server", UID: "test-uid"},
			}

			statusMgr := status.NewManager(client)
			err := r.reconcileExternalCertRole(ctx, server, statusMgr, tt.createOnlyMode, tt.externalSecretRef)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			tt.postTestChecks(t, client)
		})
	}
}

func TestReconcileExternalCertRoleBinding(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		setupObjects   func() []client.Object
		setupScheme    func() *runtime.Scheme
		setupClient    func(store *testStore) customClient.CustomCtrlClient
		createOnlyMode bool
		expectError    bool
		postTestChecks func(t *testing.T, client customClient.CustomCtrlClient)
	}{
		{
			name:           "creates rolebinding when it doesn't exist",
			setupObjects:   func() []client.Object { return []client.Object{} },
			setupScheme:    func() *runtime.Scheme { return newTestScheme() },
			setupClient:    func(store *testStore) customClient.CustomCtrlClient { return newFakeClient(store) },
			createOnlyMode: false,
			expectError:    false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				rb := &rbacv1.RoleBinding{}
				err := client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireServerExternalCertRoleBindingName,
					Namespace: utils.GetOperatorNamespace(),
				}, rb)
				if err != nil {
					t.Errorf("Expected rolebinding to be created, got error: %v", err)
				}
			},
		},
		{
			name: "updates rolebinding when it exists and is different",
			setupObjects: func() []client.Object {
				return []client.Object{
					&rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireServerExternalCertRoleBindingName,
							Namespace: utils.GetOperatorNamespace(),
							Labels:    map[string]string{"old": "label"},
						},
						Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "old-sa"}},
						RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "old"},
					},
				}
			},
			setupScheme:    func() *runtime.Scheme { return newTestScheme() },
			setupClient:    func(store *testStore) customClient.CustomCtrlClient { return newFakeClient(store) },
			createOnlyMode: false,
			expectError:    false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
		{
			name: "skips update in create-only mode",
			setupObjects: func() []client.Object {
				return []client.Object{
					&rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireServerExternalCertRoleBindingName,
							Namespace: utils.GetOperatorNamespace(),
							Labels:    map[string]string{"old": "label"},
							OwnerReferences: []metav1.OwnerReference{{
								APIVersion: "ztwim.openshift.io/v1alpha1",
								Kind:       "SpireServer",
								Name:       "test-server",
								UID:        "test-uid",
								Controller: func() *bool { b := true; return &b }(),
							}},
						},
						Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "old-sa"}},
						RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "old"},
					},
				}
			},
			setupScheme:    func() *runtime.Scheme { return newTestScheme() },
			setupClient:    func(store *testStore) customClient.CustomCtrlClient { return newFakeClient(store) },
			createOnlyMode: true,
			expectError:    false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				rb := &rbacv1.RoleBinding{}
				_ = client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireServerExternalCertRoleBindingName,
					Namespace: utils.GetOperatorNamespace(),
				}, rb)
				if val, ok := rb.Labels["old"]; !ok || val != "label" {
					t.Error("Expected old label to remain in createOnlyMode")
				}
			},
		},
		{
			name: "no update when rolebinding is already up to date",
			setupObjects: func() []client.Object {
				desiredRB := getSpireServerExternalCertRoleBinding(nil)
				desiredRB.OwnerReferences = []metav1.OwnerReference{{
					APIVersion: "ztwim.openshift.io/v1alpha1",
					Kind:       "SpireServer",
					Name:       "test-server",
					UID:        "test-uid",
					Controller: func() *bool { b := true; return &b }(),
				}}
				return []client.Object{desiredRB}
			},
			setupScheme:    func() *runtime.Scheme { return newTestScheme() },
			setupClient:    func(store *testStore) customClient.CustomCtrlClient { return newFakeClient(store) },
			createOnlyMode: false,
			expectError:    false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
		{
			name:           "fails when SetControllerReference fails",
			setupObjects:   func() []client.Object { return []client.Object{} },
			setupScheme:    func() *runtime.Scheme { return runtime.NewScheme() },
			setupClient:    func(store *testStore) customClient.CustomCtrlClient { return newFakeClient(store) },
			createOnlyMode: false,
			expectError:    true,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
		{
			name:         "fails when Get returns unexpected error",
			setupObjects: func() []client.Object { return []client.Object{} },
			setupScheme:  func() *runtime.Scheme { return newTestScheme() },
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				fake.GetReturns(testError)
				return fake
			},
			createOnlyMode: false,
			expectError:    true,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
		{
			name:         "fails when Create returns error",
			setupObjects: func() []client.Object { return []client.Object{} },
			setupScheme:  func() *runtime.Scheme { return newTestScheme() },
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				fake.GetReturns(kerrors.NewNotFound(rbacv1.Resource("rolebindings"), "test"))
				fake.CreateReturns(testError)
				return fake
			},
			createOnlyMode: false,
			expectError:    true,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
		{
			name: "fails when Update returns error",
			setupObjects: func() []client.Object {
				return []client.Object{
					&rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireServerExternalCertRoleBindingName,
							Namespace: utils.GetOperatorNamespace(),
						},
						Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "old-sa"}},
						RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "old"},
					},
				}
			},
			setupScheme: func() *runtime.Scheme { return newTestScheme() },
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				existingRB := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      utils.SpireServerExternalCertRoleBindingName,
						Namespace: utils.GetOperatorNamespace(),
					},
					Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "old-sa"}},
					RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "old"},
				}
				fake.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if rb, ok := obj.(*rbacv1.RoleBinding); ok {
						*rb = *existingRB
					}
					return nil
				}
				fake.UpdateReturns(testError)
				return fake
			},
			createOnlyMode: false,
			expectError:    true,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := tt.setupScheme()
			store := newTestStore()
			for _, obj := range tt.setupObjects() {
				_ = store.Create(ctx, obj)
			}
			client := tt.setupClient(store)

			r := &SpireServerReconciler{
				ctrlClient: client,
				scheme:     scheme,
				log:        logr.Discard(),
			}

			server := &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "test-server", UID: "test-uid"},
			}

			statusMgr := status.NewManager(client)
			err := r.reconcileExternalCertRoleBinding(ctx, server, statusMgr, tt.createOnlyMode)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			tt.postTestChecks(t, client)
		})
	}
}

func TestReconcileExternalCertRBAC(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		server         *v1alpha1.SpireServer
		setupObjects   func() []client.Object
		setupClient    func(store *testStore) customClient.CustomCtrlClient
		expectError    bool
		postTestChecks func(t *testing.T, client customClient.CustomCtrlClient)
	}{
		{
			name: "creates RBAC resources when externalSecretRef is configured",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "test-server", UID: "test-uid"},
				Spec: v1alpha1.SpireServerSpec{
					Federation: &v1alpha1.FederationConfig{
						BundleEndpoint: v1alpha1.BundleEndpointConfig{
							HttpsWeb: &v1alpha1.HttpsWebConfig{
								ServingCert: &v1alpha1.ServingCertConfig{
									ExternalSecretRef: "test-secret",
								},
							},
						},
					},
				},
			},
			setupObjects: func() []client.Object { return []client.Object{} },
			setupClient:  func(store *testStore) customClient.CustomCtrlClient { return newFakeClient(store) },
			expectError:  false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				role := &rbacv1.Role{}
				err := client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireServerExternalCertRoleName,
					Namespace: utils.GetOperatorNamespace(),
				}, role)
				if err != nil {
					t.Errorf("Expected role to be created: %v", err)
				}

				rb := &rbacv1.RoleBinding{}
				err = client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireServerExternalCertRoleBindingName,
					Namespace: utils.GetOperatorNamespace(),
				}, rb)
				if err != nil {
					t.Errorf("Expected rolebinding to be created: %v", err)
				}
			},
		},
		{
			name: "cleans up RBAC when externalSecretRef is empty",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "test-server", UID: "test-uid"},
				Spec:       v1alpha1.SpireServerSpec{Federation: nil},
			},
			setupObjects: func() []client.Object {
				return []client.Object{
					&rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireServerExternalCertRoleName,
							Namespace: utils.GetOperatorNamespace(),
						},
					},
					&rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireServerExternalCertRoleBindingName,
							Namespace: utils.GetOperatorNamespace(),
						},
					},
				}
			},
			setupClient: func(store *testStore) customClient.CustomCtrlClient { return newFakeClient(store) },
			expectError: false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				role := &rbacv1.Role{}
				err := client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireServerExternalCertRoleName,
					Namespace: utils.GetOperatorNamespace(),
				}, role)
				if !kerrors.IsNotFound(err) {
					t.Error("Expected role to be deleted")
				}

				rb := &rbacv1.RoleBinding{}
				err = client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireServerExternalCertRoleBindingName,
					Namespace: utils.GetOperatorNamespace(),
				}, rb)
				if !kerrors.IsNotFound(err) {
					t.Error("Expected rolebinding to be deleted")
				}
			},
		},
		{
			name: "fails when reconcileExternalCertRole returns error",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "test-server", UID: "test-uid"},
				Spec: v1alpha1.SpireServerSpec{
					Federation: &v1alpha1.FederationConfig{
						BundleEndpoint: v1alpha1.BundleEndpointConfig{
							HttpsWeb: &v1alpha1.HttpsWebConfig{
								ServingCert: &v1alpha1.ServingCertConfig{
									ExternalSecretRef: "test-secret",
								},
							},
						},
					},
				},
			},
			setupObjects: func() []client.Object { return []client.Object{} },
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				fake.GetReturns(testError)
				return fake
			},
			expectError:    true,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
		{
			name: "fails when reconcileExternalCertRoleBinding returns error",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "test-server", UID: "test-uid"},
				Spec: v1alpha1.SpireServerSpec{
					Federation: &v1alpha1.FederationConfig{
						BundleEndpoint: v1alpha1.BundleEndpointConfig{
							HttpsWeb: &v1alpha1.HttpsWebConfig{
								ServingCert: &v1alpha1.ServingCertConfig{
									ExternalSecretRef: "test-secret",
								},
							},
						},
					},
				},
			},
			setupObjects: func() []client.Object { return []client.Object{} },
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				callCount := 0
				fake.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					if callCount == 1 {
						// Role Get returns NotFound
						return kerrors.NewNotFound(rbacv1.Resource("roles"), key.Name)
					}
					// RoleBinding Get returns error
					return testError
				}
				fake.CreateStub = func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					return nil // Allow Role creation
				}
				return fake
			},
			expectError:    true,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newTestScheme()
			store := newTestStore()
			for _, obj := range tt.setupObjects() {
				_ = store.Create(ctx, obj)
			}
			client := tt.setupClient(store)

			r := &SpireServerReconciler{
				ctrlClient: client,
				scheme:     scheme,
				log:        logr.Discard(),
			}

			statusMgr := status.NewManager(client)
			err := r.reconcileExternalCertRBAC(ctx, tt.server, statusMgr, false)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			tt.postTestChecks(t, client)
		})
	}
}

func TestCleanupExternalCertRBAC(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		setupObjects   func() []client.Object
		setupClient    func(store *testStore) customClient.CustomCtrlClient
		expectError    bool
		postTestChecks func(t *testing.T, client customClient.CustomCtrlClient)
	}{
		{
			name: "deletes role and rolebinding when they exist",
			setupObjects: func() []client.Object {
				return []client.Object{
					&rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireServerExternalCertRoleName,
							Namespace: utils.GetOperatorNamespace(),
						},
					},
					&rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireServerExternalCertRoleBindingName,
							Namespace: utils.GetOperatorNamespace(),
						},
					},
				}
			},
			setupClient: func(store *testStore) customClient.CustomCtrlClient { return newFakeClient(store) },
			expectError: false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				role := &rbacv1.Role{}
				err := client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireServerExternalCertRoleName,
					Namespace: utils.GetOperatorNamespace(),
				}, role)
				if !kerrors.IsNotFound(err) {
					t.Error("Expected role to be deleted")
				}

				rb := &rbacv1.RoleBinding{}
				err = client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireServerExternalCertRoleBindingName,
					Namespace: utils.GetOperatorNamespace(),
				}, rb)
				if !kerrors.IsNotFound(err) {
					t.Error("Expected rolebinding to be deleted")
				}
			},
		},
		{
			name:           "succeeds when resources don't exist",
			setupObjects:   func() []client.Object { return []client.Object{} },
			setupClient:    func(store *testStore) customClient.CustomCtrlClient { return newFakeClient(store) },
			expectError:    false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
		{
			name:         "fails when Get RoleBinding returns unexpected error",
			setupObjects: func() []client.Object { return []client.Object{} },
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				fake.GetReturns(testError)
				return fake
			},
			expectError:    true,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
		{
			name: "fails when Delete RoleBinding returns error",
			setupObjects: func() []client.Object {
				return []client.Object{
					&rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireServerExternalCertRoleBindingName,
							Namespace: utils.GetOperatorNamespace(),
						},
					},
				}
			},
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				existingRB := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      utils.SpireServerExternalCertRoleBindingName,
						Namespace: utils.GetOperatorNamespace(),
					},
				}
				fake.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if rb, ok := obj.(*rbacv1.RoleBinding); ok {
						*rb = *existingRB
					}
					return nil
				}
				fake.DeleteReturns(testError)
				return fake
			},
			expectError:    true,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
		{
			name:         "fails when Get Role returns unexpected error",
			setupObjects: func() []client.Object { return []client.Object{} },
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				callCount := 0
				fake.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					if callCount == 1 {
						return kerrors.NewNotFound(rbacv1.Resource("rolebindings"), key.Name)
					}
					return testError
				}
				return fake
			},
			expectError:    true,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
		{
			name:         "fails when Delete Role returns error",
			setupObjects: func() []client.Object { return []client.Object{} },
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				existingRole := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      utils.SpireServerExternalCertRoleName,
						Namespace: utils.GetOperatorNamespace(),
					},
				}
				callCount := 0
				fake.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					if callCount == 1 {
						return kerrors.NewNotFound(rbacv1.Resource("rolebindings"), key.Name)
					}
					if role, ok := obj.(*rbacv1.Role); ok {
						*role = *existingRole
					}
					return nil
				}
				fake.DeleteReturns(testError)
				return fake
			},
			expectError:    true,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newTestScheme()
			store := newTestStore()
			for _, obj := range tt.setupObjects() {
				_ = store.Create(ctx, obj)
			}
			client := tt.setupClient(store)

			r := &SpireServerReconciler{
				ctrlClient: client,
				scheme:     scheme,
				log:        logr.Discard(),
			}

			err := r.cleanupExternalCertRBAC(ctx)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			tt.postTestChecks(t, client)
		})
	}
}

// Helper functions
func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	return scheme
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
