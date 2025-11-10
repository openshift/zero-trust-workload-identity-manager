package spiffe_csi_driver

import (
	"context"
	"fmt"

	storagev1 "k8s.io/api/storage/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

// Constants for status conditions are defined in controller.go

// reconcileCSIDriver reconciles the Spiffe CSI Driver resource
func (r *SpiffeCsiReconciler) reconcileCSIDriver(ctx context.Context, driver *v1alpha1.SpiffeCSIDriver, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpiffeCSIDriver(driver.Spec.Labels)

	if err := controllerutil.SetControllerReference(driver, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on CSI driver")
		statusMgr.AddCondition(CSIDriverAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on CSIDriver: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Get existing resource (from cache)
	existing := &storagev1.CSIDriver{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get CSI driver")
			statusMgr.AddCondition(CSIDriverAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get CSIDriver: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create CSI driver")
			statusMgr.AddCondition(CSIDriverAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create CSIDriver: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created CSIDriver", "name", desired.Name)
		statusMgr.AddCondition(CSIDriverAvailable, v1alpha1.ReasonReady,
			"All CSIDriver resources available",
			metav1.ConditionTrue)
		return nil
	}

	// Resource exists, check if we need to update
	if createOnlyMode {
		r.log.V(1).Info("CSIDriver exists, skipping update due to create-only mode", "name", desired.Name)
		statusMgr.AddCondition(CSIDriverAvailable, v1alpha1.ReasonReady,
			"All CSIDriver resources available",
			metav1.ConditionTrue)
		return nil
	}

	// Check if update is needed
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("CSIDriver is up to date", "name", desired.Name)
		statusMgr.AddCondition(CSIDriverAvailable, v1alpha1.ReasonReady,
			"All CSIDriver resources available",
			metav1.ConditionTrue)
		return nil
	}

	// Update the resource
	desired.ResourceVersion = existing.ResourceVersion
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update CSI driver")
		statusMgr.AddCondition(CSIDriverAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update CSIDriver: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated CSIDriver", "name", desired.Name)
	statusMgr.AddCondition(CSIDriverAvailable, v1alpha1.ReasonReady,
		"All CSIDriver resources available",
		metav1.ConditionTrue)
	return nil
}

// getSpiffeCSIDriver returns the Spiffe CSI Driver with proper labels
func getSpiffeCSIDriver(customLabels map[string]string) *storagev1.CSIDriver {
	csiDriver := utils.DecodeCsiDriverObjBytes(assets.MustAsset(utils.SpiffeCsiDriverAssetName))
	for k, v := range customLabels {
		csiDriver.Labels[k] = v
	}
	csiDriver.Labels = utils.SpiffeCSIDriverLabels(csiDriver.Labels)
	return csiDriver
}
