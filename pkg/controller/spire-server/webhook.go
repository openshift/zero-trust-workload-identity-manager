package spire_server

import (
	"context"
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
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

// reconcileWebhook reconciles the ValidatingWebhookConfiguration for Controller Manager
func (r *SpireServerReconciler) reconcileWebhook(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpireControllerManagerValidatingWebhookConfiguration(server.Spec.Labels)

	if err := controllerutil.SetControllerReference(server, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on validating webhook")
		statusMgr.AddCondition(ValidatingWebhookAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on ValidatingWebhookConfiguration: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Get existing resource (from cache)
	existing := &admissionregistrationv1.ValidatingWebhookConfiguration{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get validating webhook")
			statusMgr.AddCondition(ValidatingWebhookAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get ValidatingWebhookConfiguration: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create validating webhook")
			statusMgr.AddCondition(ValidatingWebhookAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create ValidatingWebhookConfiguration: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created ValidatingWebhookConfiguration", "name", desired.Name)
		statusMgr.AddCondition(ValidatingWebhookAvailable, v1alpha1.ReasonReady,
			"All ValidatingWebhookConfiguration resources available",
			metav1.ConditionTrue)
		return nil
	}

	// Resource exists, check if we need to update
	if createOnlyMode {
		r.log.V(1).Info("ValidatingWebhookConfiguration exists, skipping update due to create-only mode", "name", desired.Name)
		statusMgr.AddCondition(ValidatingWebhookAvailable, v1alpha1.ReasonReady,
			"All ValidatingWebhookConfiguration resources available",
			metav1.ConditionTrue)
		return nil
	}

	// Preserve Kubernetes-managed fields from existing resource BEFORE comparison
	desired.ResourceVersion = existing.ResourceVersion

	// Preserve CABundle, matchPolicy, timeoutSeconds, scope, and selectors for each webhook
	for i := range desired.Webhooks {
		if i < len(existing.Webhooks) {
			// Preserve caBundle - typically injected by cert-manager or service-ca-operator
			if len(existing.Webhooks[i].ClientConfig.CABundle) > 0 {
				desired.Webhooks[i].ClientConfig.CABundle = existing.Webhooks[i].ClientConfig.CABundle
			}
			// Preserve Kubernetes defaults
			if existing.Webhooks[i].MatchPolicy != nil {
				desired.Webhooks[i].MatchPolicy = existing.Webhooks[i].MatchPolicy
			}
			if existing.Webhooks[i].TimeoutSeconds != nil {
				desired.Webhooks[i].TimeoutSeconds = existing.Webhooks[i].TimeoutSeconds
			}
			if existing.Webhooks[i].NamespaceSelector != nil {
				desired.Webhooks[i].NamespaceSelector = existing.Webhooks[i].NamespaceSelector
			}
			if existing.Webhooks[i].ObjectSelector != nil {
				desired.Webhooks[i].ObjectSelector = existing.Webhooks[i].ObjectSelector
			}
			// Preserve scope and port in rules and service
			for j := range desired.Webhooks[i].Rules {
				if j < len(existing.Webhooks[i].Rules) && existing.Webhooks[i].Rules[j].Scope != nil {
					desired.Webhooks[i].Rules[j].Scope = existing.Webhooks[i].Rules[j].Scope
				}
			}
			if existing.Webhooks[i].ClientConfig.Service != nil && desired.Webhooks[i].ClientConfig.Service != nil {
				if existing.Webhooks[i].ClientConfig.Service.Port != nil {
					desired.Webhooks[i].ClientConfig.Service.Port = existing.Webhooks[i].ClientConfig.Service.Port
				}
			}
		}
	}

	// Check if update is needed
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("ValidatingWebhookConfiguration is up to date", "name", desired.Name)
		statusMgr.AddCondition(ValidatingWebhookAvailable, v1alpha1.ReasonReady,
			"All ValidatingWebhookConfiguration resources available",
			metav1.ConditionTrue)
		return nil
	}

	// Update the resource
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update validating webhook")
		statusMgr.AddCondition(ValidatingWebhookAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update ValidatingWebhookConfiguration: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated ValidatingWebhookConfiguration", "name", desired.Name)
	statusMgr.AddCondition(ValidatingWebhookAvailable, v1alpha1.ReasonReady,
		"All ValidatingWebhookConfiguration resources available",
		metav1.ConditionTrue)
	return nil
}

// getSpireControllerManagerValidatingWebhookConfiguration returns the ValidatingWebhookConfiguration with proper labels
func getSpireControllerManagerValidatingWebhookConfiguration(customLabels map[string]string) *admissionregistrationv1.ValidatingWebhookConfiguration {
	webhook := utils.DecodeValidatingWebhookConfigurationByBytes(assets.MustAsset(utils.SpireControllerManagerValidatingWebhookConfigurationAssetName))
	webhook.Labels = utils.SpireControllerManagerLabels(customLabels)
	// Update webhook service namespaces dynamically
	for i := range webhook.Webhooks {
		if webhook.Webhooks[i].ClientConfig.Service != nil {
			webhook.Webhooks[i].ClientConfig.Service.Namespace = utils.GetOperatorNamespace()
		}
	}
	return webhook
}
