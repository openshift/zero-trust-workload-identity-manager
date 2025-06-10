package static_resource_controller

import (
	"context"
	corev1 "k8s.io/api/core/v1"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

func (r *StaticResourceReconciler) CreateOrApplyServiceResources(ctx context.Context) error {
	for _, service := range r.listStaticServiceResource() {
		err := r.ctrlClient.CreateOrUpdateObject(ctx, service)
		if err != nil {
			r.log.Error(err, "unable to create or update Service resource")
			return err
		}
	}
	return nil
}

func (r *StaticResourceReconciler) listStaticServiceResource() []*corev1.Service {
	staticServices := []*corev1.Service{}
	staticServices = append(staticServices, r.getSpireServerService(), r.getSpireAgentService(), r.getSpireOIDCDiscoveryProviderService(), r.getSpireControllerMangerWebhookService())
	return staticServices
}

func (r *StaticResourceReconciler) getSpireOIDCDiscoveryProviderService() *corev1.Service {
	spireOIDCDiscoveryProviderService := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireOIDCDiscoveryProviderServiceAssetName))
	return spireOIDCDiscoveryProviderService
}

func (r *StaticResourceReconciler) getSpireServerService() *corev1.Service {
	spireServerService := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireServerServiceAssetName))
	return spireServerService
}

func (r *StaticResourceReconciler) getSpireControllerMangerWebhookService() *corev1.Service {
	spireControllerMangerWebhookService := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireControllerMangerWebhookServiceAssetName))
	return spireControllerMangerWebhookService
}

func (r *StaticResourceReconciler) getSpireAgentService() *corev1.Service {
	spireAgentService := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireAgentServiceAssetName))
	return spireAgentService
}
