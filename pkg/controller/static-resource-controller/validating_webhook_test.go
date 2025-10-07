package static_resource_controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func TestGetSpireControllerManagerValidatingWebhookConfiguration(t *testing.T) {
	r := &StaticResourceReconciler{}

	vwc := r.GetSpireControllerManagerValidatingWebhookConfiguration()
	require.NotNil(t, vwc, "Expected ValidatingWebhookConfiguration to be not nil")

	// Metadata check
	assert.Equal(t, "spire-controller-manager-webhook", vwc.Name)
	assert.Contains(t, vwc.Labels, "app.kubernetes.io/name")
	assert.Equal(t, "spire-controller-manager", vwc.Labels["app.kubernetes.io/name"])

	// NOTE: ValidatingWebhookConfiguration is cluster-scoped, so Namespace is empty.
	// assert.Equal(t, "zero-trust-workload-identity-manager", vwc.Namespace) // Remove this test

	require.Len(t, vwc.Webhooks, 2)

	// Helper to dereference string pointers safely
	strPtrValue := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}

	// First webhook
	wh1 := vwc.Webhooks[0]
	assert.Equal(t, "vclusterfederatedtrustdomain.kb.io", wh1.Name)
	require.NotNil(t, wh1.ClientConfig.Service)
	assert.Equal(t, "spire-controller-manager-webhook", wh1.ClientConfig.Service.Name)
	assert.Equal(t, "zero-trust-workload-identity-manager", wh1.ClientConfig.Service.Namespace)
	assert.Equal(t, "/validate-spire-spiffe-io-v1alpha1-clusterfederatedtrustdomain", strPtrValue(wh1.ClientConfig.Service.Path))
	require.NotNil(t, wh1.FailurePolicy)
	assert.Equal(t, admissionregistrationv1.Fail, *wh1.FailurePolicy)
	require.NotNil(t, wh1.SideEffects)
	assert.Equal(t, admissionregistrationv1.SideEffectClassNone, *wh1.SideEffects)

	require.Len(t, wh1.Rules, 1)
	rule1 := wh1.Rules[0]
	assert.ElementsMatch(t, []string{"spire.spiffe.io"}, rule1.APIGroups)
	assert.ElementsMatch(t, []string{"v1alpha1"}, rule1.APIVersions)
	assert.ElementsMatch(t, []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}, rule1.Operations)
	assert.ElementsMatch(t, []string{"clusterfederatedtrustdomains"}, rule1.Resources)

	// Second webhook
	wh2 := vwc.Webhooks[1]
	assert.Equal(t, "vclusterspiffeid.kb.io", wh2.Name)
	require.NotNil(t, wh2.ClientConfig.Service)
	assert.Equal(t, "spire-controller-manager-webhook", wh2.ClientConfig.Service.Name)
	assert.Equal(t, "zero-trust-workload-identity-manager", wh2.ClientConfig.Service.Namespace)
	assert.Equal(t, "/validate-spire-spiffe-io-v1alpha1-clusterspiffeid", strPtrValue(wh2.ClientConfig.Service.Path))
	require.NotNil(t, wh2.FailurePolicy)
	assert.Equal(t, admissionregistrationv1.Fail, *wh2.FailurePolicy)
	require.NotNil(t, wh2.SideEffects)
	assert.Equal(t, admissionregistrationv1.SideEffectClassNone, *wh2.SideEffects)

	require.Len(t, wh2.Rules, 1)
	rule2 := wh2.Rules[0]
	assert.ElementsMatch(t, []string{"spire.spiffe.io"}, rule2.APIGroups)
	assert.ElementsMatch(t, []string{"v1alpha1"}, rule2.APIVersions)
	assert.ElementsMatch(t, []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}, rule2.Operations)
	assert.ElementsMatch(t, []string{"clusterspiffeids"}, rule2.Resources)
}
