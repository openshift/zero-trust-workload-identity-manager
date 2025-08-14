package static_resource_controller

import (
	"context"
	corev1 "k8s.io/api/core/v1"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

func (r *StaticResourceReconciler) CreateOrApplyServiceAccountResources(ctx context.Context) error {
	for _, serviceAccount := range r.listStaticServiceAccount() {
		err := r.ctrlClient.CreateOrUpdateObject(ctx, serviceAccount)
		if err != nil {
			r.log.Error(err, "unable to create or update Service resource")
			return err
		}
	}
	return nil
}

func (r *StaticResourceReconciler) listStaticServiceAccount() []*corev1.ServiceAccount {
	serviceAccounts := []*corev1.ServiceAccount{}
	serviceAccounts = append(serviceAccounts, r.getSpiffeCsiDriverServiceAccount(), r.getSpireAgentServiceAccount(),
		r.getSpireOIDCDiscoveryProviderServiceAccount(), r.getSpireServerServiceAccount())
	return serviceAccounts

}

func (r *StaticResourceReconciler) getSpiffeCsiDriverServiceAccount() *corev1.ServiceAccount {
	spiffeCsiDriverServiceAccount := utils.DecodeServiceAccountObjBytes(assets.MustAsset(utils.SpiffeCsiDriverServiceAccountAssetName))
	spiffeCsiDriverServiceAccount.Labels = utils.SpiffeCSIDriverLabels(spiffeCsiDriverServiceAccount.Labels)
	return spiffeCsiDriverServiceAccount
}

func (r *StaticResourceReconciler) getSpireAgentServiceAccount() *corev1.ServiceAccount {
	spireAgentServiceAccount := utils.DecodeServiceAccountObjBytes(assets.MustAsset(utils.SpireAgentServiceAccountAssetName))
	spireAgentServiceAccount.Labels = utils.SpireAgentLabels(spireAgentServiceAccount.Labels)
	return spireAgentServiceAccount
}

func (r *StaticResourceReconciler) getSpireServerServiceAccount() *corev1.ServiceAccount {
	spireSeverServiceAccount := utils.DecodeServiceAccountObjBytes(assets.MustAsset(utils.SpireServerServiceAccountAssetName))
	spireSeverServiceAccount.Labels = utils.SpireServerLabels(spireSeverServiceAccount.Labels)
	return spireSeverServiceAccount
}

func (r *StaticResourceReconciler) getSpireOIDCDiscoveryProviderServiceAccount() *corev1.ServiceAccount {
	spireOIDCDiscoveryProviderServiceAccount := utils.DecodeServiceAccountObjBytes(assets.MustAsset(utils.SpireOIDCDiscoveryProviderServiceAccountAssetName))
	spireOIDCDiscoveryProviderServiceAccount.Labels = utils.SpireOIDCDiscoveryProviderLabels(spireOIDCDiscoveryProviderServiceAccount.Labels)
	return spireOIDCDiscoveryProviderServiceAccount
}
