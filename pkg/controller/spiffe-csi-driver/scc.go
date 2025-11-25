package spiffe_csi_driver

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/utils/ptr"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

// generateSpiffeCSIDriverSCC returns a pointer to the desired SCC object
func generateSpiffeCSIDriverSCC(customLabels map[string]string) *securityv1.SecurityContextConstraints {
	csiServiceAccountUser := "system:serviceaccount:" + utils.GetOperatorNamespace() + ":spire-spiffe-csi-driver"
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "spire-spiffe-csi-driver",
			Labels: utils.SpiffeCSIDriverLabels(customLabels),
		},
		ReadOnlyRootFilesystem: true,
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type: securityv1.RunAsUserStrategyMustRunAsRange,
		},
		SELinuxContext: securityv1.SELinuxContextStrategyOptions{
			Type: securityv1.SELinuxStrategyMustRunAs,
		},
		SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
			Type: securityv1.SupplementalGroupsStrategyMustRunAs,
		},
		FSGroup: securityv1.FSGroupStrategyOptions{
			Type: securityv1.FSGroupStrategyMustRunAs,
		},
		Users: []string{
			csiServiceAccountUser,
		},
		Volumes: []securityv1.FSType{
			securityv1.FSTypeConfigMap,
			securityv1.FSTypeHostPath,
			securityv1.FSTypeSecret,
		},
		AllowHostDirVolumePlugin: true,
		AllowHostIPC:             false,
		AllowHostNetwork:         false,
		AllowHostPID:             false,
		AllowHostPorts:           false,
		AllowPrivilegeEscalation: ptr.To(true),
		AllowPrivilegedContainer: true,
		DefaultAddCapabilities:   []corev1.Capability{},
		RequiredDropCapabilities: []corev1.Capability{
			"ALL",
		},
	}
}

// reconcileSCC reconciles the Spiffe CSI Driver Security Context Constraints
func (r *SpiffeCsiReconciler) reconcileSCC(ctx context.Context, driver *v1alpha1.SpiffeCSIDriver, statusMgr *status.Manager) error {
	desired := generateSpiffeCSIDriverSCC(driver.Spec.Labels)
	if err := controllerutil.SetControllerReference(driver, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set the owner reference for the SCC resource")
		statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpiffeCSISCCGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	// Get existing resource (from cache)
	existing := &securityv1.SecurityContextConstraints{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get SecurityContextConstraints")
			statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpiffeCSISCCGetFailed",
				fmt.Sprintf("Failed to get SecurityContextConstraints: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "Failed to create SpiffeCsiSCC")
			statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpiffeCSISCCCreationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created SecurityContextConstraints", "name", desired.Name)
		statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpiffeCSISCCResourceCreated",
			"SpiffeCSISCC resource created",
			metav1.ConditionTrue)
		return nil
	}

	// Preserve fields set by OpenShift from existing resource BEFORE comparison
	desired.ResourceVersion = existing.ResourceVersion
	desired.Priority = existing.Priority
	if existing.UserNamespaceLevel != "" {
		desired.UserNamespaceLevel = existing.UserNamespaceLevel
	}

	// Resource exists, check if we need to update
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("SecurityContextConstraints is up to date", "name", desired.Name)
		statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpiffeCSISCCResourceUpToDate",
			"SpiffeCSISCC resource is up to date",
			metav1.ConditionTrue)
		return nil
	}

	// Update the resource
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "Failed to update SpiffeCsiSCC")
		statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpiffeCSISCCUpdateFailed",
			fmt.Sprintf("Failed to update SecurityContextConstraints: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated SecurityContextConstraints", "name", desired.Name)
	statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpiffeCSISCCResourceUpdated",
		"SpiffeCSISCC resource updated",
		metav1.ConditionTrue)
	return nil
}
