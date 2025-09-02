package static_resource_controller

import (
	"context"
	corev1 "k8s.io/api/core/v1"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

func (r *StaticResourceReconciler) CreateOrApplyServiceResources(ctx context.Context, createOnlyMode bool) error {
	for _, service := range r.listStaticServiceResource() {
		err := r.createOrUpdateResource(ctx, service, createOnlyMode, "Service")
		if err != nil {
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
	spireOIDCDiscoveryProviderService.Labels = utils.SpireOIDCDiscoveryProviderLabels(spireOIDCDiscoveryProviderService.Labels)
	// Update selector to match standardized labels
	spireOIDCDiscoveryProviderService.Spec.Selector = map[string]string{
		"app.kubernetes.io/name":     "spiffe-oidc-discovery-provider",
		"app.kubernetes.io/instance": utils.StandardInstance,
	}
	return spireOIDCDiscoveryProviderService
}

func (r *StaticResourceReconciler) getSpireServerService() *corev1.Service {
	spireServerService := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireServerServiceAssetName))
	spireServerService.Labels = utils.SpireServerLabels(spireServerService.Labels)
	// Update selector to match standardized labels
	spireServerService.Spec.Selector = map[string]string{
		"app.kubernetes.io/name":     "spire-server",
		"app.kubernetes.io/instance": utils.StandardInstance,
	}
	return spireServerService
}

func (r *StaticResourceReconciler) getSpireControllerMangerWebhookService() *corev1.Service {
	spireControllerMangerWebhookService := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireControllerMangerWebhookServiceAssetName))
	spireControllerMangerWebhookService.Labels = utils.SpireControllerManagerLabels(spireControllerMangerWebhookService.Labels)
	// Update selector to match standardized labels
	spireControllerMangerWebhookService.Spec.Selector = map[string]string{
		"app.kubernetes.io/name":     "spire-controller-manager",
		"app.kubernetes.io/instance": utils.StandardInstance,
	}
	return spireControllerMangerWebhookService
}

func (r *StaticResourceReconciler) getSpireAgentService() *corev1.Service {
	spireAgentService := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireAgentServiceAssetName))
	spireAgentService.Labels = utils.SpireAgentLabels(spireAgentService.Labels)
	// Update selector to match standardized labels
	spireAgentService.Spec.Selector = map[string]string{
		"app.kubernetes.io/name":     "spire-agent",
		"app.kubernetes.io/instance": utils.StandardInstance,
	}
	return spireAgentService
}
