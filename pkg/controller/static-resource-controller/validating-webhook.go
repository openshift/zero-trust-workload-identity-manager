package static_resource_controller

import (
	"context"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (r *StaticResourceReconciler) ApplyOrCreateValidatingWebhookConfiguration(ctx context.Context) error {
	desired := r.GetSpireControllerManagerValidatingWebhookConfiguration()
	err := r.ctrlClient.Create(ctx, desired)
	if err != nil && apierrors.IsAlreadyExists(err) {
		return nil
	}
	if err != nil {
		r.log.Error(err, "failed to create SpireControllerManager ValidatingWebhookConfiguration resources")
		return err
	}
	return nil
}

func (r *StaticResourceReconciler) GetSpireControllerManagerValidatingWebhookConfiguration() *admissionregistrationv1.ValidatingWebhookConfiguration {
	spireControllerManagerValidatingWebhookConfiguration := utils.DecodeValidatingWebhookConfigurationByBytes(assets.MustAsset(utils.SpireControllerManagerValidatingWebhookConfigurationAssetName))
	spireControllerManagerValidatingWebhookConfiguration.Labels = utils.SpireControllerManagerLabels(spireControllerManagerValidatingWebhookConfiguration.Labels)
	return spireControllerManagerValidatingWebhookConfiguration
}
