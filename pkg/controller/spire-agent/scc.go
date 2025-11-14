package spire_agent

import (
	"context"
	"fmt"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	securityv1 "github.com/openshift/api/security/v1"
)

// generateSpireAgentSCC returns a SecurityContextConstraints object for spire-agent
func generateSpireAgentSCC(config *v1alpha1.SpireAgent) *securityv1.SecurityContextConstraints {
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "spire-agent",
			Labels: utils.SpireAgentLabels(config.Spec.Labels),
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
			"system:serviceaccount:zero-trust-workload-identity-manager:spire-agent",
		},
		Volumes: []securityv1.FSType{
			securityv1.FSTypeConfigMap,
			securityv1.FSTypeHostPath,
			securityv1.FSProjected,
			securityv1.FSTypeSecret,
			securityv1.FSTypeEmptyDir,
		},
		AllowHostDirVolumePlugin: true,
		AllowHostIPC:             false,
		AllowHostNetwork:         true,
		AllowHostPID:             true,
		AllowHostPorts:           true,
		AllowPrivilegeEscalation: ptr.To(true),
		AllowPrivilegedContainer: true,
		AllowedCapabilities:      []corev1.Capability{},
		DefaultAddCapabilities:   []corev1.Capability{},
		RequiredDropCapabilities: []corev1.Capability{
			"ALL",
		},
		Groups: []string{},
	}
}

// reconcileSCC reconciles the Spire Agent Security Context Constraints
func (r *SpireAgentReconciler) reconcileSCC(ctx context.Context, agent *v1alpha1.SpireAgent, statusMgr *status.Manager) error {
	desired := generateSpireAgentSCC(agent)
	if err := controllerutil.SetControllerReference(agent, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpireAgentSCCGenerationFailed",
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
			statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpireAgentSCCGetFailed",
				fmt.Sprintf("Failed to get SecurityContextConstraints: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "Failed to create SpireAgentSCC")
			statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpireAgentSCCCreationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created SecurityContextConstraints", "name", desired.Name)
		statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpireAgentSCCResourceCreated",
			"Spire Agent SCC resources applied",
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
		statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpireAgentSCCResourceUpToDate",
			"Spire Agent SCC resources are up to date",
			metav1.ConditionTrue)
		return nil
	}

	// Update the resource
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "Failed to update SpireAgentSCC")
		statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpireAgentSCCUpdateFailed",
			fmt.Sprintf("Failed to update SecurityContextConstraints: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated SecurityContextConstraints", "name", desired.Name)
	statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpireAgentSCCResourceUpdated",
		"Spire Agent SCC resources updated",
		metav1.ConditionTrue)
	return nil
}
