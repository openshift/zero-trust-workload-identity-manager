package spire_oidc_discovery_provider

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReconcileClusterSpiffeIDs(t *testing.T) {
	tests := []struct {
		name           string
		notFound       bool
		getError       error
		createError    error
		updateError    error
		createOnlyMode bool
		useEmptyScheme bool
		expectError    bool
		expectCreate   int
	}{
		{
			name:         "create success",
			notFound:     true,
			expectError:  false,
			expectCreate: 2,
		},
		{
			name:        "create error for oidc",
			notFound:    true,
			createError: errors.New("create failed"),
			expectError: true,
		},
		{
			name:        "get error for oidc",
			getError:    errors.New("connection refused"),
			expectError: true,
		},
		{
			name:        "update success",
			expectError: false,
		},
		{
			name:        "update error for oidc",
			updateError: errors.New("update conflict"),
			expectError: true,
		},
		{
			name:           "create only mode skips update",
			createOnlyMode: true,
			expectError:    false,
		},
		{
			name:           "set controller reference error",
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
				reconciler = newClusterSpiffeIDTestReconciler(fakeClient)
			}

			oidc := createClusterSpiffeIDTestOIDCCR()
			oidc.Spec.Labels = map[string]string{"new": "label"}
			statusMgr := status.NewManager(fakeClient)

			if tt.notFound {
				fakeClient.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "test"))
			} else if tt.getError != nil {
				fakeClient.GetReturns(tt.getError)
			} else {
				existingCSID := &spiffev1alpha1.ClusterSPIFFEID{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-oidc-discovery-provider",
						ResourceVersion: "123",
						Labels:          map[string]string{"old": "label"},
					},
				}
				fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if csid, ok := obj.(*spiffev1alpha1.ClusterSPIFFEID); ok {
						*csid = *existingCSID
					}
					return nil
				}
			}
			fakeClient.CreateReturns(tt.createError)
			fakeClient.UpdateReturns(tt.updateError)

			err := reconciler.reconcileClusterSpiffeIDs(context.Background(), oidc, statusMgr, tt.createOnlyMode)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if tt.expectCreate > 0 && fakeClient.CreateCallCount() != tt.expectCreate {
				t.Errorf("Expected Create to be called %d times, got %d", tt.expectCreate, fakeClient.CreateCallCount())
			}
			if tt.createOnlyMode && fakeClient.UpdateCallCount() != 0 {
				t.Error("Expected Update not to be called in create-only mode")
			}
		})
	}
}

func TestGenerateClusterSPIFFEIDs(t *testing.T) {
	tests := []struct {
		name         string
		genFunc      func(map[string]string) *spiffev1alpha1.ClusterSPIFFEID
		expectedName string
		customLabels map[string]string
		checkCustom  bool
	}{
		{
			name:         "OIDC ClusterSPIFFEID basic",
			genFunc:      generateSpireIODCDiscoveryProviderSpiffeID,
			expectedName: "zero-trust-workload-identity-manager-spire-oidc-discovery-provider",
		},
		{
			name:         "OIDC ClusterSPIFFEID with labels",
			genFunc:      generateSpireIODCDiscoveryProviderSpiffeID,
			expectedName: "zero-trust-workload-identity-manager-spire-oidc-discovery-provider",
			customLabels: map[string]string{"custom": "label"},
			checkCustom:  true,
		},
		{
			name:         "Default ClusterSPIFFEID basic",
			genFunc:      generateDefaultFallbackClusterSPIFFEID,
			expectedName: "zero-trust-workload-identity-manager-spire-default",
		},
		{
			name:         "Default ClusterSPIFFEID with labels",
			genFunc:      generateDefaultFallbackClusterSPIFFEID,
			expectedName: "zero-trust-workload-identity-manager-spire-default",
			customLabels: map[string]string{"custom": "label"},
			checkCustom:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csid := tt.genFunc(tt.customLabels)
			if csid == nil {
				t.Fatal("Expected non-nil ClusterSPIFFEID")
			}
			if csid.Name != tt.expectedName {
				t.Errorf("Expected name %q, got %q", tt.expectedName, csid.Name)
			}
			if tt.checkCustom && csid.Labels["custom"] != "label" {
				t.Error("Expected custom label to be present")
			}
		})
	}
}

func newClusterSpiffeIDTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireOidcDiscoveryProviderReconciler {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = spiffev1alpha1.AddToScheme(scheme)
	return &SpireOidcDiscoveryProviderReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}
}

func createClusterSpiffeIDTestOIDCCR() *v1alpha1.SpireOIDCDiscoveryProvider {
	return &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
			UID:  "test-uid",
		},
		Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
			JwtIssuer: "https://oidc.example.org",
		},
	}
}
