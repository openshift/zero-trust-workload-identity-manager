package spire_oidc_discovery_provider

import (
	"testing"

	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateSpireIODCDiscoveryProviderSpiffeID(t *testing.T) {
	t.Run("should return valid ClusterSPIFFEID for OIDC discovery provider", func(t *testing.T) {
		// Act
		result := generateSpireIODCDiscoveryProviderSpiffeID()

		// Assert
		require.NotNil(t, result)
		assert.IsType(t, &spiffev1alpha1.ClusterSPIFFEID{}, result)

		// Verify ObjectMeta
		assert.Equal(t, "zero-trust-workload-identity-manager-spire-oidc-discovery-provider", result.ObjectMeta.Name)

		// Verify Spec fields
		assert.Equal(t, "oidc-discovery-provider", result.Spec.Hint)
		assert.Equal(t, "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}", result.Spec.SPIFFEIDTemplate)
		assert.True(t, result.Spec.AutoPopulateDNSNames)
		assert.False(t, result.Spec.Fallback) // Should be false (default value)

		// Verify DNSNameTemplates
		expectedDNSNames := []string{"oidc-discovery.{{ .TrustDomain }}"}
		assert.Equal(t, expectedDNSNames, result.Spec.DNSNameTemplates)

		// Verify PodSelector
		require.NotNil(t, result.Spec.PodSelector)
		expectedPodLabels := map[string]string{
			"app.kubernetes.io/name":      "spiffe-oidc-discovery-provider",
			"app.kubernetes.io/instance":  "cluster-zero-trust-workload-identity-manager",
			"app.kubernetes.io/component": "discovery",
		}
		assert.Equal(t, expectedPodLabels, result.Spec.PodSelector.MatchLabels)

		// Verify NamespaceSelector
		require.NotNil(t, result.Spec.NamespaceSelector)
		require.Len(t, result.Spec.NamespaceSelector.MatchExpressions, 1)

		nsExpression := result.Spec.NamespaceSelector.MatchExpressions[0]
		assert.Equal(t, "kubernetes.io/metadata.name", nsExpression.Key)
		assert.Equal(t, metav1.LabelSelectorOpIn, nsExpression.Operator)
		expectedNamespaces := []string{
			"zero-trust-workload-identity-manager",
		}
		assert.Equal(t, expectedNamespaces, nsExpression.Values)
	})

	t.Run("should return consistent results across multiple calls", func(t *testing.T) {
		// Act
		result1 := generateSpireIODCDiscoveryProviderSpiffeID()
		result2 := generateSpireIODCDiscoveryProviderSpiffeID()

		// Assert
		assert.Equal(t, result1.ObjectMeta.Name, result2.ObjectMeta.Name)
		assert.Equal(t, result1.Spec.Hint, result2.Spec.Hint)
		assert.Equal(t, result1.Spec.SPIFFEIDTemplate, result2.Spec.SPIFFEIDTemplate)
		assert.Equal(t, result1.Spec.DNSNameTemplates, result2.Spec.DNSNameTemplates)
		assert.Equal(t, result1.Spec.AutoPopulateDNSNames, result2.Spec.AutoPopulateDNSNames)
	})
}

func TestGenerateDefaultFallbackClusterSPIFFEID(t *testing.T) {
	t.Run("should return valid ClusterSPIFFEID for default fallback", func(t *testing.T) {
		// Act
		result := generateDefaultFallbackClusterSPIFFEID()

		// Assert
		require.NotNil(t, result)
		assert.IsType(t, &spiffev1alpha1.ClusterSPIFFEID{}, result)

		// Verify ObjectMeta
		assert.Equal(t, "zero-trust-workload-identity-manager-spire-default", result.ObjectMeta.Name)

		// Verify Spec fields
		assert.Equal(t, "default", result.Spec.Hint)
		assert.Equal(t, "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}", result.Spec.SPIFFEIDTemplate)
		assert.True(t, result.Spec.Fallback)
		assert.False(t, result.Spec.AutoPopulateDNSNames) // Should be false (default value)

		// Verify DNSNameTemplates is empty
		assert.Nil(t, result.Spec.DNSNameTemplates)

		// Verify PodSelector is nil
		assert.Nil(t, result.Spec.PodSelector)

		// Verify NamespaceSelector
		require.NotNil(t, result.Spec.NamespaceSelector)
		require.Len(t, result.Spec.NamespaceSelector.MatchExpressions, 1)

		nsExpression := result.Spec.NamespaceSelector.MatchExpressions[0]
		assert.Equal(t, "kubernetes.io/metadata.name", nsExpression.Key)
		assert.Equal(t, metav1.LabelSelectorOpNotIn, nsExpression.Operator)
		expectedNamespaces := []string{
			"zero-trust-workload-identity-manager",
		}
		assert.Equal(t, expectedNamespaces, nsExpression.Values)
	})

	t.Run("should return consistent results across multiple calls", func(t *testing.T) {
		// Act
		result1 := generateDefaultFallbackClusterSPIFFEID()
		result2 := generateDefaultFallbackClusterSPIFFEID()

		// Assert
		assert.Equal(t, result1.ObjectMeta.Name, result2.ObjectMeta.Name)
		assert.Equal(t, result1.Spec.Hint, result2.Spec.Hint)
		assert.Equal(t, result1.Spec.SPIFFEIDTemplate, result2.Spec.SPIFFEIDTemplate)
		assert.Equal(t, result1.Spec.Fallback, result2.Spec.Fallback)
		assert.Equal(t, result1.Spec.AutoPopulateDNSNames, result2.Spec.AutoPopulateDNSNames)
	})
}

func TestBothFunctions_DifferentBehaviors(t *testing.T) {
	t.Run("should have different configurations between OIDC and default fallback", func(t *testing.T) {
		// Act
		oidcResult := generateSpireIODCDiscoveryProviderSpiffeID()
		defaultResult := generateDefaultFallbackClusterSPIFFEID()

		// Assert - Names should be different
		assert.NotEqual(t, oidcResult.ObjectMeta.Name, defaultResult.ObjectMeta.Name)

		// Assert - Hints should be different
		assert.NotEqual(t, oidcResult.Spec.Hint, defaultResult.Spec.Hint)

		// Assert - Fallback behavior should be different
		assert.False(t, oidcResult.Spec.Fallback)
		assert.True(t, defaultResult.Spec.Fallback)

		// Assert - AutoPopulateDNSNames should be different
		assert.True(t, oidcResult.Spec.AutoPopulateDNSNames)
		assert.False(t, defaultResult.Spec.AutoPopulateDNSNames)

		// Assert - PodSelector presence should be different
		assert.NotNil(t, oidcResult.Spec.PodSelector)
		assert.Nil(t, defaultResult.Spec.PodSelector)

		// Assert - DNSNameTemplates presence should be different
		assert.NotNil(t, oidcResult.Spec.DNSNameTemplates)
		assert.Nil(t, defaultResult.Spec.DNSNameTemplates)

		// Assert - NamespaceSelector operators should be different
		assert.Equal(t, metav1.LabelSelectorOpIn, oidcResult.Spec.NamespaceSelector.MatchExpressions[0].Operator)
		assert.Equal(t, metav1.LabelSelectorOpNotIn, defaultResult.Spec.NamespaceSelector.MatchExpressions[0].Operator)

		// Assert - Same SPIFFE ID template
		assert.Equal(t, oidcResult.Spec.SPIFFEIDTemplate, defaultResult.Spec.SPIFFEIDTemplate)

		// Assert - Same namespace values (but different operators)
		assert.Equal(t, oidcResult.Spec.NamespaceSelector.MatchExpressions[0].Values,
			defaultResult.Spec.NamespaceSelector.MatchExpressions[0].Values)
	})
}

// Benchmark tests (optional)
func BenchmarkGenerateSpireIODCDiscoveryProviderSpiffeID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateSpireIODCDiscoveryProviderSpiffeID()
	}
}

func BenchmarkGenerateDefaultFallbackClusterSPIFFEID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateDefaultFallbackClusterSPIFFEID()
	}
}

// Table-driven test for namespace values validation
func TestNamespaceValues(t *testing.T) {
	expectedNamespaces := []string{
		"zero-trust-workload-identity-manager",
	}

	testCases := []struct {
		name     string
		function func() *spiffev1alpha1.ClusterSPIFFEID
	}{
		{
			name:     "OIDC Discovery Provider",
			function: generateSpireIODCDiscoveryProviderSpiffeID,
		},
		{
			name:     "Default Fallback",
			function: generateDefaultFallbackClusterSPIFFEID,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.function()

			require.NotNil(t, result.Spec.NamespaceSelector)
			require.Len(t, result.Spec.NamespaceSelector.MatchExpressions, 1)

			actualNamespaces := result.Spec.NamespaceSelector.MatchExpressions[0].Values
			assert.Equal(t, expectedNamespaces, actualNamespaces)
		})
	}
}
