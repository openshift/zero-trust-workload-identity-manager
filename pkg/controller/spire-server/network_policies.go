package spire_server

import (
	"context"

	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

var spireServerBaselineNetworkPolicyAssets = []string{
	utils.SpireServerNetworkPolicyIngressAssetName,
}

func (r *SpireServerReconciler) reconcileNetworkPolicies(ctx context.Context, server *v1alpha1.SpireServer, createOnlyMode bool) error {
	applied := utils.BaselineNetworkPolicyLastApplied(server.Status.NetworkPolicy, len(spireServerBaselineNetworkPolicyAssets))
	if !utils.ManagedNetworkPolicyEnabled(server.Spec.ManagedNetworkPolicy) {
		r.log.V(1).Info("Operand NetworkPolicy management is disabled")
		server.Status.NetworkPolicy = utils.NetworkPolicyStatusWith(v1alpha1.NetworkPolicyManagementUnmanaged, applied)
		return nil
	}

	defer func() {
		server.Status.NetworkPolicy = utils.NetworkPolicyStatusWith(v1alpha1.NetworkPolicyManagementManaged, applied)
	}()

	for _, assetName := range spireServerBaselineNetworkPolicyAssets {
		desired := networkPolicyFromAsset(assetName, server.Spec.Labels)
		if err := controllerutil.SetControllerReference(server, desired, r.scheme); err != nil {
			r.log.Error(err, "failed to set controller reference on NetworkPolicy", "name", desired.Name)
			return err
		}

		existing := &networkingv1.NetworkPolicy{}
		err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
		if err != nil {
			if !kerrors.IsNotFound(err) {
				r.log.Error(err, "failed to get NetworkPolicy", "name", desired.Name, "namespace", desired.Namespace)
				return err
			}
			if err := r.ctrlClient.Create(ctx, desired); err != nil {
				r.log.Error(err, "failed to create NetworkPolicy", "name", desired.Name, "namespace", desired.Namespace)
				return err
			}
			r.log.Info("Created NetworkPolicy", "name", desired.Name, "namespace", desired.Namespace)
			applied = utils.RecordNetworkPolicyApplied(applied, desired.Name, metav1.Now(), true)
			continue
		}

		if createOnlyMode {
			if utils.OperandManagedResourceNeedsUpdate(existing, desired, server) {
				r.log.V(1).Info("Skipping NetworkPolicy update due to create-only mode", "name", desired.Name)
			}
			applied = utils.RecordNetworkPolicyApplied(applied, desired.Name, existing.CreationTimestamp, false)
			continue
		}

		if utils.OperandManagedResourceNeedsUpdate(existing, desired, server) {
			desired.ResourceVersion = existing.ResourceVersion
			if err := r.ctrlClient.Update(ctx, desired); err != nil {
				r.log.Error(err, "failed to update NetworkPolicy", "name", desired.Name, "namespace", desired.Namespace)
				return err
			}
			r.log.Info("Updated NetworkPolicy", "name", desired.Name, "namespace", desired.Namespace)
			applied = utils.RecordNetworkPolicyApplied(applied, desired.Name, metav1.Now(), true)
			continue
		}

		r.log.V(1).Info("NetworkPolicy is up to date", "name", desired.Name)
		applied = utils.RecordNetworkPolicyApplied(applied, desired.Name, existing.CreationTimestamp, false)
	}

	return nil
}

func networkPolicyFromAsset(assetName string, customLabels map[string]string) *networkingv1.NetworkPolicy {
	np := utils.DecodeNetworkPolicyObjBytes(assets.MustAsset(assetName))
	np.Namespace = utils.GetOperatorNamespace()
	labels := utils.SpireServerLabels(customLabels)
	for k, v := range labels {
		np.Labels[k] = v
	}
	return np
}
