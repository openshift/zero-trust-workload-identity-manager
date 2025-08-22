package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
)

// MockCustomClient is a mock implementation of CustomCtrlClient
type MockCustomClient struct {
	mock.Mock
}

func (m *MockCustomClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	args := m.Called(ctx, key, obj)

	// Only copy if we have a valid return object and no error
	if args.Error(0) == nil && args.Get(1) != nil {
		switch v := obj.(type) {
		case *v1alpha1.SpireServer:
			if server, ok := args.Get(1).(*v1alpha1.SpireServer); ok {
				*v = *server
			}
		case *v1alpha1.SpireAgent:
			if agent, ok := args.Get(1).(*v1alpha1.SpireAgent); ok {
				*v = *agent
			}
		case *v1alpha1.SpireOIDCDiscoveryProvider:
			if oidc, ok := args.Get(1).(*v1alpha1.SpireOIDCDiscoveryProvider); ok {
				*v = *oidc
			}
		}
	}

	return args.Error(0)
}

// Implement remaining CustomCtrlClient interface methods as stubs
func (m *MockCustomClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return nil
}

func (m *MockCustomClient) StatusUpdate(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return nil
}

func (m *MockCustomClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return nil
}

func (m *MockCustomClient) UpdateWithRetry(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return nil
}

func (m *MockCustomClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return nil
}

func (m *MockCustomClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return nil
}

func (m *MockCustomClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
}

func (m *MockCustomClient) Exists(ctx context.Context, key client.ObjectKey, obj client.Object) (bool, error) {
	return false, nil
}

func (m *MockCustomClient) CreateOrUpdateObject(ctx context.Context, obj client.Object) error {
	return nil
}

func (m *MockCustomClient) StatusUpdateWithRetry(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return nil
}

// Helper functions to create test objects
func createSpireServer(trustDomain, clusterName, jwtIssuer string) *v1alpha1.SpireServer {
	return &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: v1alpha1.SpireServerSpec{
			TrustDomain: trustDomain,
			ClusterName: clusterName,
			JwtIssuer:   jwtIssuer,
		},
	}
}

func createSpireAgent(trustDomain, clusterName string) *v1alpha1.SpireAgent {
	return &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: v1alpha1.SpireAgentSpec{
			TrustDomain: trustDomain,
			ClusterName: clusterName,
		},
	}
}

func createSpireOIDCDiscoveryProvider(trustDomain, jwtIssuer string) *v1alpha1.SpireOIDCDiscoveryProvider {
	return &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
			TrustDomain: trustDomain,
			JwtIssuer:   jwtIssuer,
		},
	}
}

// Test normalizeIssuer function
func TestNormalizeIssuer(t *testing.T) {
	tests := []struct {
		name        string
		issuer      string
		trustDomain string
		expected    string
	}{
		{
			name:        "empty issuer returns default",
			issuer:      "",
			trustDomain: "example.com",
			expected:    "oidc-discovery.example.com",
		},
		{
			name:        "https prefix is stripped",
			issuer:      "https://my-issuer.com",
			trustDomain: "example.com",
			expected:    "my-issuer.com",
		},
		{
			name:        "http prefix is stripped",
			issuer:      "http://my-issuer.com",
			trustDomain: "example.com",
			expected:    "my-issuer.com",
		},
		{
			name:        "no prefix returns as is",
			issuer:      "my-issuer.com",
			trustDomain: "example.com",
			expected:    "my-issuer.com",
		},
		{
			name:        "both prefixes handled correctly",
			issuer:      "https://http://my-issuer.com",
			trustDomain: "example.com",
			expected:    "my-issuer.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeIssuer(tt.issuer, tt.trustDomain)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test SpireServerValidator
func TestSpireServerValidator_ValidateCreate(t *testing.T) {
	tests := []struct {
		name        string
		server      *v1alpha1.SpireServer
		agentExists bool
		agent       *v1alpha1.SpireAgent
		agentError  error
		oidcExists  bool
		oidc        *v1alpha1.SpireOIDCDiscoveryProvider
		oidcError   error
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "successful creation - no existing resources",
			server:      createSpireServer("example.com", "test-cluster", "https://issuer.com"),
			agentExists: false,
			agentError:  kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			oidcExists:  false,
			oidcError:   kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:     false,
		},
		{
			name:        "successful creation - matching fields with agent",
			server:      createSpireServer("example.com", "test-cluster", "https://issuer.com"),
			agentExists: true,
			agent:       createSpireAgent("example.com", "test-cluster"),
			oidcExists:  false,
			oidcError:   kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:     false,
		},
		{
			name:        "failed creation - trustDomain mismatch with agent",
			server:      createSpireServer("different.com", "test-cluster", "https://issuer.com"),
			agentExists: true,
			agent:       createSpireAgent("example.com", "test-cluster"),
			oidcExists:  false,
			oidcError:   kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:     true,
			errMsg:      "validation failed: SpireServer trustDomain",
		},
		{
			name:        "failed creation - jwtIssuer mismatch with oidc",
			server:      createSpireServer("example.com", "test-cluster", "https://different-issuer.com"),
			agentExists: false,
			agentError:  kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			oidcExists:  true,
			oidc:        createSpireOIDCDiscoveryProvider("example.com", "https://issuer.com"),
			wantErr:     true,
			errMsg:      "validation failed: SpireServer jwtIssuer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockCustomClient{}
			validator := &SpireServerValidator{Client: mockClient}

			// Mock agent Get call
			if tt.agentExists {
				mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireAgent")).
					Return(nil, tt.agent).Once()
			} else {
				mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireAgent")).
					Return(tt.agentError, nil).Once()
			}

			// Mock oidc Get call
			if tt.oidcExists {
				mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireOIDCDiscoveryProvider")).
					Return(nil, tt.oidc).Once()
			} else {
				mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireOIDCDiscoveryProvider")).
					Return(tt.oidcError, nil).Once()
			}

			warnings, err := validator.ValidateCreate(context.Background(), tt.server)

			assert.Nil(t, warnings)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestSpireServerValidator_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name        string
		oldServer   *v1alpha1.SpireServer
		newServer   *v1alpha1.SpireServer
		agentExists bool
		agent       *v1alpha1.SpireAgent
		agentError  error
		oidcExists  bool
		oidc        *v1alpha1.SpireOIDCDiscoveryProvider
		oidcError   error
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "successful update with no changes",
			oldServer:   createSpireServer("example.com", "test-cluster", "https://issuer.com"),
			newServer:   createSpireServer("example.com", "test-cluster", "https://issuer.com"),
			agentExists: false,
			agentError:  kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			oidcExists:  false,
			oidcError:   kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:     false,
		},
		{
			name:        "successful update with allowed field changes",
			oldServer:   createSpireServer("example.com", "test-cluster", "https://issuer.com"),
			newServer:   createSpireServer("example.com", "test-cluster", "https://new-issuer.com"),
			agentExists: false,
			agentError:  kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			oidcExists:  false,
			oidcError:   kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:     false,
		},
		{
			name:        "failed update - trustDomain changed",
			oldServer:   createSpireServer("example.com", "test-cluster", "https://issuer.com"),
			newServer:   createSpireServer("new-domain.com", "test-cluster", "https://issuer.com"),
			agentExists: false,
			agentError:  kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			oidcExists:  false,
			oidcError:   kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:     true,
			errMsg:      "validation failed: trustDomain field is immutable and cannot be changed",
		},
		{
			name:        "failed update - clusterName changed",
			oldServer:   createSpireServer("example.com", "test-cluster", "https://issuer.com"),
			newServer:   createSpireServer("example.com", "new-cluster", "https://issuer.com"),
			agentExists: false,
			agentError:  kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			oidcExists:  false,
			oidcError:   kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:     true,
			errMsg:      "validation failed: clusterName field is immutable and cannot be changed",
		},
		{
			name:        "failed update - trustDomain mismatch with existing agent",
			oldServer:   createSpireServer("example.com", "test-cluster", "https://issuer.com"),
			newServer:   createSpireServer("example.com", "test-cluster", "https://new-issuer.com"),
			agentExists: true,
			agent:       createSpireAgent("different.com", "test-cluster"),
			oidcExists:  false,
			oidcError:   kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:     true,
			errMsg:      "validation failed: SpireServer update trustDomain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockCustomClient{}
			validator := &SpireServerValidator{Client: mockClient}

			// Only set up mocks if the validation won't fail on immutability
			isImmutabilityError := (tt.oldServer.Spec.TrustDomain != tt.newServer.Spec.TrustDomain) ||
				(tt.oldServer.Spec.ClusterName != tt.newServer.Spec.ClusterName)

			if !isImmutabilityError {
				// Mock agent Get call
				if tt.agentExists {
					mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireAgent")).
						Return(nil, tt.agent).Once()
				} else {
					mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireAgent")).
						Return(tt.agentError, nil).Once()
				}

				// Mock oidc Get call
				if tt.oidcExists {
					mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireOIDCDiscoveryProvider")).
						Return(nil, tt.oidc).Once()
				} else {
					mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireOIDCDiscoveryProvider")).
						Return(tt.oidcError, nil).Once()
				}
			}

			warnings, err := validator.ValidateUpdate(context.Background(), tt.oldServer, tt.newServer)

			assert.Nil(t, warnings)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestSpireServerValidator_ValidateDelete(t *testing.T) {
	validator := &SpireServerValidator{}
	server := createSpireServer("example.com", "test-cluster", "https://issuer.com")

	warnings, err := validator.ValidateDelete(context.Background(), server)

	assert.Nil(t, warnings)
	assert.NoError(t, err, "ValidateDelete should not perform any validation for SpireServer")
}

// Test SpireAgentValidator
func TestSpireAgentValidator_ValidateCreate(t *testing.T) {
	tests := []struct {
		name         string
		agent        *v1alpha1.SpireAgent
		serverExists bool
		server       *v1alpha1.SpireServer
		serverError  error
		oidcExists   bool
		oidc         *v1alpha1.SpireOIDCDiscoveryProvider
		oidcError    error
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "successful creation with matching fields",
			agent:        createSpireAgent("example.com", "test-cluster"),
			serverExists: true,
			server:       createSpireServer("example.com", "test-cluster", "https://issuer.com"),
			oidcExists:   false,
			oidcError:    kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:      false,
		},
		{
			name:         "successful creation - server does not exist",
			agent:        createSpireAgent("example.com", "test-cluster"),
			serverExists: false,
			serverError:  kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			oidcExists:   false,
			oidcError:    kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:      false,
		},
		{
			name:         "failed creation - trustDomain mismatch",
			agent:        createSpireAgent("different.com", "test-cluster"),
			serverExists: true,
			server:       createSpireServer("example.com", "test-cluster", "https://issuer.com"),
			oidcExists:   false,
			oidcError:    kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:      true,
			errMsg:       "validation failed: SpireAgent trustDomain",
		},
		{
			name:         "failed creation - clusterName mismatch",
			agent:        createSpireAgent("example.com", "different-cluster"),
			serverExists: true,
			server:       createSpireServer("example.com", "test-cluster", "https://issuer.com"),
			oidcExists:   false,
			oidcError:    kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:      true,
			errMsg:       "validation failed: SpireAgent clusterName",
		},
		{
			name:         "failed creation - trustDomain mismatch with OIDC",
			agent:        createSpireAgent("different.com", "test-cluster"),
			serverExists: false,
			serverError:  kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			oidcExists:   true,
			oidc:         createSpireOIDCDiscoveryProvider("example.com", "https://issuer.com"),
			wantErr:      true,
			errMsg:       "validation failed: SpireAgent trustDomain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockCustomClient{}
			validator := &SpireAgentValidator{Client: mockClient}

			if tt.serverExists {
				mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireServer")).
					Return(nil, tt.server).Once()
			} else {
				mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireServer")).
					Return(tt.serverError, nil).Once()
			}

			if tt.oidcExists {
				mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireOIDCDiscoveryProvider")).
					Return(nil, tt.oidc).Once()
			} else {
				mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireOIDCDiscoveryProvider")).
					Return(tt.oidcError, nil).Once()
			}

			warnings, err := validator.ValidateCreate(context.Background(), tt.agent)

			assert.Nil(t, warnings)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestSpireAgentValidator_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name         string
		oldAgent     *v1alpha1.SpireAgent
		newAgent     *v1alpha1.SpireAgent
		serverExists bool
		server       *v1alpha1.SpireServer
		serverError  error
		oidcExists   bool
		oidc         *v1alpha1.SpireOIDCDiscoveryProvider
		oidcError    error
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "successful update with no changes",
			oldAgent:     createSpireAgent("example.com", "test-cluster"),
			newAgent:     createSpireAgent("example.com", "test-cluster"),
			serverExists: false,
			serverError:  kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			oidcExists:   false,
			oidcError:    kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:      false,
		},
		{
			name:         "failed update - trustDomain changed",
			oldAgent:     createSpireAgent("example.com", "test-cluster"),
			newAgent:     createSpireAgent("new-domain.com", "test-cluster"),
			serverExists: false,
			serverError:  kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			oidcExists:   false,
			oidcError:    kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:      true,
			errMsg:       "validation failed: trustDomain field is immutable and cannot be changed",
		},
		{
			name:         "failed update - clusterName changed",
			oldAgent:     createSpireAgent("example.com", "test-cluster"),
			newAgent:     createSpireAgent("example.com", "new-cluster"),
			serverExists: false,
			serverError:  kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			oidcExists:   false,
			oidcError:    kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:      true,
			errMsg:       "validation failed: clusterName field is immutable and cannot be changed",
		},
		{
			name:         "failed update - trustDomain mismatch with existing server",
			oldAgent:     createSpireAgent("example.com", "test-cluster"),
			newAgent:     createSpireAgent("example.com", "test-cluster"),
			serverExists: true,
			server:       createSpireServer("different.com", "test-cluster", "https://issuer.com"),
			oidcExists:   false,
			oidcError:    kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:      true,
			errMsg:       "validation failed: SpireAgent update trustDomain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockCustomClient{}
			validator := &SpireAgentValidator{Client: mockClient}

			// Only set up mocks if the validation won't fail on immutability
			isImmutabilityError := (tt.oldAgent.Spec.TrustDomain != tt.newAgent.Spec.TrustDomain) ||
				(tt.oldAgent.Spec.ClusterName != tt.newAgent.Spec.ClusterName)

			if !isImmutabilityError {
				// Mock server Get call
				if tt.serverExists {
					mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireServer")).
						Return(nil, tt.server).Once()
				} else {
					mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireServer")).
						Return(tt.serverError, nil).Once()
				}

				// Mock oidc Get call
				if tt.oidcExists {
					mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireOIDCDiscoveryProvider")).
						Return(nil, tt.oidc).Once()
				} else {
					mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireOIDCDiscoveryProvider")).
						Return(tt.oidcError, nil).Once()
				}
			}

			warnings, err := validator.ValidateUpdate(context.Background(), tt.oldAgent, tt.newAgent)

			assert.Nil(t, warnings)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestSpireAgentValidator_ValidateDelete(t *testing.T) {
	validator := &SpireAgentValidator{}
	agent := createSpireAgent("example.com", "test-cluster")

	warnings, err := validator.ValidateDelete(context.Background(), agent)

	assert.Nil(t, warnings)
	assert.NoError(t, err, "ValidateDelete should not perform any validation for SpireAgent")
}

// Test SpireOIDCDiscoveryProviderValidator
func TestSpireOIDCDiscoveryProviderValidator_ValidateCreate(t *testing.T) {
	tests := []struct {
		name         string
		oidc         *v1alpha1.SpireOIDCDiscoveryProvider
		server       *v1alpha1.SpireServer
		serverExists bool
		serverError  error
		agent        *v1alpha1.SpireAgent
		agentExists  bool
		agentError   error
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "successful creation with matching fields",
			oidc:         createSpireOIDCDiscoveryProvider("example.com", "https://issuer.com"),
			server:       createSpireServer("example.com", "test-cluster", "https://issuer.com"),
			serverExists: true,
			agent:        createSpireAgent("example.com", "test-cluster"),
			agentExists:  true,
			wantErr:      false,
		},
		{
			name:         "successful creation with normalized issuer matching",
			oidc:         createSpireOIDCDiscoveryProvider("example.com", "issuer.com"),
			server:       createSpireServer("example.com", "test-cluster", "https://issuer.com"),
			serverExists: true,
			agent:        createSpireAgent("example.com", "test-cluster"),
			agentExists:  true,
			wantErr:      false,
		},
		{
			name:         "successful creation with default issuer",
			oidc:         createSpireOIDCDiscoveryProvider("example.com", ""),
			server:       createSpireServer("example.com", "test-cluster", ""),
			serverExists: true,
			agent:        createSpireAgent("example.com", "test-cluster"),
			agentExists:  true,
			wantErr:      false,
		},
		{
			name:         "successful creation - server does not exist",
			oidc:         createSpireOIDCDiscoveryProvider("example.com", "https://issuer.com"),
			serverExists: false,
			serverError:  kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			agentExists:  false,
			agentError:   kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:      false,
		},
		{
			name:         "successful creation - agent does not exist",
			oidc:         createSpireOIDCDiscoveryProvider("example.com", "https://issuer.com"),
			server:       createSpireServer("example.com", "test-cluster", "https://issuer.com"),
			serverExists: true,
			agentExists:  false,
			agentError:   kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:      false,
		},
		{
			name:         "failed creation - trustDomain mismatch with server",
			oidc:         createSpireOIDCDiscoveryProvider("different.com", "https://issuer.com"),
			server:       createSpireServer("example.com", "test-cluster", "https://issuer.com"),
			serverExists: true,
			agent:        createSpireAgent("example.com", "test-cluster"),
			agentExists:  true,
			wantErr:      true,
			errMsg:       "validation failed: SpireOIDCDiscoveryProvider trustDomain",
		},
		{
			name:         "failed creation - trustDomain mismatch with agent",
			oidc:         createSpireOIDCDiscoveryProvider("example.com", "https://issuer.com"),
			server:       createSpireServer("example.com", "test-cluster", "https://issuer.com"),
			serverExists: true,
			agent:        createSpireAgent("different.com", "test-cluster"),
			agentExists:  true,
			wantErr:      true,
			errMsg:       "validation failed: SpireOIDCDiscoveryProvider trustDomain",
		},
		{
			name:         "failed creation - jwtIssuer mismatch",
			oidc:         createSpireOIDCDiscoveryProvider("example.com", "https://different-issuer.com"),
			server:       createSpireServer("example.com", "test-cluster", "https://issuer.com"),
			serverExists: true,
			agent:        createSpireAgent("example.com", "test-cluster"),
			agentExists:  true,
			wantErr:      true,
			errMsg:       "validation failed: SpireOIDCDiscoveryProvider jwtIssuer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockCustomClient{}
			validator := &SpireOIDCDiscoveryProviderValidator{Client: mockClient}

			// Mock server Get call
			if tt.serverExists {
				mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireServer")).
					Return(nil, tt.server).Once()
			} else {
				mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireServer")).
					Return(tt.serverError, nil).Once()
			}

			// Mock agent Get call - always called now since we check agent even if server doesn't exist
			if tt.agentExists {
				mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireAgent")).
					Return(nil, tt.agent).Once()
			} else {
				mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireAgent")).
					Return(tt.agentError, nil).Once()
			}

			warnings, err := validator.ValidateCreate(context.Background(), tt.oidc)

			assert.Nil(t, warnings)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestSpireOIDCDiscoveryProviderValidator_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name         string
		oldOIDC      *v1alpha1.SpireOIDCDiscoveryProvider
		newOIDC      *v1alpha1.SpireOIDCDiscoveryProvider
		serverExists bool
		server       *v1alpha1.SpireServer
		serverError  error
		agentExists  bool
		agent        *v1alpha1.SpireAgent
		agentError   error
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "successful update with no changes",
			oldOIDC:      createSpireOIDCDiscoveryProvider("example.com", "https://issuer.com"),
			newOIDC:      createSpireOIDCDiscoveryProvider("example.com", "https://issuer.com"),
			serverExists: false,
			serverError:  kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			agentExists:  false,
			agentError:   kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:      false,
		},
		{
			name:         "successful update with allowed field changes",
			oldOIDC:      createSpireOIDCDiscoveryProvider("example.com", "https://issuer.com"),
			newOIDC:      createSpireOIDCDiscoveryProvider("example.com", "https://new-issuer.com"),
			serverExists: false,
			serverError:  kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			agentExists:  false,
			agentError:   kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:      false,
		},
		{
			name:         "failed update - trustDomain changed",
			oldOIDC:      createSpireOIDCDiscoveryProvider("example.com", "https://issuer.com"),
			newOIDC:      createSpireOIDCDiscoveryProvider("new-domain.com", "https://issuer.com"),
			serverExists: false,
			serverError:  kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			agentExists:  false,
			agentError:   kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:      true,
			errMsg:       "validation failed: trustDomain field is immutable and cannot be changed",
		},
		{
			name:         "failed update - trustDomain mismatch with existing server",
			oldOIDC:      createSpireOIDCDiscoveryProvider("example.com", "https://issuer.com"),
			newOIDC:      createSpireOIDCDiscoveryProvider("example.com", "https://new-issuer.com"),
			serverExists: true,
			server:       createSpireServer("different.com", "test-cluster", "https://issuer.com"),
			agentExists:  false,
			agentError:   kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			wantErr:      true,
			errMsg:       "validation failed: SpireOIDCDiscoveryProvider update trustDomain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockCustomClient{}
			validator := &SpireOIDCDiscoveryProviderValidator{Client: mockClient}

			// Only set up mocks if the validation won't fail on immutability
			isImmutabilityError := (tt.oldOIDC.Spec.TrustDomain != tt.newOIDC.Spec.TrustDomain)

			if !isImmutabilityError {
				// Mock server Get call
				if tt.serverExists {
					mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireServer")).
						Return(nil, tt.server).Once()
				} else {
					mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireServer")).
						Return(tt.serverError, nil).Once()
				}

				// Mock agent Get call
				if tt.agentExists {
					mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireAgent")).
						Return(nil, tt.agent).Once()
				} else {
					mockClient.On("Get", mock.Anything, types.NamespacedName{Name: "cluster"}, mock.AnythingOfType("*v1alpha1.SpireAgent")).
						Return(tt.agentError, nil).Once()
				}
			}

			warnings, err := validator.ValidateUpdate(context.Background(), tt.oldOIDC, tt.newOIDC)

			assert.Nil(t, warnings)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestSpireOIDCDiscoveryProviderValidator_ValidateDelete(t *testing.T) {
	validator := &SpireOIDCDiscoveryProviderValidator{}
	oidc := createSpireOIDCDiscoveryProvider("example.com", "https://issuer.com")

	warnings, err := validator.ValidateDelete(context.Background(), oidc)

	assert.Nil(t, warnings)
	assert.NoError(t, err, "ValidateDelete should not perform any validation for SpireOIDCDiscoveryProvider")
}
