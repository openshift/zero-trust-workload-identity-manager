package static_resource_controller

import (
	"context"
	corev1 "k8s.io/api/core/v1"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/version"
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
	spireOIDCDiscoveryProviderService.Labels = utils.SetLabel(spireOIDCDiscoveryProviderService.Labels, "app.kubernetes.io/version", version.SpireOIDCDiscoveryProviderVersion)
	return spireOIDCDiscoveryProviderService
}

func (r *StaticResourceReconciler) getSpireServerService() *corev1.Service {
	spireServerService := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireServerServiceAssetName))
	spireServerService.Labels = utils.SetLabel(spireServerService.Labels, "app.kubernetes.io/version", version.SpireServerVersion)
	return spireServerService
}

func (r *StaticResourceReconciler) getSpireControllerMangerWebhookService() *corev1.Service {
	spireControllerMangerWebhookService := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireControllerMangerWebhookServiceAssetName))
	spireControllerMangerWebhookService.Labels = utils.SetLabel(spireControllerMangerWebhookService.Labels, "app.kubernetes.io/version", version.SpireControllerManagerVersion)
	return spireControllerMangerWebhookService
}

func (r *StaticResourceReconciler) getSpireAgentService() *corev1.Service {
	spireAgentService := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireAgentServiceAssetName))
	spireAgentService.Labels = utils.SetLabel(spireAgentService.Labels, "app.kubernetes.io/version", version.SpireAgentVersion)
	return spireAgentService
}
