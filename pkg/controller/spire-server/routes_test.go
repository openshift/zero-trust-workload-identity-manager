package spire_server

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGenerateFederationRoute(t *testing.T) {
	tests := []struct {
		name                   string
		server                 *v1alpha1.SpireServer
		expectedHost           string
		expectedTLSTermination routev1.TLSTerminationType
		expectExternalCert     bool
	}{
		{
			name: "https_spiffe profile with passthrough TLS",
			server: &v1alpha1.SpireServer{
				Spec: v1alpha1.SpireServerSpec{
					Federation: &v1alpha1.FederationConfig{
						BundleEndpoint: v1alpha1.BundleEndpointConfig{
							Profile: v1alpha1.HttpsSpiffeProfile,
						},
					},
				},
			},
			expectedHost:           "federation.example.org",
			expectedTLSTermination: routev1.TLSTerminationPassthrough,
			expectExternalCert:     false,
		},
		{
			name: "https_web profile with ACME uses passthrough TLS",
			server: &v1alpha1.SpireServer{
				Spec: v1alpha1.SpireServerSpec{
					Federation: &v1alpha1.FederationConfig{
						BundleEndpoint: v1alpha1.BundleEndpointConfig{
							Profile: v1alpha1.HttpsWebProfile,
							HttpsWeb: &v1alpha1.HttpsWebConfig{
								Acme: &v1alpha1.AcmeConfig{
									DirectoryUrl: "https://acme.example.com/directory",
									DomainName:   "federation.example.org",
									Email:        "admin@example.org",
								},
							},
						},
					},
				},
			},
			expectedHost:           "federation.example.org",
			expectedTLSTermination: routev1.TLSTerminationPassthrough,
			expectExternalCert:     false,
		},
		{
			name: "https_web profile with ServingCert uses re-encrypt TLS",
			server: &v1alpha1.SpireServer{
				Spec: v1alpha1.SpireServerSpec{
					Federation: &v1alpha1.FederationConfig{
						BundleEndpoint: v1alpha1.BundleEndpointConfig{
							Profile: v1alpha1.HttpsWebProfile,
							HttpsWeb: &v1alpha1.HttpsWebConfig{
								ServingCert: &v1alpha1.ServingCertConfig{},
							},
						},
					},
				},
			},
			expectedHost:           "federation.example.org",
			expectedTLSTermination: routev1.TLSTerminationReencrypt,
			expectExternalCert:     false,
		},
		{
			name: "https_web profile with external certificate",
			server: &v1alpha1.SpireServer{
				Spec: v1alpha1.SpireServerSpec{
					Federation: &v1alpha1.FederationConfig{
						BundleEndpoint: v1alpha1.BundleEndpointConfig{
							Profile: v1alpha1.HttpsWebProfile,
							HttpsWeb: &v1alpha1.HttpsWebConfig{
								ServingCert: &v1alpha1.ServingCertConfig{
									ExternalSecretRef: "external-cert-secret",
								},
							},
						},
					},
				},
			},
			expectedHost:           "federation.example.org",
			expectedTLSTermination: routev1.TLSTerminationReencrypt,
			expectExternalCert:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain:     "example.org",
					ClusterName:     "test-cluster",
					BundleConfigMap: "spire-bundle",
				},
			}
			route := generateFederationRoute(tt.server, ztwim)

			if route.Name != "spire-server-federation" {
				t.Errorf("Expected route name 'spire-server-federation', got %q", route.Name)
			}

			if route.Namespace != utils.OperatorNamespace {
				t.Errorf("Expected namespace %q, got %q", utils.OperatorNamespace, route.Namespace)
			}

			if route.Spec.Host != tt.expectedHost {
				t.Errorf("Expected host %q, got %q", tt.expectedHost, route.Spec.Host)
			}

			if route.Spec.To.Kind != "Service" {
				t.Errorf("Expected To.Kind 'Service', got %q", route.Spec.To.Kind)
			}

			if route.Spec.To.Name != "spire-server" {
				t.Errorf("Expected To.Name 'spire-server', got %q", route.Spec.To.Name)
			}

			if route.Spec.Port == nil {
				t.Fatal("Expected Port to be set")
			}

			if route.Spec.Port.TargetPort != intstr.FromString("federation") {
				t.Errorf("Expected TargetPort 'federation', got %v", route.Spec.Port.TargetPort)
			}

			if route.Spec.TLS == nil {
				t.Fatal("Expected TLS configuration to be set")
			}

			if route.Spec.TLS.Termination != tt.expectedTLSTermination {
				t.Errorf("Expected TLS termination %q, got %q", tt.expectedTLSTermination, route.Spec.TLS.Termination)
			}

			if route.Spec.TLS.InsecureEdgeTerminationPolicy != routev1.InsecureEdgeTerminationPolicyRedirect {
				t.Errorf("Expected insecure edge termination policy 'Redirect', got %q", route.Spec.TLS.InsecureEdgeTerminationPolicy)
			}

			if tt.expectExternalCert {
				if route.Spec.TLS.ExternalCertificate == nil {
					t.Error("Expected ExternalCertificate to be set")
				} else if route.Spec.TLS.ExternalCertificate.Name != "external-cert-secret" {
					t.Errorf("Expected ExternalCertificate name 'external-cert-secret', got %q", route.Spec.TLS.ExternalCertificate.Name)
				}
			} else {
				if route.Spec.TLS.ExternalCertificate != nil {
					t.Error("Expected ExternalCertificate to be nil")
				}
			}

			if route.Spec.WildcardPolicy != routev1.WildcardPolicyNone {
				t.Errorf("Expected WildcardPolicy 'None', got %q", route.Spec.WildcardPolicy)
			}
		})
	}
}

func TestCheckFederationRouteConflict(t *testing.T) {
	baseRoute := &routev1.Route{
		Spec: routev1.RouteSpec{
			Host: "federation.example.org",
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: "spire-server",
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString("federation"),
			},
			TLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationPassthrough,
			},
		},
	}

	tests := []struct {
		name           string
		current        *routev1.Route
		desired        *routev1.Route
		expectConflict bool
	}{
		{
			name:           "identical routes - no conflict",
			current:        baseRoute.DeepCopy(),
			desired:        baseRoute.DeepCopy(),
			expectConflict: false,
		},
		{
			name:    "different host - conflict",
			current: baseRoute.DeepCopy(),
			desired: func() *routev1.Route {
				r := baseRoute.DeepCopy()
				r.Spec.Host = "different.example.org"
				return r
			}(),
			expectConflict: true,
		},
		{
			name:    "different TLS termination - conflict",
			current: baseRoute.DeepCopy(),
			desired: func() *routev1.Route {
				r := baseRoute.DeepCopy()
				r.Spec.TLS.Termination = routev1.TLSTerminationReencrypt
				return r
			}(),
			expectConflict: true,
		},
		{
			name:    "different service name - conflict",
			current: baseRoute.DeepCopy(),
			desired: func() *routev1.Route {
				r := baseRoute.DeepCopy()
				r.Spec.To.Name = "different-service"
				return r
			}(),
			expectConflict: true,
		},
		{
			name: "different labels - conflict",
			current: func() *routev1.Route {
				r := baseRoute.DeepCopy()
				r.Labels = map[string]string{"key": "value1"}
				return r
			}(),
			desired: func() *routev1.Route {
				r := baseRoute.DeepCopy()
				r.Labels = map[string]string{"key": "value2"}
				return r
			}(),
			expectConflict: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasConflict := checkFederationRouteConflict(tt.current, tt.desired)
			if hasConflict != tt.expectConflict {
				t.Errorf("Expected conflict=%v, got %v", tt.expectConflict, hasConflict)
			}
		})
	}
}

func TestGenerateFederationRouteWithDifferentTrustDomains(t *testing.T) {
	trustDomains := []string{
		"example.org",
		"test.com",
		"my-domain.io",
	}

	for _, td := range trustDomains {
		t.Run(td, func(t *testing.T) {
			server := &v1alpha1.SpireServer{
				Spec: v1alpha1.SpireServerSpec{
					Federation: &v1alpha1.FederationConfig{
						BundleEndpoint: v1alpha1.BundleEndpointConfig{
							Profile: v1alpha1.HttpsSpiffeProfile,
						},
					},
				},
			}

			ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain:     td,
					ClusterName:     "test-cluster",
					BundleConfigMap: "spire-bundle",
				},
			}

			route := generateFederationRoute(server, ztwim)
			expectedHost := "federation." + td

			if route.Spec.Host != expectedHost {
				t.Errorf("Expected host %q, got %q", expectedHost, route.Spec.Host)
			}
		})
	}
}

func TestGenerateFederationRouteLabels(t *testing.T) {
	customLabels := map[string]string{
		"custom-label-1": "value1",
		"custom-label-2": "value2",
	}

	server := &v1alpha1.SpireServer{
		Spec: v1alpha1.SpireServerSpec{
			Federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile: v1alpha1.HttpsSpiffeProfile,
				},
			},
			CommonConfig: v1alpha1.CommonConfig{
				Labels: customLabels,
			},
		},
	}

	ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
			TrustDomain:     "example.org",
			ClusterName:     "test-cluster",
			BundleConfigMap: "spire-bundle",
		},
	}

	route := generateFederationRoute(server, ztwim)

	// Check that custom labels are present
	for key, expectedValue := range customLabels {
		if actualValue, exists := route.Labels[key]; !exists {
			t.Errorf("Expected label %q to exist", key)
		} else if actualValue != expectedValue {
			t.Errorf("Expected label %q to have value %q, got %q", key, expectedValue, actualValue)
		}
	}

	// Check that standard labels are also present
	expectedStandardLabels := utils.SpireServerLabels(customLabels)
	for key, expectedValue := range expectedStandardLabels {
		if actualValue, exists := route.Labels[key]; !exists {
			t.Errorf("Expected standard label %q to exist", key)
		} else if actualValue != expectedValue {
			t.Errorf("Expected standard label %q to have value %q, got %q", key, expectedValue, actualValue)
		}
	}
}

// newRouteTestReconciler creates a reconciler for route tests
func newRouteTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireServerReconciler {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = routev1.AddToScheme(scheme)
	return &SpireServerReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}
}

// createRouteTestServer creates a test server for route tests
func createRouteTestServer() *v1alpha1.SpireServer {
	return &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
			UID:  "test-uid",
		},
		Spec: v1alpha1.SpireServerSpec{
			Federation: &v1alpha1.FederationConfig{
				ManagedRoute: "true",
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile: v1alpha1.HttpsSpiffeProfile,
				},
			},
		},
	}
}

// createRouteTestZTWIM creates a test ZTWIM for route tests
func createRouteTestZTWIM() *v1alpha1.ZeroTrustWorkloadIdentityManager {
	return &v1alpha1.ZeroTrustWorkloadIdentityManager{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
			TrustDomain:     "example.org",
			ClusterName:     "test-cluster",
			BundleConfigMap: "spire-bundle",
		},
	}
}

func TestReconcileRoute(t *testing.T) {
	tests := []struct {
		name           string
		server         *v1alpha1.SpireServer
		setupClient    func(*fakes.FakeCustomCtrlClient)
		createOnlyMode bool
		expectError    bool
		expectCreate   bool
		expectUpdate   bool
		expectNoOp     bool
	}{
		{
			name: "no federation configured - returns nil",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec:       v1alpha1.SpireServerSpec{Federation: nil},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {},
			expectError: false,
			expectNoOp:  true,
		},
		{
			name: "managed route disabled - does not create route",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireServerSpec{
					Federation: &v1alpha1.FederationConfig{
						ManagedRoute: "false",
						BundleEndpoint: v1alpha1.BundleEndpointConfig{
							Profile: v1alpha1.HttpsSpiffeProfile,
						},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {},
			expectError: false,
			expectNoOp:  true,
		},
		{
			name:   "create route success",
			server: createRouteTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-server-federation"))
				fc.CreateReturns(nil)
			},
			expectError:  false,
			expectCreate: true,
		},
		{
			name:   "create route error",
			server: createRouteTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-server-federation"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name:   "get route error",
			server: createRouteTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(errors.New("connection refused"))
			},
			expectError: true,
		},
		{
			name: "update route success",
			server: func() *v1alpha1.SpireServer {
				s := createRouteTestServer()
				s.Spec.Labels = map[string]string{"new": "label"}
				return s
			}(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingRoute := &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-server-federation",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old": "label"},
					},
					Spec: routev1.RouteSpec{Host: "old-host"},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if route, ok := obj.(*routev1.Route); ok {
						*route = *existingRoute
					}
					return nil
				}
				fc.UpdateReturns(nil)
			},
			expectError:  false,
			expectUpdate: true,
		},
		{
			name: "update route error",
			server: func() *v1alpha1.SpireServer {
				s := createRouteTestServer()
				s.Spec.Labels = map[string]string{"new": "label"}
				return s
			}(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingRoute := &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-server-federation",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old": "label"},
					},
					Spec: routev1.RouteSpec{Host: "old-host"},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if route, ok := obj.(*routev1.Route); ok {
						*route = *existingRoute
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
			server: func() *v1alpha1.SpireServer {
				s := createRouteTestServer()
				s.Spec.Labels = map[string]string{"new": "label"}
				return s
			}(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingRoute := &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-server-federation",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old": "label"},
					},
					Spec: routev1.RouteSpec{Host: "old-host"},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if route, ok := obj.(*routev1.Route); ok {
						*route = *existingRoute
					}
					return nil
				}
			},
			createOnlyMode: true,
			expectError:    false,
			expectUpdate:   false,
		},
		{
			name:   "route exists and up to date",
			server: createRouteTestServer(),
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				server := createRouteTestServer()
				ztwim := createRouteTestZTWIM()
				expectedRoute := generateFederationRoute(server, ztwim)
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if route, ok := obj.(*routev1.Route); ok {
						*route = *expectedRoute
					}
					return nil
				}
			},
			expectError:  false,
			expectUpdate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newRouteTestReconciler(fakeClient)
			tt.setupClient(fakeClient)

			ztwim := createRouteTestZTWIM()
			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileRoute(context.Background(), tt.server, statusMgr, ztwim, tt.createOnlyMode)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if tt.expectNoOp {
				if fakeClient.GetCallCount() != 0 {
					t.Error("Expected Get not to be called")
				}
				if fakeClient.CreateCallCount() != 0 {
					t.Error("Expected Create not to be called")
				}
			}
			if tt.expectCreate && fakeClient.CreateCallCount() != 1 {
				t.Errorf("Expected Create to be called once, got %d", fakeClient.CreateCallCount())
			}
			if tt.expectUpdate && fakeClient.UpdateCallCount() != 1 {
				t.Errorf("Expected Update to be called once, got %d", fakeClient.UpdateCallCount())
			}
			if !tt.expectUpdate && fakeClient.UpdateCallCount() != 0 {
				t.Error("Expected Update not to be called")
			}
		})
	}
}
