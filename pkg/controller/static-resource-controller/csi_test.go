package static_resource_controller

import (
	"testing"

	storagev1 "k8s.io/api/storage/v1"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/version"
	"github.com/stretchr/testify/assert"
)

func TestGetSpiffeCsiObject(t *testing.T) {
	reconciler := &StaticResourceReconciler{}

	csiDriver := reconciler.getSpiffeCsiObject()

	assert.NotNil(t, csiDriver)
	assert.Equal(t, "csi.spiffe.io", csiDriver.Name)

	// Validate pointer fields
	if assert.NotNil(t, csiDriver.Spec.AttachRequired) {
		assert.Equal(t, false, *csiDriver.Spec.AttachRequired)
	}
	if assert.NotNil(t, csiDriver.Spec.PodInfoOnMount) {
		assert.Equal(t, true, *csiDriver.Spec.PodInfoOnMount)
	}
	if assert.NotNil(t, csiDriver.Spec.FSGroupPolicy) {
		assert.Equal(t, storagev1.FSGroupPolicy("None"), *csiDriver.Spec.FSGroupPolicy)
	}

	assert.ElementsMatch(t, []storagev1.VolumeLifecycleMode{"Ephemeral"}, csiDriver.Spec.VolumeLifecycleModes)

	expectedLabels := map[string]string{
		"security.openshift.io/csi-ephemeral-volume-profile": "restricted",
		"app.kubernetes.io/name":                             "spiffe-csi-driver",
		"app.kubernetes.io/instance":                         "cluster-zero-trust-workload-identity-manager",
		"app.kubernetes.io/component":                        "csi",
		"app.kubernetes.io/managed-by":                       "zero-trust-workload-identity-manager",
		"app.kubernetes.io/part-of":                          "zero-trust-workload-identity-manager",
		"app.kubernetes.io/version":                          version.SpiffeCsiVersion,
	}
	assert.Equal(t, expectedLabels, csiDriver.Labels)
}
