package spire_agent

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGenerateSpireAgentSCC(t *testing.T) {
	customLabels := map[string]string{
		"custom-label": "custom-value",
	}
	config := &v1alpha1.SpireAgent{
		Spec: v1alpha1.SpireAgentSpec{
			CommonConfig: v1alpha1.CommonConfig{
				Labels: customLabels,
			},
		},
	}

	scc := generateSpireAgentSCC(config)
	expectedLabels := utils.SpireAgentLabels(customLabels)

	if scc.Name != "spire-agent" {
		t.Errorf("expected SCC name to be 'spire-agent', got %s", scc.Name)
	}

	if !reflect.DeepEqual(scc.Labels, expectedLabels) {
		t.Errorf("expected labels %v, got %v", expectedLabels, scc.Labels)
	}

	if !scc.ReadOnlyRootFilesystem {
		t.Errorf("expected ReadOnlyRootFilesystem to be true")
	}

	if scc.RunAsUser.Type != securityv1.RunAsUserStrategyMustRunAsRange {
		t.Errorf("expected RunAsUser.Type to be MustRunAsRange")
	}

	if scc.SELinuxContext.Type != securityv1.SELinuxStrategyMustRunAs {
		t.Errorf("expected SELinuxContext.Type to be MustRunAs")
	}

	if scc.SupplementalGroups.Type != securityv1.SupplementalGroupsStrategyMustRunAs {
		t.Errorf("expected SupplementalGroups.Type to be MustRunAs")
	}

	if scc.FSGroup.Type != securityv1.FSGroupStrategyMustRunAs {
		t.Errorf("expected FSGroup.Type to be MustRunAs")
	}

	expectedUser := fmt.Sprintf("system:serviceaccount:%s:spire-agent", utils.GetOperatorNamespace())
	if len(scc.Users) != 1 || scc.Users[0] != expectedUser {
		t.Errorf("expected Users to contain %s, got %v", expectedUser, scc.Users)
	}

	expectedVolumes := []securityv1.FSType{
		securityv1.FSTypeConfigMap,
		securityv1.FSTypeHostPath,
		securityv1.FSProjected,
		securityv1.FSTypeSecret,
		securityv1.FSTypeEmptyDir,
	}
	if !reflect.DeepEqual(scc.Volumes, expectedVolumes) {
		t.Errorf("expected Volumes %v, got %v", expectedVolumes, scc.Volumes)
	}

	if !scc.AllowHostDirVolumePlugin {
		t.Errorf("expected AllowHostDirVolumePlugin to be true")
	}
	if scc.AllowHostIPC {
		t.Errorf("expected AllowHostIPC to be false")
	}
	if !scc.AllowHostNetwork {
		t.Errorf("expected AllowHostNetwork to be true")
	}
	if !scc.AllowHostPID {
		t.Errorf("expected AllowHostPID to be true")
	}
	if !scc.AllowHostPorts {
		t.Errorf("expected AllowHostPorts to be true")
	}
	if scc.AllowPrivilegeEscalation == nil || !*scc.AllowPrivilegeEscalation {
		t.Errorf("expected AllowPrivilegeEscalation to be true")
	}
	if !scc.AllowPrivilegedContainer {
		t.Errorf("expected AllowPrivilegedContainer to be true")
	}

	if len(scc.AllowedCapabilities) != 0 {
		t.Errorf("expected AllowedCapabilities to be empty")
	}
	if len(scc.DefaultAddCapabilities) != 0 {
		t.Errorf("expected DefaultAddCapabilities to be empty")
	}
	if len(scc.RequiredDropCapabilities) != 1 || scc.RequiredDropCapabilities[0] != "ALL" {
		t.Errorf("expected RequiredDropCapabilities to contain 'ALL', got %v", scc.RequiredDropCapabilities)
	}
	if len(scc.Groups) != 0 {
		t.Errorf("expected Groups to be empty")
	}
}

// newSCCTestReconciler creates a reconciler for SCC tests
func newSCCTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireAgentReconciler {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = securityv1.AddToScheme(scheme)
	return &SpireAgentReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}
}

// TestReconcileSCC tests the reconcileSCC function
func TestReconcileSCC(t *testing.T) {
	tests := []struct {
		name           string
		agent          *v1alpha1.SpireAgent
		setupClient    func(*fakes.FakeCustomCtrlClient, *v1alpha1.SpireAgent)
		useEmptyScheme bool
		expectError    bool
		expectCreate   bool
		expectUpdate   bool
	}{
		{
			name: "create success",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient, _ *v1alpha1.SpireAgent) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-agent"))
				fc.CreateReturns(nil)
			},
			expectCreate: true,
		},
		{
			name: "create error",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient, _ *v1alpha1.SpireAgent) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-agent"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name: "get error",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient, _ *v1alpha1.SpireAgent) {
				fc.GetReturns(errors.New("connection refused"))
			},
			expectError: true,
		},
		{
			name: "no update needed",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient, agent *v1alpha1.SpireAgent) {
				desiredSCC := generateSpireAgentSCC(agent)
				desiredSCC.ResourceVersion = "123"
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if scc, ok := obj.(*securityv1.SecurityContextConstraints); ok {
						*scc = *desiredSCC
					}
					return nil
				}
			},
		},
		{
			name: "update success",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireAgentSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient, _ *v1alpha1.SpireAgent) {
				existingSCC := &securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-agent",
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if scc, ok := obj.(*securityv1.SecurityContextConstraints); ok {
						*scc = *existingSCC
					}
					return nil
				}
				fc.UpdateReturns(nil)
			},
			expectUpdate: true,
		},
		{
			name: "update error",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireAgentSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient, _ *v1alpha1.SpireAgent) {
				existingSCC := &securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-agent",
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if scc, ok := obj.(*securityv1.SecurityContextConstraints); ok {
						*scc = *existingSCC
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
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient:    func(fc *fakes.FakeCustomCtrlClient, _ *v1alpha1.SpireAgent) {},
			useEmptyScheme: true,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			var reconciler *SpireAgentReconciler
			if tt.useEmptyScheme {
				reconciler = &SpireAgentReconciler{
					ctrlClient:    fakeClient,
					ctx:           context.Background(),
					log:           logr.Discard(),
					scheme:        runtime.NewScheme(),
					eventRecorder: record.NewFakeRecorder(100),
				}
			} else {
				reconciler = newSCCTestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient, tt.agent)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileSCC(context.Background(), tt.agent, statusMgr)

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

// TestReconcileSCC_PreservesExistingFields tests that reconcileSCC preserves OpenShift-managed fields
func TestReconcileSCC_PreservesExistingFields(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newSCCTestReconciler(fakeClient)

	agent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
			UID:  "test-uid",
		},
		Spec: v1alpha1.SpireAgentSpec{
			CommonConfig: v1alpha1.CommonConfig{
				Labels: map[string]string{"new-label": "new-value"},
			},
		},
	}

	existingSCC := &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "spire-agent",
			ResourceVersion: "123",
			Labels:          map[string]string{"old-label": "old-value"},
		},
		Priority:           func() *int32 { p := int32(10); return &p }(),
		UserNamespaceLevel: "pod",
	}

	var capturedSCC *securityv1.SecurityContextConstraints
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		if scc, ok := obj.(*securityv1.SecurityContextConstraints); ok {
			*scc = *existingSCC
		}
		return nil
	}
	fakeClient.UpdateStub = func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
		if scc, ok := obj.(*securityv1.SecurityContextConstraints); ok {
			capturedSCC = scc
		}
		return nil
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.reconcileSCC(context.Background(), agent, statusMgr)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Check that preserved fields are intact
	if capturedSCC != nil {
		if capturedSCC.Priority == nil || *capturedSCC.Priority != 10 {
			t.Error("Expected Priority to be preserved")
		}
		if capturedSCC.UserNamespaceLevel != "pod" {
			t.Errorf("Expected UserNamespaceLevel to be preserved, got: %s", capturedSCC.UserNamespaceLevel)
		}
	}
}
