package spiffe_helper

import (
	"context"
	"fmt"

	"os"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	customClient "github.com/openshift/zero-trust-workload-identity-manager/pkg/client"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

const (
	MutatingWebhookAvailable   = "MutatingWebhookAvailable"
	WebhookServiceAvailable    = "WebhookServiceAvailable"
	WebhookDeploymentAvailable = "WebhookDeploymentAvailable"
)

// SpiffeHelperReconciler reconciles webhook resources for the spiffe-helper sidecar injector
type SpiffeHelperReconciler struct {
	ctrlClient    customClient.CustomCtrlClient
	ctx           context.Context
	eventRecorder record.EventRecorder
	log           logr.Logger
	scheme        *runtime.Scheme
}

// New returns a new SpiffeHelperReconciler instance
func New(mgr ctrl.Manager) (*SpiffeHelperReconciler, error) {
	c, err := customClient.NewCustomClient(mgr)
	if err != nil {
		return nil, err
	}
	return &SpiffeHelperReconciler{
		ctrlClient:    c,
		ctx:           context.Background(),
		eventRecorder: mgr.GetEventRecorderFor(utils.ZeroTrustWorkloadIdentityManagerSpiffeHelperControllerName),
		log:           ctrl.Log.WithName(utils.ZeroTrustWorkloadIdentityManagerSpiffeHelperControllerName),
		scheme:        mgr.GetScheme(),
	}, nil
}

func (r *SpiffeHelperReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Info(fmt.Sprintf("reconciling %s", utils.ZeroTrustWorkloadIdentityManagerSpiffeHelperControllerName))

	var csiDriver v1alpha1.SpiffeCSIDriver
	if err := r.ctrlClient.Get(ctx, req.NamespacedName, &csiDriver); err != nil {
		if kerrors.IsNotFound(err) {
			r.log.Info("SpiffeCSIDriver resource not found. Ignoring since object must be deleted or not been created.")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	statusMgr := status.NewManager(r.ctrlClient)
	defer func() {
		if err := statusMgr.ApplyStatus(ctx, &csiDriver, func() *v1alpha1.ConditionalStatus {
			return &csiDriver.Status.ConditionalStatus
		}); err != nil {
			r.log.Error(err, "failed to update status")
		}
	}()

	createOnlyMode := utils.IsInCreateOnlyMode()

	// Reconcile MutatingWebhookConfiguration
	if err := r.reconcileMutatingWebhook(ctx, &csiDriver, statusMgr, createOnlyMode); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile webhook Service
	if err := r.reconcileWebhookService(ctx, &csiDriver, statusMgr, createOnlyMode); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile webhook Deployment
	if err := r.reconcileWebhookDeployment(ctx, &csiDriver, statusMgr, createOnlyMode); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *SpiffeHelperReconciler) SetupWithManager(mgr ctrl.Manager) error {
	mapFunc := func(ctx context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name: "cluster",
				},
			},
		}
	}

	controllerManagedResourcePredicates := builder.WithPredicates(utils.ControllerManagedResourcesForComponent(utils.ComponentSidecarInjector))

	err := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.SpiffeCSIDriver{}, builder.WithPredicates(utils.GenerationOrOwnerReferenceChangedPredicate)).
		Named(utils.ZeroTrustWorkloadIdentityManagerSpiffeHelperControllerName).
		Watches(&admissionregistrationv1.MutatingWebhookConfiguration{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&appsv1.Deployment{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Complete(r)
	if err != nil {
		return err
	}
	return nil
}

// reconcileMutatingWebhook reconciles the MutatingWebhookConfiguration
func (r *SpiffeHelperReconciler) reconcileMutatingWebhook(ctx context.Context, csiDriver *v1alpha1.SpiffeCSIDriver, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpiffeHelperMutatingWebhookConfiguration(csiDriver.Spec.Labels)

	if err := controllerutil.SetControllerReference(csiDriver, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on mutating webhook")
		statusMgr.AddCondition(MutatingWebhookAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on MutatingWebhookConfiguration: %v", err),
			metav1.ConditionFalse)
		return err
	}

	existing := &admissionregistrationv1.MutatingWebhookConfiguration{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			r.log.Error(err, "failed to get mutating webhook")
			statusMgr.AddCondition(MutatingWebhookAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get MutatingWebhookConfiguration: %v", err),
				metav1.ConditionFalse)
			return err
		}

		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create mutating webhook")
			statusMgr.AddCondition(MutatingWebhookAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create MutatingWebhookConfiguration: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created MutatingWebhookConfiguration", "name", desired.Name)
		statusMgr.AddCondition(MutatingWebhookAvailable, v1alpha1.ReasonReady,
			"MutatingWebhookConfiguration available",
			metav1.ConditionTrue)
		return nil
	}

	if createOnlyMode {
		r.log.V(1).Info("MutatingWebhookConfiguration exists, skipping update due to create-only mode", "name", desired.Name)
		statusMgr.AddCondition(MutatingWebhookAvailable, v1alpha1.ReasonReady,
			"MutatingWebhookConfiguration available",
			metav1.ConditionTrue)
		return nil
	}

	// Preserve externally managed fields
	desired.ResourceVersion = existing.ResourceVersion

	// Build lookup by webhook name for safe matching
	existingByName := make(map[string]admissionregistrationv1.MutatingWebhook, len(existing.Webhooks))
	for _, wh := range existing.Webhooks {
		existingByName[wh.Name] = wh
	}
	for i := range desired.Webhooks {
		existingWH, ok := existingByName[desired.Webhooks[i].Name]
		if !ok {
			continue
		}
		// Preserve caBundle — injected by service-ca-operator
		if len(existingWH.ClientConfig.CABundle) > 0 {
			desired.Webhooks[i].ClientConfig.CABundle = existingWH.ClientConfig.CABundle
		}
		// Preserve service port — only when desired doesn't explicitly set one
		if existingWH.ClientConfig.Service != nil &&
			desired.Webhooks[i].ClientConfig.Service != nil &&
			desired.Webhooks[i].ClientConfig.Service.Port == nil &&
			existingWH.ClientConfig.Service.Port != nil {
			desired.Webhooks[i].ClientConfig.Service.Port = existingWH.ClientConfig.Service.Port
		}
	}

	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("MutatingWebhookConfiguration is up to date", "name", desired.Name)
		statusMgr.AddCondition(MutatingWebhookAvailable, v1alpha1.ReasonReady,
			"MutatingWebhookConfiguration available",
			metav1.ConditionTrue)
		return nil
	}

	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update mutating webhook")
		statusMgr.AddCondition(MutatingWebhookAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update MutatingWebhookConfiguration: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated MutatingWebhookConfiguration", "name", desired.Name)
	statusMgr.AddCondition(MutatingWebhookAvailable, v1alpha1.ReasonReady,
		"MutatingWebhookConfiguration available",
		metav1.ConditionTrue)
	return nil
}

// reconcileWebhookService reconciles the Service for the webhook
func (r *SpiffeHelperReconciler) reconcileWebhookService(ctx context.Context, csiDriver *v1alpha1.SpiffeCSIDriver, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpiffeHelperWebhookService(csiDriver.Spec.Labels)

	if err := controllerutil.SetControllerReference(csiDriver, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on webhook service")
		statusMgr.AddCondition(WebhookServiceAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on webhook Service: %v", err),
			metav1.ConditionFalse)
		return err
	}

	existing := &corev1.Service{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			r.log.Error(err, "failed to get webhook service")
			statusMgr.AddCondition(WebhookServiceAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get webhook Service: %v", err),
				metav1.ConditionFalse)
			return err
		}

		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create webhook service")
			statusMgr.AddCondition(WebhookServiceAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create webhook Service: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created webhook Service", "name", desired.Name, "namespace", desired.Namespace)
		statusMgr.AddCondition(WebhookServiceAvailable, v1alpha1.ReasonReady,
			"Webhook Service available",
			metav1.ConditionTrue)
		return nil
	}

	if createOnlyMode {
		r.log.V(1).Info("Webhook Service exists, skipping update due to create-only mode", "name", desired.Name)
		statusMgr.AddCondition(WebhookServiceAvailable, v1alpha1.ReasonReady,
			"Webhook Service available",
			metav1.ConditionTrue)
		return nil
	}

	// Preserve Kubernetes-managed fields
	desired.ResourceVersion = existing.ResourceVersion
	desired.Spec.ClusterIP = existing.Spec.ClusterIP
	desired.Spec.ClusterIPs = existing.Spec.ClusterIPs
	desired.Spec.IPFamilies = existing.Spec.IPFamilies
	desired.Spec.IPFamilyPolicy = existing.Spec.IPFamilyPolicy
	desired.Spec.InternalTrafficPolicy = existing.Spec.InternalTrafficPolicy
	desired.Spec.SessionAffinity = existing.Spec.SessionAffinity
	if existing.Spec.HealthCheckNodePort != 0 {
		desired.Spec.HealthCheckNodePort = existing.Spec.HealthCheckNodePort
	}

	for i := range desired.Spec.Ports {
		if desired.Spec.Ports[i].Protocol == "" {
			desired.Spec.Ports[i].Protocol = corev1.ProtocolTCP
		}
	}

	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("Webhook Service is up to date", "name", desired.Name)
		statusMgr.AddCondition(WebhookServiceAvailable, v1alpha1.ReasonReady,
			"Webhook Service available",
			metav1.ConditionTrue)
		return nil
	}

	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update webhook service")
		statusMgr.AddCondition(WebhookServiceAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update webhook Service: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated webhook Service", "name", desired.Name, "namespace", desired.Namespace)
	statusMgr.AddCondition(WebhookServiceAvailable, v1alpha1.ReasonReady,
		"Webhook Service available",
		metav1.ConditionTrue)
	return nil
}

// getSpiffeHelperMutatingWebhookConfiguration returns the MutatingWebhookConfiguration with proper labels
func getSpiffeHelperMutatingWebhookConfiguration(customLabels map[string]string) *admissionregistrationv1.MutatingWebhookConfiguration {
	webhook := utils.DecodeMutatingWebhookConfigurationByBytes(assets.MustAsset(utils.SpiffeHelperMutatingWebhookConfigurationAssetName))
	webhook.Labels = utils.SpiffeHelperLabels(customLabels)
	for i := range webhook.Webhooks {
		if webhook.Webhooks[i].ClientConfig.Service != nil {
			webhook.Webhooks[i].ClientConfig.Service.Namespace = utils.GetOperatorNamespace()
		}
	}
	return webhook
}

// getSpiffeHelperWebhookService returns the webhook Service with proper labels
func getSpiffeHelperWebhookService(customLabels map[string]string) *corev1.Service {
	svc := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpiffeHelperWebhookServiceAssetName))
	svc.Labels = utils.SpiffeHelperLabels(customLabels)
	svc.Namespace = utils.GetOperatorNamespace()
	return svc
}

// reconcileWebhookDeployment reconciles the Deployment for the webhook server
func (r *SpiffeHelperReconciler) reconcileWebhookDeployment(ctx context.Context, csiDriver *v1alpha1.SpiffeCSIDriver, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpiffeHelperWebhookDeployment(csiDriver.Spec.Labels)

	if err := controllerutil.SetControllerReference(csiDriver, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on webhook deployment")
		statusMgr.AddCondition(WebhookDeploymentAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on webhook Deployment: %v", err),
			metav1.ConditionFalse)
		return err
	}

	existing := &appsv1.Deployment{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			r.log.Error(err, "failed to get webhook deployment")
			statusMgr.AddCondition(WebhookDeploymentAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get webhook Deployment: %v", err),
				metav1.ConditionFalse)
			return err
		}

		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create webhook deployment")
			statusMgr.AddCondition(WebhookDeploymentAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create webhook Deployment: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created webhook Deployment", "name", desired.Name, "namespace", desired.Namespace)
		statusMgr.AddCondition(WebhookDeploymentAvailable, v1alpha1.ReasonReady,
			"Webhook Deployment available",
			metav1.ConditionTrue)
		return nil
	}

	if createOnlyMode {
		r.log.V(1).Info("Webhook Deployment exists, skipping update due to create-only mode", "name", desired.Name)
		statusMgr.AddCondition(WebhookDeploymentAvailable, v1alpha1.ReasonReady,
			"Webhook Deployment available",
			metav1.ConditionTrue)
		return nil
	}

	desired.ResourceVersion = existing.ResourceVersion
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("Webhook Deployment is up to date", "name", desired.Name)
		statusMgr.AddCondition(WebhookDeploymentAvailable, v1alpha1.ReasonReady,
			"Webhook Deployment available",
			metav1.ConditionTrue)
		return nil
	}

	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update webhook deployment")
		statusMgr.AddCondition(WebhookDeploymentAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update webhook Deployment: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated webhook Deployment", "name", desired.Name, "namespace", desired.Namespace)
	statusMgr.AddCondition(WebhookDeploymentAvailable, v1alpha1.ReasonReady,
		"Webhook Deployment available",
		metav1.ConditionTrue)
	return nil
}

// getSpiffeHelperWebhookDeployment returns the webhook Deployment with proper labels and image
func getSpiffeHelperWebhookDeployment(customLabels map[string]string) *appsv1.Deployment {
	deployment := utils.DecodeDeploymentObjBytes(assets.MustAsset(utils.SpiffeHelperWebhookDeploymentAssetName))
	deployment.Labels = utils.SpiffeHelperLabels(customLabels)
	deployment.Namespace = utils.GetOperatorNamespace()

	// Set the webhook server image from RELATED_IMAGE env var
	webhookImage := os.Getenv(utils.SpiffeHelperWebhookImageEnv)
	if webhookImage != "" {
		for i := range deployment.Spec.Template.Spec.Containers {
			if deployment.Spec.Template.Spec.Containers[i].Name == "webhook" {
				deployment.Spec.Template.Spec.Containers[i].Image = webhookImage
			}
		}
	}

	// Set RELATED_IMAGE_SPIFFE_HELPER from operator env
	spiffeHelperImage := os.Getenv(utils.SpiffeHelperImageEnv)
	if spiffeHelperImage != "" {
		for i := range deployment.Spec.Template.Spec.Containers {
			for j := range deployment.Spec.Template.Spec.Containers[i].Env {
				if deployment.Spec.Template.Spec.Containers[i].Env[j].Name == utils.SpiffeHelperImageEnv {
					deployment.Spec.Template.Spec.Containers[i].Env[j].Value = spiffeHelperImage
				}
			}
		}
	}

	return deployment
}
