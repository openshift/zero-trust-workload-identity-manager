package spire_oidc_discovery_provider

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	return obj.GetNamespace() + "/" + obj.GetName()
}

func (s *testStore) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	k := key.Namespace + "/" + key.Name
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

// TestGetExternalCertRole tests the getExternalCertRole function
func TestGetExternalCertRole(t *testing.T) {
	tests := []struct {
		name         string
		customLabels map[string]string
		checkFunc    func(t *testing.T, role *rbacv1.Role)
	}{
		{
			name:         "without custom labels",
			customLabels: nil,
			checkFunc: func(t *testing.T, role *rbacv1.Role) {
				assert.Equal(t, utils.SpireOIDCExternalCertRoleName, role.Name)
				assert.Equal(t, utils.GetOperatorNamespace(), role.Namespace)
				assert.NotEmpty(t, role.Labels)
				assert.Equal(t, utils.AppManagedByLabelValue, role.Labels[utils.AppManagedByLabelKey])
				assert.Equal(t, utils.ComponentDiscovery, role.Labels["app.kubernetes.io/component"])
				assert.NotEmpty(t, role.Rules)
			},
		},
		{
			name: "with custom labels",
			customLabels: map[string]string{
				"team":        "platform",
				"environment": "production",
			},
			checkFunc: func(t *testing.T, role *rbacv1.Role) {
				assert.Equal(t, "platform", role.Labels["team"])
				assert.Equal(t, "production", role.Labels["environment"])
				assert.Equal(t, utils.AppManagedByLabelValue, role.Labels[utils.AppManagedByLabelKey])
				assert.Equal(t, utils.ComponentDiscovery, role.Labels["app.kubernetes.io/component"])
			},
		},
		{
			name:         "verifies role rules structure",
			customLabels: nil,
			checkFunc: func(t *testing.T, role *rbacv1.Role) {
				require.NotEmpty(t, role.Rules)
				foundGetRule := false
				foundWatchRule := false
				for _, rule := range role.Rules {
					for _, resource := range rule.Resources {
						if resource == "secrets" {
							if contains(rule.Verbs, "get") {
								foundGetRule = true
							}
							if contains(rule.Verbs, "list") && contains(rule.Verbs, "watch") {
								foundWatchRule = true
							}
						}
					}
				}
				assert.True(t, foundGetRule, "Expected role to have a rule for 'get' on secrets")
				assert.True(t, foundWatchRule, "Expected role to have a rule for 'list' and 'watch' on secrets")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role := getExternalCertRole(tt.customLabels)
			require.NotNil(t, role)
			tt.checkFunc(t, role)
		})
	}
}

// TestGetExternalCertRoleBinding tests the getExternalCertRoleBinding function
func TestGetExternalCertRoleBinding(t *testing.T) {
	tests := []struct {
		name         string
		customLabels map[string]string
		checkFunc    func(t *testing.T, rb *rbacv1.RoleBinding)
	}{
		{
			name:         "without custom labels",
			customLabels: nil,
			checkFunc: func(t *testing.T, rb *rbacv1.RoleBinding) {
				assert.Equal(t, utils.SpireOIDCExternalCertRoleBindingName, rb.Name)
				assert.Equal(t, utils.GetOperatorNamespace(), rb.Namespace)
				assert.NotEmpty(t, rb.Labels)
				assert.Equal(t, utils.AppManagedByLabelValue, rb.Labels[utils.AppManagedByLabelKey])
				assert.Equal(t, utils.ComponentDiscovery, rb.Labels["app.kubernetes.io/component"])
				assert.NotEmpty(t, rb.Subjects)
				assert.NotEmpty(t, rb.RoleRef.Name)
			},
		},
		{
			name: "with custom labels",
			customLabels: map[string]string{
				"team":     "security",
				"priority": "high",
			},
			checkFunc: func(t *testing.T, rb *rbacv1.RoleBinding) {
				assert.Equal(t, "security", rb.Labels["team"])
				assert.Equal(t, "high", rb.Labels["priority"])
				assert.Equal(t, utils.AppManagedByLabelValue, rb.Labels[utils.AppManagedByLabelKey])
				assert.Equal(t, utils.ComponentDiscovery, rb.Labels["app.kubernetes.io/component"])
			},
		},
		{
			name:         "verifies rolebinding structure",
			customLabels: nil,
			checkFunc: func(t *testing.T, rb *rbacv1.RoleBinding) {
				require.NotEmpty(t, rb.Subjects)
				assert.Equal(t, "Role", rb.RoleRef.Kind)
				assert.Equal(t, "rbac.authorization.k8s.io", rb.RoleRef.APIGroup)
				assert.NotEmpty(t, rb.RoleRef.Name)
				assert.NotEmpty(t, rb.Subjects[0].Name)
				assert.NotEmpty(t, rb.Subjects[0].Namespace)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := getExternalCertRoleBinding(tt.customLabels)
			require.NotNil(t, rb)
			tt.checkFunc(t, rb)
		})
	}
}

// TestReconcileExternalCertRole tests the reconcileExternalCertRole function
func TestReconcileExternalCertRole(t *testing.T) {
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
			name: "creates role when it doesn't exist",
			setupObjects: func() []client.Object {
				return []client.Object{}
			},
			setupScheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = v1alpha1.AddToScheme(scheme)
				_ = rbacv1.AddToScheme(scheme)
				return scheme
			},
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				return newFakeClient(store)
			},
			createOnlyMode: false,
			expectError:    false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				role := &rbacv1.Role{}
				err := client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireOIDCExternalCertRoleName,
					Namespace: utils.GetOperatorNamespace(),
				}, role)
				require.NoError(t, err)
				assert.Equal(t, utils.SpireOIDCExternalCertRoleName, role.Name)
				assert.Contains(t, role.Rules[0].ResourceNames, "test-secret")
				require.NotEmpty(t, role.OwnerReferences)
			},
		},
		{
			name: "updates role when it exists and is different",
			setupObjects: func() []client.Object {
				return []client.Object{
					&rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireOIDCExternalCertRoleName,
							Namespace: utils.GetOperatorNamespace(),
							Labels:    map[string]string{"old-label": "old-value"},
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups:     []string{""},
								Resources:     []string{"secrets"},
								Verbs:         []string{"get"},
								ResourceNames: []string{"old-secret"},
							},
						},
					},
				}
			},
			setupScheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = v1alpha1.AddToScheme(scheme)
				_ = rbacv1.AddToScheme(scheme)
				return scheme
			},
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				return newFakeClient(store)
			},
			createOnlyMode: false,
			expectError:    false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				role := &rbacv1.Role{}
				err := client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireOIDCExternalCertRoleName,
					Namespace: utils.GetOperatorNamespace(),
				}, role)
				require.NoError(t, err)
				assert.Contains(t, role.Rules[0].ResourceNames, "test-secret")
			},
		},
		{
			name: "skips update in create-only mode",
			setupObjects: func() []client.Object {
				return []client.Object{
					&rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireOIDCExternalCertRoleName,
							Namespace: utils.GetOperatorNamespace(),
							Labels:    map[string]string{"old-label": "old-value"},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "ztwim.openshift.io/v1alpha1",
									Kind:       "SpireOIDCDiscoveryProvider",
									Name:       "test-oidc",
									UID:        "test-uid",
									Controller: func() *bool { b := true; return &b }(),
								},
							},
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups:     []string{""},
								Resources:     []string{"secrets"},
								Verbs:         []string{"get"},
								ResourceNames: []string{"old-secret"},
							},
						},
					},
				}
			},
			setupScheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = v1alpha1.AddToScheme(scheme)
				_ = rbacv1.AddToScheme(scheme)
				return scheme
			},
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				return newFakeClient(store)
			},
			createOnlyMode: true,
			expectError:    false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				role := &rbacv1.Role{}
				err := client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireOIDCExternalCertRoleName,
					Namespace: utils.GetOperatorNamespace(),
				}, role)
				require.NoError(t, err)
				assert.Contains(t, role.Rules[0].ResourceNames, "old-secret")
				assert.NotContains(t, role.Rules[0].ResourceNames, "test-secret")
			},
		},
		{
			name: "no update when role is already up to date",
			setupObjects: func() []client.Object {
				// Create a role that matches what getExternalCertRole returns
				desiredRole := getExternalCertRole(nil)
				desiredRole.Rules[0].ResourceNames = []string{"test-secret"}
				desiredRole.OwnerReferences = []metav1.OwnerReference{
					{
						APIVersion: "ztwim.openshift.io/v1alpha1",
						Kind:       "SpireOIDCDiscoveryProvider",
						Name:       "test-oidc",
						UID:        "test-uid",
						Controller: func() *bool { b := true; return &b }(),
					},
				}
				return []client.Object{desiredRole}
			},
			setupScheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = v1alpha1.AddToScheme(scheme)
				_ = rbacv1.AddToScheme(scheme)
				return scheme
			},
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				return newFakeClient(store)
			},
			createOnlyMode: false,
			expectError:    false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				role := &rbacv1.Role{}
				err := client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireOIDCExternalCertRoleName,
					Namespace: utils.GetOperatorNamespace(),
				}, role)
				require.NoError(t, err)
				assert.Contains(t, role.Rules[0].ResourceNames, "test-secret")
			},
		},
		{
			name: "fails when SetControllerReference fails",
			setupObjects: func() []client.Object {
				return []client.Object{}
			},
			setupScheme: func() *runtime.Scheme {
				return runtime.NewScheme() // Empty scheme causes SetControllerReference to fail
			},
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				return newFakeClient(store)
			},
			createOnlyMode: false,
			expectError:    true,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
		{
			name:         "fails when Get returns unexpected error",
			setupObjects: func() []client.Object { return []client.Object{} },
			setupScheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = v1alpha1.AddToScheme(scheme)
				_ = rbacv1.AddToScheme(scheme)
				return scheme
			},
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
			setupScheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = v1alpha1.AddToScheme(scheme)
				_ = rbacv1.AddToScheme(scheme)
				return scheme
			},
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				fake.GetReturns(kerrors.NewNotFound(rbacv1.Resource("roles"), "test-role"))
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
					&rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireOIDCExternalCertRoleName,
							Namespace: utils.GetOperatorNamespace(),
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups:     []string{""},
								Resources:     []string{"secrets"},
								Verbs:         []string{"get"},
								ResourceNames: []string{"old-secret"},
							},
						},
					},
				}
			},
			setupScheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = v1alpha1.AddToScheme(scheme)
				_ = rbacv1.AddToScheme(scheme)
				return scheme
			},
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				existingRole := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      utils.SpireOIDCExternalCertRoleName,
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
			createOnlyMode: false,
			expectError:    true,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			scheme := tt.setupScheme()
			store := newTestStore()

			// Create objects in store
			for _, obj := range tt.setupObjects() {
				_ = store.Create(ctx, obj)
			}

			client := tt.setupClient(store)

			r := &SpireOidcDiscoveryProviderReconciler{
				ctrlClient: client,
				scheme:     scheme,
				log:        logr.Discard(),
			}

			oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-oidc",
					UID:  "test-uid",
				},
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
					ExternalSecretRef: "test-secret",
				},
			}

			statusMgr := status.NewManager(client)

			// Execute
			err := r.reconcileExternalCertRole(ctx, oidc, statusMgr, tt.createOnlyMode)

			// Assert
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// Post-test checks
			tt.postTestChecks(t, client)
		})
	}
}

// TestReconcileExternalCertRoleBinding tests the reconcileExternalCertRoleBinding function
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
			name:         "creates rolebinding when it doesn't exist",
			setupObjects: func() []client.Object { return []client.Object{} },
			setupScheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = v1alpha1.AddToScheme(scheme)
				_ = rbacv1.AddToScheme(scheme)
				return scheme
			},
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				return newFakeClient(store)
			},
			createOnlyMode: false,
			expectError:    false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				rb := &rbacv1.RoleBinding{}
				err := client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireOIDCExternalCertRoleBindingName,
					Namespace: utils.GetOperatorNamespace(),
				}, rb)
				require.NoError(t, err)
				assert.Equal(t, utils.SpireOIDCExternalCertRoleBindingName, rb.Name)
				require.NotEmpty(t, rb.OwnerReferences)
			},
		},
		{
			name: "updates rolebinding when it exists and is different",
			setupObjects: func() []client.Object {
				return []client.Object{
					&rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireOIDCExternalCertRoleBindingName,
							Namespace: utils.GetOperatorNamespace(),
							Labels:    map[string]string{"old-label": "old-value"},
						},
						Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "old-sa", Namespace: "old-ns"}},
						RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "old-role"},
					},
				}
			},
			setupScheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = v1alpha1.AddToScheme(scheme)
				_ = rbacv1.AddToScheme(scheme)
				return scheme
			},
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				return newFakeClient(store)
			},
			createOnlyMode: false,
			expectError:    false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				rb := &rbacv1.RoleBinding{}
				err := client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireOIDCExternalCertRoleBindingName,
					Namespace: utils.GetOperatorNamespace(),
				}, rb)
				require.NoError(t, err)
			},
		},
		{
			name: "skips update in create-only mode",
			setupObjects: func() []client.Object {
				return []client.Object{
					&rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireOIDCExternalCertRoleBindingName,
							Namespace: utils.GetOperatorNamespace(),
							Labels:    map[string]string{"old-label": "old-value"},
							OwnerReferences: []metav1.OwnerReference{
								{
									APIVersion: "ztwim.openshift.io/v1alpha1",
									Kind:       "SpireOIDCDiscoveryProvider",
									Name:       "test-oidc",
									UID:        "test-uid",
									Controller: func() *bool { b := true; return &b }(),
								},
							},
						},
						Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "old-sa", Namespace: "old-ns"}},
						RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "old-role"},
					},
				}
			},
			setupScheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = v1alpha1.AddToScheme(scheme)
				_ = rbacv1.AddToScheme(scheme)
				return scheme
			},
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				return newFakeClient(store)
			},
			createOnlyMode: true,
			expectError:    false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				rb := &rbacv1.RoleBinding{}
				err := client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireOIDCExternalCertRoleBindingName,
					Namespace: utils.GetOperatorNamespace(),
				}, rb)
				require.NoError(t, err)
				// Should still have old label
				assert.Equal(t, "old-value", rb.Labels["old-label"])
			},
		},
		{
			name: "no update when rolebinding is already up to date",
			setupObjects: func() []client.Object {
				// Create a rolebinding that matches what getExternalCertRoleBinding returns
				desiredRB := getExternalCertRoleBinding(nil)
				desiredRB.OwnerReferences = []metav1.OwnerReference{
					{
						APIVersion: "ztwim.openshift.io/v1alpha1",
						Kind:       "SpireOIDCDiscoveryProvider",
						Name:       "test-oidc",
						UID:        "test-uid",
						Controller: func() *bool { b := true; return &b }(),
					},
				}
				return []client.Object{desiredRB}
			},
			setupScheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = v1alpha1.AddToScheme(scheme)
				_ = rbacv1.AddToScheme(scheme)
				return scheme
			},
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				return newFakeClient(store)
			},
			createOnlyMode: false,
			expectError:    false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				rb := &rbacv1.RoleBinding{}
				err := client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireOIDCExternalCertRoleBindingName,
					Namespace: utils.GetOperatorNamespace(),
				}, rb)
				require.NoError(t, err)
			},
		},
		{
			name:         "fails when SetControllerReference fails",
			setupObjects: func() []client.Object { return []client.Object{} },
			setupScheme: func() *runtime.Scheme {
				return runtime.NewScheme() // Empty scheme
			},
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				return newFakeClient(store)
			},
			createOnlyMode: false,
			expectError:    true,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
		{
			name:         "fails when Get returns unexpected error",
			setupObjects: func() []client.Object { return []client.Object{} },
			setupScheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = v1alpha1.AddToScheme(scheme)
				_ = rbacv1.AddToScheme(scheme)
				return scheme
			},
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
			setupScheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = v1alpha1.AddToScheme(scheme)
				_ = rbacv1.AddToScheme(scheme)
				return scheme
			},
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				fake.GetReturns(kerrors.NewNotFound(rbacv1.Resource("rolebindings"), "test-rb"))
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
							Name:      utils.SpireOIDCExternalCertRoleBindingName,
							Namespace: utils.GetOperatorNamespace(),
							Labels:    map[string]string{"old-label": "old-value"},
						},
						Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "old-sa", Namespace: "old-ns"}},
						RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "old-role"},
					},
				}
			},
			setupScheme: func() *runtime.Scheme {
				scheme := runtime.NewScheme()
				_ = v1alpha1.AddToScheme(scheme)
				_ = rbacv1.AddToScheme(scheme)
				return scheme
			},
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				existingRB := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      utils.SpireOIDCExternalCertRoleBindingName,
						Namespace: utils.GetOperatorNamespace(),
						Labels:    map[string]string{"old-label": "old-value"},
					},
					Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "old-sa", Namespace: "old-ns"}},
					RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "old-role"},
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
			// Setup
			scheme := tt.setupScheme()
			store := newTestStore()

			for _, obj := range tt.setupObjects() {
				_ = store.Create(ctx, obj)
			}

			client := tt.setupClient(store)

			r := &SpireOidcDiscoveryProviderReconciler{
				ctrlClient: client,
				scheme:     scheme,
				log:        logr.Discard(),
			}

			oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-oidc",
					UID:  "test-uid",
				},
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
					ExternalSecretRef: "test-secret",
				},
			}

			statusMgr := status.NewManager(client)

			// Execute
			err := r.reconcileExternalCertRoleBinding(ctx, oidc, statusMgr, tt.createOnlyMode)

			// Assert
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// Post-test checks
			tt.postTestChecks(t, client)
		})
	}
}

// TestReconcileExternalCertRBAC tests the reconcileExternalCertRBAC function
func TestReconcileExternalCertRBAC(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		setupObjects   func() []client.Object
		externalSecret string
		setupClient    func(store *testStore) customClient.CustomCtrlClient
		expectError    bool
		postTestChecks func(t *testing.T, client customClient.CustomCtrlClient)
	}{
		{
			name:           "creates RBAC resources when externalSecretRef is configured",
			setupObjects:   func() []client.Object { return []client.Object{} },
			externalSecret: "test-secret",
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				return newFakeClient(store)
			},
			expectError: false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				role := &rbacv1.Role{}
				err := client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireOIDCExternalCertRoleName,
					Namespace: utils.GetOperatorNamespace(),
				}, role)
				require.NoError(t, err)

				rb := &rbacv1.RoleBinding{}
				err = client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireOIDCExternalCertRoleBindingName,
					Namespace: utils.GetOperatorNamespace(),
				}, rb)
				require.NoError(t, err)
			},
		},
		{
			name:           "does not create RBAC when externalSecretRef is empty from start",
			setupObjects:   func() []client.Object { return []client.Object{} },
			externalSecret: "", // Empty from start
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				return newFakeClient(store)
			},
			expectError: false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				role := &rbacv1.Role{}
				err := client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireOIDCExternalCertRoleName,
					Namespace: utils.GetOperatorNamespace(),
				}, role)
				assert.True(t, kerrors.IsNotFound(err), "Expected role to not be created when externalSecretRef is empty")

				rb := &rbacv1.RoleBinding{}
				err = client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireOIDCExternalCertRoleBindingName,
					Namespace: utils.GetOperatorNamespace(),
				}, rb)
				assert.True(t, kerrors.IsNotFound(err), "Expected rolebinding to not be created when externalSecretRef is empty")
			},
		},
		{
			name: "does not delete RBAC when externalSecretRef is unset after being set",
			setupObjects: func() []client.Object {
				return []client.Object{
					&rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireOIDCExternalCertRoleName,
							Namespace: utils.GetOperatorNamespace(),
						},
					},
					&rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      utils.SpireOIDCExternalCertRoleBindingName,
							Namespace: utils.GetOperatorNamespace(),
						},
					},
				}
			},
			externalSecret: "", // Unset (was previously set)
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				return newFakeClient(store)
			},
			expectError: false,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {
				role := &rbacv1.Role{}
				err := client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireOIDCExternalCertRoleName,
					Namespace: utils.GetOperatorNamespace(),
				}, role)
				require.NoError(t, err, "Expected role to still exist")

				rb := &rbacv1.RoleBinding{}
				err = client.Get(ctx, types.NamespacedName{
					Name:      utils.SpireOIDCExternalCertRoleBindingName,
					Namespace: utils.GetOperatorNamespace(),
				}, rb)
				require.NoError(t, err, "Expected rolebinding to still exist")
			},
		},
		{
			name:           "fails when reconcileExternalCertRole returns error",
			setupObjects:   func() []client.Object { return []client.Object{} },
			externalSecret: "test-secret",
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				fake.GetReturns(testError)
				return fake
			},
			expectError:    true,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
		{
			name:           "fails when reconcileExternalCertRoleBinding returns error",
			setupObjects:   func() []client.Object { return []client.Object{} },
			externalSecret: "test-secret",
			setupClient: func(store *testStore) customClient.CustomCtrlClient {
				fake := &fakes.FakeCustomCtrlClient{}
				callCount := 0
				// First call (Role Get) returns NotFound, so Role gets created
				// Second call (RoleBinding Get) returns error to fail RoleBinding reconcile
				fake.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					if callCount == 1 {
						// Role Get - return NotFound so it creates
						return kerrors.NewNotFound(rbacv1.Resource("roles"), key.Name)
					}
					// RoleBinding Get - return error
					return testError
				}
				fake.CreateStub = func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					// Allow Role creation to succeed
					return nil
				}
				return fake
			},
			expectError:    true,
			postTestChecks: func(t *testing.T, client customClient.CustomCtrlClient) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)
			_ = rbacv1.AddToScheme(scheme)

			store := newTestStore()
			for _, obj := range tt.setupObjects() {
				_ = store.Create(ctx, obj)
			}

			client := tt.setupClient(store)

			r := &SpireOidcDiscoveryProviderReconciler{
				ctrlClient: client,
				scheme:     scheme,
				log:        logr.Discard(),
			}

			oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-oidc",
					UID:  "test-uid",
				},
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
					ExternalSecretRef: tt.externalSecret,
				},
			}

			statusMgr := status.NewManager(client)

			// Execute
			err := r.reconcileExternalCertRBAC(ctx, oidc, statusMgr, false)

			// Assert
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// Post-test checks
			tt.postTestChecks(t, client)
		})
	}
}

// contains checks if a string is in a slice of strings
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
