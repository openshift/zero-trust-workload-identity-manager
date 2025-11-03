package spiffe_csi_driver

import (
	"context"
	"fmt"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

// Constants for status conditions are defined in controller.go

// reconcileCSIDriver reconciles the Spiffe CSI Driver resource
func (r *SpiffeCsiReconciler) reconcileCSIDriver(ctx context.Context, driver *v1alpha1.SpiffeCSIDriver, statusMgr *status.Manager, createOnlyMode bool) error {
	csiDriver := getSpiffeCSIDriver()

	if err := controllerutil.SetControllerReference(driver, csiDriver, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on CSI driver")
		statusMgr.AddCondition(CSIDriverAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on CSIDriver: %v", err),
			metav1.ConditionFalse)
		return err
	}

	if err := r.createOrUpdateResource(ctx, csiDriver, createOnlyMode); err != nil {
		statusMgr.AddCondition(CSIDriverAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to create CSIDriver: %v", err),
			metav1.ConditionFalse)
		return err
	}

	statusMgr.AddCondition(CSIDriverAvailable, v1alpha1.ReasonReady,
		"All CSIDriver resources available",
		metav1.ConditionTrue)
	return nil
}

// getSpiffeCSIDriver returns the Spiffe CSI Driver with proper labels
func getSpiffeCSIDriver() *storagev1.CSIDriver {
	csiDriver := utils.DecodeCsiDriverObjBytes(assets.MustAsset(utils.SpiffeCsiDriverAssetName))
	csiDriver.Labels = utils.SpiffeCSIDriverLabels(csiDriver.Labels)
	return csiDriver
}
