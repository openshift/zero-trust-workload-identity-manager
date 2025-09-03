package static_resource_controller

import (
	"context"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	customClient "github.com/openshift/zero-trust-workload-identity-manager/pkg/client"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

const (
	RBACResourcesGeneration                           = "RBACResourcesGeneration"
	ServiceResourcesGeneration                        = "ServiceResourcesGeneration"
	ServiceAccountResourcesGeneration                 = "ServiceAccountResourcesGeneration"
	SpiffeCSIResourcesGeneration                      = "SpiffeCSIResourcesGeneration"
	ValidatingWebhookConfigurationResourcesGeneration = "ValidatingWebhookConfigurationResourcesGeneration"
)

type StaticResourceReconciler struct {
	ctrlClient     customClient.CustomCtrlClient
	ctx            context.Context
	eventRecorder  record.EventRecorder
	log            logr.Logger
	scheme         *runtime.Scheme
	createOnlyMode bool
}

type reconcilerStatus struct {
	Status  metav1.ConditionStatus
	Message string
	Reason  string
}

// +kubebuilder:rbac:groups=operator.openshift.io,resources=zerotrustworkloadidentitymanagers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.openshift.io,resources=zerotrustworkloadidentitymanagers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.openshift.io,resources=zerotrustworkloadidentitymanagers/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spiffecsidrivers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spiffecsidrivers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spiffecsidrivers/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireagents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireagents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireagents/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireoidcdiscoveryproviders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireoidcdiscoveryproviders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireoidcdiscoveryproviders/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes/proxy,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterfederatedtrustdomains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterfederatedtrustdomains/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterfederatedtrustdomains/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterspiffeids,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterspiffeids/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterspiffeids/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterstaticentries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterstaticentries/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterstaticentries/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete

// New returns a new Reconciler instance.
func New(mgr ctrl.Manager) (*StaticResourceReconciler, error) {
	c, err := customClient.NewCustomClient(mgr)
	if err != nil {
		return nil, err
	}
	return &StaticResourceReconciler{
		ctrlClient:     c,
		ctx:            context.Background(),
		eventRecorder:  mgr.GetEventRecorderFor(utils.ZeroTrustWorkloadIdentityManagerStaticResourceControllerName),
		log:            ctrl.Log.WithName(utils.ZeroTrustWorkloadIdentityManagerStaticResourceControllerName),
		scheme:         mgr.GetScheme(),
		createOnlyMode: false,
	}, nil
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

func hasControllerManagedLabel(obj client.Object) bool {
	val, ok := obj.GetLabels()[utils.AppManagedByLabelKey]
	return ok && val == utils.AppManagedByLabelValue
}

// Reconcile function to checks for the ZeroTrustWorkloadIdentityManager and creates the static resources required for
// the operands to be used, and reflect the reconciliation status on the ZeroTrustWorkloadIdentityManager CR.
func (r *StaticResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var config v1alpha1.ZeroTrustWorkloadIdentityManager
	err := r.ctrlClient.Get(ctx, req.NamespacedName, &config)
	if err != nil {
		if errors.IsNotFound(err) {
			// Ensure the 'cluster' instance always exists
			if req.Name == "cluster" {
				r.log.Info("Recreating ZeroTrustWorkloadIdentityManager 'cluster' as it was deleted")
				newConfig := &v1alpha1.ZeroTrustWorkloadIdentityManager{
					ObjectMeta: metav1.ObjectMeta{
						Name: req.Name,
					},
				}
				if err = r.ctrlClient.Create(ctx, newConfig); err != nil {
					r.log.Error(err, "failed to recreate ZeroTrustWorkloadIdentityManager 'cluster'")
					return ctrl.Result{}, err
				}
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	reconcileStatus := map[string]reconcilerStatus{}
	defer func(reconcileStatus map[string]reconcilerStatus) {
		originalStatus := config.Status.DeepCopy()
		if config.Status.ConditionalStatus.Conditions == nil {
			config.Status.ConditionalStatus = v1alpha1.ConditionalStatus{
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
			apimeta.SetStatusCondition(&config.Status.ConditionalStatus.Conditions, newCondition)
		}
		newConfig := config.DeepCopy()
		if !equality.Semantic.DeepEqual(originalStatus, &config.Status) {
			if err = r.ctrlClient.StatusUpdateWithRetry(ctx, newConfig); err != nil {
				r.log.Error(err, "failed to update status")
			}
		}
	}(reconcileStatus)

	createOnlyMode := utils.IsInCreateOnlyMode(&config, &r.createOnlyMode)
	if createOnlyMode {
		r.log.Info("Running in create-only mode - will create resources if they don't exist but skip updates")
		reconcileStatus[utils.CreateOnlyModeStatusType] = reconcilerStatus{
			Status:  metav1.ConditionTrue,
			Reason:  utils.CreateOnlyModeEnabled,
			Message: "Create-only mode is enabled via ztwim.openshift.io/create-only annotation",
		}
	} else {
		existingCondition := apimeta.FindStatusCondition(config.Status.ConditionalStatus.Conditions, utils.CreateOnlyModeStatusType)
		if existingCondition != nil && existingCondition.Status == metav1.ConditionTrue {
			reconcileStatus[utils.CreateOnlyModeStatusType] = reconcilerStatus{
				Status:  metav1.ConditionFalse,
				Reason:  utils.CreateOnlyModeDisabled,
				Message: "Create-only mode is disabled",
			}
		}
	}

	err = r.CreateOrApplyRbacResources(ctx, createOnlyMode)
	if err != nil {
		r.log.Error(err, "failed to create or apply rbac resources")
		r.eventRecorder.Event(&config, corev1.EventTypeWarning, "failed to create RBAC resources",
			err.Error())
		reconcileStatus[RBACResourcesGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "RBACResourceCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	reconcileStatus[RBACResourcesGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "RBACResourceCreated",
		Message: "All RBAC resources for operands created",
	}
	err = r.CreateOrApplyServiceAccountResources(ctx, createOnlyMode)
	if err != nil {
		r.log.Error(err, "failed to create or apply service accounts resources")
		r.eventRecorder.Event(&config, corev1.EventTypeWarning, "failed to create Service Account resources",
			err.Error())
		reconcileStatus[ServiceAccountResourcesGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "ServiceAccountResourceCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	reconcileStatus[ServiceAccountResourcesGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "ServiceAccountResourceCreated",
		Message: "Service Account resources for operands are created",
	}
	err = r.CreateOrApplyServiceResources(ctx, createOnlyMode)
	if err != nil {
		r.log.Error(err, "failed to create or apply services resources")
		r.eventRecorder.Event(&config, corev1.EventTypeWarning, "failed to create Service resources",
			err.Error())
		reconcileStatus[ServiceResourcesGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "ServiceResourceCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	reconcileStatus[ServiceResourcesGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "ServiceResourceCreated",
		Message: "All Service resource for operands are created",
	}
	err = r.CreateSpiffeCsiDriver(ctx)
	if err != nil {
		r.log.Error(err, "failed to create or apply spiffe csi driver resources")
		r.eventRecorder.Event(&config, corev1.EventTypeWarning, "failed to create CSI driver resources",
			err.Error())
		reconcileStatus[SpiffeCSIResourcesGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpiffeCSIResourceCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	reconcileStatus[SpiffeCSIResourcesGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "SpiffeCSIResourceCreated",
		Message: "CSI driver resource created",
	}
	err = r.ApplyOrCreateValidatingWebhookConfiguration(ctx)
	if err != nil {
		r.log.Error(err, "failed to create or apply validating webhook configuration resources")
		r.eventRecorder.Event(&config, corev1.EventTypeWarning, "Failed to create validating webhook configuration resource",
			err.Error())
		reconcileStatus[ValidatingWebhookConfigurationResourcesGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "ValidatingWebhookConfigurationResourcesCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	reconcileStatus[ValidatingWebhookConfigurationResourcesGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "ValidatingWebhookConfigurationResourcesCreated",
		Message: "All ValidatingWebhookConfiguration resources for operands are created",
	}
	return ctrl.Result{}, nil
}

func (r *StaticResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {

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
		For(&v1alpha1.ZeroTrustWorkloadIdentityManager{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named(utils.ZeroTrustWorkloadIdentityManagerStaticResourceControllerName).
		Watches(&corev1.ServiceAccount{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&rbacv1.Role{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&rbacv1.RoleBinding{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&rbacv1.ClusterRole{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&rbacv1.ClusterRoleBinding{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&storagev1.CSIDriver{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&admissionregistrationv1.ValidatingWebhookConfiguration{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Complete(r)
	if err != nil {
		return err
	}
	return nil
}
