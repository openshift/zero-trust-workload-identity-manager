package spiffe_csi_driver

import (
	"context"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

const legacyCSIDriverSCCName = "spire-spiffe-csi-driver"

// legacyCSIDriverSCCPredicate limits SCC watches to the legacy CSI driver SCC only.
func legacyCSIDriverSCCPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == legacyCSIDriverSCCName
	})
}

// deleteLegacyCSIDriverSCC removes the operator-managed legacy custom SCC.
// CSI pods are pinned to the privileged SCC via openshift.io/required-scc on the DaemonSet.
func (r *SpiffeCsiReconciler) deleteLegacyCSIDriverSCC(ctx context.Context) error {
	scc := &securityv1.SecurityContextConstraints{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: legacyCSIDriverSCCName}, scc)
	if err != nil {
		if kerrors.IsNotFound(err) {
			r.log.V(1).Info("Legacy CSI driver SCC not found, nothing to clean up")
			return nil
		}
		r.log.Error(err, "failed to get legacy CSI driver SCC")
		return err
	}

	if scc.Labels == nil || scc.Labels[utils.AppManagedByLabelKey] != utils.AppManagedByLabelValue {
		r.log.Info("Legacy CSI driver SCC exists but is not managed by the operator, skipping delete",
			"name", legacyCSIDriverSCCName)
		return nil
	}

	if err := r.ctrlClient.Delete(ctx, scc); err != nil {
		if kerrors.IsNotFound(err) {
			return nil
		}
		r.log.Error(err, "failed to delete legacy CSI driver SCC")
		return err
	}

	r.log.Info("Deleted legacy CSI driver SCC", "name", legacyCSIDriverSCCName)
	return nil
}
