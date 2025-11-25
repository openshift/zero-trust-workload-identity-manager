package spire_oidc_discovery_provider

import (
	"context"
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
)

// reconcileClusterSpiffeIDs reconciles the ClusterSpiffeID resources
func (r *SpireOidcDiscoveryProviderReconciler) reconcileClusterSpiffeIDs(ctx context.Context, oidc *v1alpha1.SpireOIDCDiscoveryProvider, statusMgr *status.Manager, createOnlyMode bool) error {
	// Reconcile OIDC Discovery Provider ClusterSPIFFEID
	desiredOIDC := generateSpireIODCDiscoveryProviderSpiffeID(oidc.Spec.Labels)
	if err := controllerutil.SetControllerReference(oidc, desiredOIDC, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference for OIDC ClusterSPIFFEID")
		statusMgr.AddCondition(ClusterSPIFFEIDAvailable, "SpireClusterSpiffeIDGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	// Get existing OIDC ClusterSPIFFEID (from cache)
	existingOIDC := &spiffev1alpha1.ClusterSPIFFEID{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desiredOIDC.Name}, existingOIDC)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get OIDC ClusterSPIFFEID")
			statusMgr.AddCondition(ClusterSPIFFEIDAvailable, "SpireClusterSpiffeIDGetFailed",
				fmt.Sprintf("Failed to get OIDC ClusterSPIFFEID: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desiredOIDC); err != nil {
			r.log.Error(err, "Failed to create oidc cluster spiffe id")
			statusMgr.AddCondition(ClusterSPIFFEIDAvailable, "SpireClusterSpiffeIDCreationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return err
		}
		r.log.Info("Created OIDC ClusterSPIFFEID", "name", desiredOIDC.Name)
	} else {
		// Resource exists, check if we need to update
		if utils.ResourceNeedsUpdate(existingOIDC, desiredOIDC) {
			if createOnlyMode {
				// Skip update in create-only mode
				r.log.Info("Skipping OIDC ClusterSPIFFEID update due to create-only mode", "name", desiredOIDC.Name)
			} else {
				// Update the resource
				desiredOIDC.ResourceVersion = existingOIDC.ResourceVersion
				if err := r.ctrlClient.Update(ctx, desiredOIDC); err != nil {
					r.log.Error(err, "Failed to update OIDC ClusterSPIFFEID")
					statusMgr.AddCondition(ClusterSPIFFEIDAvailable, "SpireClusterSpiffeIDUpdateFailed",
						fmt.Sprintf("Failed to update OIDC ClusterSPIFFEID: %v", err),
						metav1.ConditionFalse)
					return err
				}
				r.log.Info("Updated OIDC ClusterSPIFFEID", "name", desiredOIDC.Name)
			}
		} else {
			r.log.V(1).Info("OIDC ClusterSPIFFEID is up to date", "name", desiredOIDC.Name)
		}
	}

	// Reconcile Default Fallback ClusterSPIFFEID
	desiredDefault := generateDefaultFallbackClusterSPIFFEID(oidc.Spec.Labels)
	if err = controllerutil.SetControllerReference(oidc, desiredDefault, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference for default ClusterSPIFFEID")
		statusMgr.AddCondition(ClusterSPIFFEIDAvailable, "SpireClusterSpiffeIDGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	// Get existing Default ClusterSPIFFEID (from cache)
	existingDefault := &spiffev1alpha1.ClusterSPIFFEID{}
	err = r.ctrlClient.Get(ctx, types.NamespacedName{Name: desiredDefault.Name}, existingDefault)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get Default ClusterSPIFFEID")
			statusMgr.AddCondition(ClusterSPIFFEIDAvailable, "SpireClusterSpiffeIDGetFailed",
				fmt.Sprintf("Failed to get Default ClusterSPIFFEID: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desiredDefault); err != nil {
			r.log.Error(err, "Failed to create DefaultFallbackClusterSPIFFEID")
			statusMgr.AddCondition(ClusterSPIFFEIDAvailable, "SpireClusterSpiffeIDCreationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return err
		}
		r.log.Info("Created Default ClusterSPIFFEID", "name", desiredDefault.Name)
	} else {
		// Resource exists, check if we need to update
		if utils.ResourceNeedsUpdate(existingDefault, desiredDefault) {
			if createOnlyMode {
				// Skip update in create-only mode
				r.log.Info("Skipping Default ClusterSPIFFEID update due to create-only mode", "name", desiredDefault.Name)
			} else {
				// Update the resource
				desiredDefault.ResourceVersion = existingDefault.ResourceVersion
				if err := r.ctrlClient.Update(ctx, desiredDefault); err != nil {
					r.log.Error(err, "Failed to update Default ClusterSPIFFEID")
					statusMgr.AddCondition(ClusterSPIFFEIDAvailable, "SpireClusterSpiffeIDUpdateFailed",
						fmt.Sprintf("Failed to update Default ClusterSPIFFEID: %v", err),
						metav1.ConditionFalse)
					return err
				}
				r.log.Info("Updated Default ClusterSPIFFEID", "name", desiredDefault.Name)
			}
		} else {
			r.log.V(1).Info("Default ClusterSPIFFEID is up to date", "name", desiredDefault.Name)
		}
	}

	statusMgr.AddCondition(ClusterSPIFFEIDAvailable, "SpireClusterSpiffeIDResourcesReady",
		"Spire OIDC and default ClusterSpiffeID resources are ready",
		metav1.ConditionTrue)
	return nil
}

func generateSpireIODCDiscoveryProviderSpiffeID(customLabels map[string]string) *spiffev1alpha1.ClusterSPIFFEID {
	clusterSpiffeID := &spiffev1alpha1.ClusterSPIFFEID{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "zero-trust-workload-identity-manager-spire-oidc-discovery-provider",
			Labels: utils.SpireOIDCDiscoveryProviderLabels(customLabels),
		},
		Spec: spiffev1alpha1.ClusterSPIFFEIDSpec{
			ClassName:        "zero-trust-workload-identity-manager-spire",
			Hint:             "oidc-discovery-provider",
			SPIFFEIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}",
			DNSNameTemplates: []string{
				"oidc-discovery.{{ .TrustDomain }}",
			},
			AutoPopulateDNSNames: true,
			PodSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":      "spiffe-oidc-discovery-provider",
					"app.kubernetes.io/instance":  "cluster-zero-trust-workload-identity-manager",
					"app.kubernetes.io/component": "discovery",
				},
			},
			NamespaceSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "kubernetes.io/metadata.name",
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{
							utils.GetOperatorNamespace(),
						},
					},
				},
			},
		},
	}
	return clusterSpiffeID
}

func generateDefaultFallbackClusterSPIFFEID(customLabels map[string]string) *spiffev1alpha1.ClusterSPIFFEID {
	clusterSpiffeID := &spiffev1alpha1.ClusterSPIFFEID{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "zero-trust-workload-identity-manager-spire-default",
			Labels: utils.SpireOIDCDiscoveryProviderLabels(customLabels),
		},
		Spec: spiffev1alpha1.ClusterSPIFFEIDSpec{
			ClassName:        "zero-trust-workload-identity-manager-spire",
			Hint:             "default",
			SPIFFEIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}",
			Fallback:         true,
			NamespaceSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "kubernetes.io/metadata.name",
						Operator: metav1.LabelSelectorOpNotIn,
						Values: []string{
							utils.GetOperatorNamespace(),
						},
					},
				},
			},
		},
	}
	return clusterSpiffeID
}
