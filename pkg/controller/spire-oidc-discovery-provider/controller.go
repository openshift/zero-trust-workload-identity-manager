package spire_oidc_discovery_provider

import (
	"context"

	"github.com/go-logr/logr"
	securityv1 "github.com/openshift/api/security/v1"
	customClient "github.com/openshift/zero-trust-workload-identity-manager/pkg/client"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/featuregate"
	"k8s.io/apimachinery/pkg/api/equality"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const spireOidcDeploymentSpireOidcConfigHashAnnotationKey = "ztwim.openshift.io/spire-oidc-discovery-provider-config-hash"

const (
	SpireOIDCDeploymentGeneration  = "SpireOIDCDeploymentGeneration"
	SpireOIDCConfigMapGeneration   = "SpireOIDCConfigMapGeneration"
	SpireOIDCSCCGeneration         = "SpireOIDCSCCGeneration"
	SpireClusterSpiffeIDGeneration = "SpireClusterSpiffeIDGeneration"
)

type reconcilerStatus struct {
	Status  metav1.ConditionStatus
	Message string
	Reason  string
}

// SpireOidcDiscoveryProviderReconciler reconciles a SpireOidcDiscoveryProvider object
type SpireOidcDiscoveryProviderReconciler struct {
	ctrlClient    customClient.CustomCtrlClient
	ctx           context.Context
	eventRecorder record.EventRecorder
	log           logr.Logger
	scheme        *runtime.Scheme
}

// New returns a new Reconciler instance.
func New(mgr ctrl.Manager) (*SpireOidcDiscoveryProviderReconciler, error) {
	c, err := customClient.NewCustomClient(mgr)
	if err != nil {
		return nil, err
	}
	return &SpireOidcDiscoveryProviderReconciler{
		ctrlClient:    c,
		ctx:           context.Background(),
		eventRecorder: mgr.GetEventRecorderFor(utils.ZeroTrustWorkloadIdentityManagerSpireOIDCDiscoveryProviderControllerName),
		log:           ctrl.Log.WithName(utils.ZeroTrustWorkloadIdentityManagerSpireOIDCDiscoveryProviderControllerName),
		scheme:        mgr.GetScheme(),
	}, nil
}

func (r *SpireOidcDiscoveryProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Info("Reconciling SpireOIDCDiscoveryProvider controller")

	var oidcDiscoveryProviderConfig v1alpha1.SpireOIDCDiscoveryProvider
	if err := r.ctrlClient.Get(ctx, req.NamespacedName, &oidcDiscoveryProviderConfig); err != nil {
		if kerrors.IsNotFound(err) {
			r.log.Info("SpireOidcDiscoveryProvider resource not found. Ignoring since object must be deleted or not been created.")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	reconcileStatus := map[string]reconcilerStatus{}
	defer func(reconcileStatus map[string]reconcilerStatus) {
		originalStatus := oidcDiscoveryProviderConfig.Status.DeepCopy()
		if oidcDiscoveryProviderConfig.Status.ConditionalStatus.Conditions == nil {
			oidcDiscoveryProviderConfig.Status.ConditionalStatus = v1alpha1.ConditionalStatus{
				Conditions: []metav1.Condition{},
			}
		}
		for key, value := range reconcileStatus {
			newCondition := metav1.Condition{
				Type:               key,
				Status:             value.Status,
				Reason:             value.Reason,
				Message:            value.Message,
				LastTransitionTime: metav1.Now(),
			}
			apimeta.SetStatusCondition(&oidcDiscoveryProviderConfig.Status.ConditionalStatus.Conditions, newCondition)
		}
		newConfig := oidcDiscoveryProviderConfig.DeepCopy()
		if !equality.Semantic.DeepEqual(originalStatus, &oidcDiscoveryProviderConfig.Status) {
			if err := r.ctrlClient.StatusUpdateWithRetry(ctx, newConfig); err != nil {
				r.log.Error(err, "failed to update status")
			}
		}
	}(reconcileStatus)

	if utils.IsAutoReconcileDisabled() {
		reconcileStatus[utils.FeatureGateStatusType] = reconcilerStatus{
			Status:  metav1.ConditionTrue,
			Reason:  utils.TechPreviewFeatureGateEnabled,
			Message: "FeatureGate is enabled",
		}
		r.log.Info("Auto-reconciliation disabled to allow manual management", "feature", featuregate.TechPreviewFeature)
		return ctrl.Result{}, nil
	} else {
		if utils.HasCondition(oidcDiscoveryProviderConfig.Status.ConditionalStatus.Conditions, utils.FeatureGateStatusType) {
			reconcileStatus[utils.FeatureGateStatusType] = reconcilerStatus{
				Status:  metav1.ConditionFalse,
				Reason:  utils.TechPreviewFeatureGateDisabled,
				Message: "FeatureGate is disabled",
			}
		}
	}

	if oidcDiscoveryProviderConfig.Spec.JwtIssuer == "" {
		oidcDiscoveryProviderConfig.Spec.JwtIssuer = "oidc-discovery." + oidcDiscoveryProviderConfig.Spec.TrustDomain
	}

	spireOIDCClusterSpiffeID := generateSpireIODCDiscoveryProviderSpiffeID()
	if err := controllerutil.SetControllerReference(&oidcDiscoveryProviderConfig, spireOIDCClusterSpiffeID, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		reconcileStatus[SpireClusterSpiffeIDGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireClusterSpiffeIDGenerationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	err := r.ctrlClient.Create(ctx, spireOIDCClusterSpiffeID)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		r.log.Error(err, "Failed to create oidc cluster spiffe id")
		reconcileStatus[SpireClusterSpiffeIDGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireClusterSpiffeIDCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	defaultSpiffeID := generateDefaultFallbackClusterSPIFFEID()
	if err = controllerutil.SetControllerReference(&oidcDiscoveryProviderConfig, defaultSpiffeID, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		reconcileStatus[SpireClusterSpiffeIDGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireClusterSpiffeIDCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	err = r.ctrlClient.Create(ctx, defaultSpiffeID)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		r.log.Error(err, "Failed to create DefaultFallbackClusterSPIFFEID")
		reconcileStatus[SpireClusterSpiffeIDGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireClusterSpiffeIDCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	reconcileStatus[SpireClusterSpiffeIDGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "SpireClusterSpiffeIDCreationSucceeded",
		Message: "Spire OIDC and default ClusterSpiffeID created successfully",
	}
	scc := generateSpireOIDCDiscoveryProviderSCC(&oidcDiscoveryProviderConfig)
	if err = controllerutil.SetControllerReference(&oidcDiscoveryProviderConfig, scc, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		reconcileStatus[SpireOIDCSCCGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireOIDCSCCGenerationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	err = r.ctrlClient.Create(ctx, scc)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		reconcileStatus[SpireOIDCSCCGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireOIDCSCCCreationFailed",
			Message: err.Error(),
		}
		r.log.Error(err, "Failed to create spire oidc discovery provider SCC")
		return ctrl.Result{}, err
	}
	reconcileStatus[SpireOIDCSCCGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "SpireOIDCSCCCreationSucceeded",
		Message: "Spire OIDC SCC created",
	}

	cm, err := GenerateOIDCConfigMapFromCR(&oidcDiscoveryProviderConfig)
	if err != nil {
		r.log.Error(err, "failed to generate OIDC ConfigMap from CR")
		reconcileStatus[SpireOIDCConfigMapGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireOIDCConfigMapCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	if err = controllerutil.SetControllerReference(&oidcDiscoveryProviderConfig, cm, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		reconcileStatus[SpireOIDCConfigMapGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireOIDCConfigMapCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	var existingOidcCm corev1.ConfigMap
	err = r.ctrlClient.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, &existingOidcCm)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, cm); err != nil {
			r.log.Error(err, "Failed to create ConfigMap")
			reconcileStatus[SpireOIDCConfigMapGeneration] = reconcilerStatus{
				Status:  metav1.ConditionFalse,
				Reason:  "SpireOIDCConfigMapCreationFailed",
				Message: err.Error(),
			}
			return ctrl.Result{}, err
		}
		r.log.Info("Created ConfigMap", "Namespace", cm.Namespace, "Name", cm.Name)
	} else if utils.GenerateMapHash(existingOidcCm.Data) != utils.GenerateMapHash(cm.Data) {
		existingOidcCm.Data = cm.Data
		if err = r.ctrlClient.Update(ctx, &existingOidcCm); err != nil {
			r.log.Error(err, "Failed to update ConfigMap", "Namespace", cm.Namespace, "Name", cm.Name)
			reconcileStatus[SpireOIDCConfigMapGeneration] = reconcilerStatus{
				Status:  metav1.ConditionFalse,
				Reason:  "SpireOIDCConfigMapCreationFailed",
				Message: err.Error(),
			}
			return ctrl.Result{}, err
		}
		r.log.Info("Updated ConfigMap", "Namespace", cm.Namespace, "Name", cm.Name)
	} else if err != nil {
		r.log.Error(err, "Failed to get ConfigMap")
		reconcileStatus[SpireOIDCConfigMapGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireOIDCConfigMapCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	reconcileStatus[SpireOIDCConfigMapGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "SpireOIDCConfigMapCreationSucceeded",
		Message: "Spire OIDC ConfigMap created",
	}

	configMapHash := utils.GenerateMapHash(cm.Data)
	deployment := buildDeployment(&oidcDiscoveryProviderConfig, configMapHash)
	if err = controllerutil.SetControllerReference(&oidcDiscoveryProviderConfig, deployment, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		reconcileStatus[SpireOIDCDeploymentGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireOIDCDeploymentCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	var existingSpireOidcDeployment appsv1.Deployment
	err = r.ctrlClient.Get(ctx, types.NamespacedName{
		Name:      deployment.Name,
		Namespace: deployment.Namespace,
	}, &existingSpireOidcDeployment)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, deployment); err != nil {
			r.log.Error(err, "Failed to create spire oidc discovery provider deployment")
			reconcileStatus[SpireOIDCDeploymentGeneration] = reconcilerStatus{
				Status:  metav1.ConditionFalse,
				Reason:  "SpireOIDCDeploymentCreationFailed",
				Message: err.Error(),
			}
			return ctrl.Result{}, err
		}
		r.log.Info("Created spire oidc discovery provider deployment")
	} else if err == nil && needsUpdate(existingSpireOidcDeployment, *deployment) {
		existingSpireOidcDeployment.Spec = deployment.Spec
		if err = r.ctrlClient.Update(ctx, &existingSpireOidcDeployment); err != nil {
			r.log.Error(err, "Failed to update spire oidc discovery provider deployment")
			reconcileStatus[SpireOIDCDeploymentGeneration] = reconcilerStatus{
				Status:  metav1.ConditionFalse,
				Reason:  "SpireOIDCDeploymentCreationFailed",
				Message: err.Error(),
			}
			return ctrl.Result{}, err
		}
		r.log.Info("Updated spire oidc discovery provider deployment")
	} else if err != nil {
		r.log.Error(err, "Failed to get existing spire oidc discovery provider deployment")
		reconcileStatus[SpireOIDCDeploymentGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireOIDCDeploymentCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	reconcileStatus[SpireOIDCDeploymentGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "SpireOIDCDeploymentCreationSucceeded",
		Message: "Spire OIDC Deployment created",
	}
	return ctrl.Result{}, nil
}

func hasControllerManagedLabel(obj client.Object) bool {
	val, ok := obj.GetLabels()[utils.AppManagedByLabelKey]
	return ok && val == utils.AppManagedByLabelValue
}

// controllerManagedResources filters resources that have a specific label indicating they are managed
var controllerManagedResources = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		return hasControllerManagedLabel(e.ObjectNew)
	},
	CreateFunc: func(e event.CreateEvent) bool {
		return hasControllerManagedLabel(e.Object)
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return hasControllerManagedLabel(e.Object)
	},
}

func (r *SpireOidcDiscoveryProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Always enqueue the "cluster" CR for reconciliation
	mapFunc := func(ctx context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name: "cluster",
				},
			},
		}
	}

	controllerManagedResourcePredicates := builder.WithPredicates(controllerManagedResources)

	err := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.SpireOIDCDiscoveryProvider{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named(utils.ZeroTrustWorkloadIdentityManagerSpireOIDCDiscoveryProviderControllerName).
		Watches(&appsv1.Deployment{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&securityv1.SecurityContextConstraints{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Complete(r)
	if err != nil {
		return err
	}
	return nil
}

// needsUpdate returns true if Deployment needs to be updated based on config checksum
func needsUpdate(current, desired appsv1.Deployment) bool {
	if current.Spec.Template.Annotations[spireOidcDeploymentSpireOidcConfigHashAnnotationKey] != desired.Spec.Template.Annotations[spireOidcDeploymentSpireOidcConfigHashAnnotationKey] {
		return true
	} else if utils.DeploymentSpecModified(&desired, &current) {
		return true
	}
	return false
}
