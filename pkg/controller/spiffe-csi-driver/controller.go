package spiffe_csi_driver

import (
	"context"
	"fmt"
	securityv1 "github.com/openshift/api/security/v1"
	customClient "github.com/openshift/zero-trust-workload-identity-manager/pkg/client"
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"

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
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

type reconcilerStatus struct {
	Status  metav1.ConditionStatus
	Message string
	Reason  string
}

const (
	SpiffeCSIDaemonSetGeneration = "SpiffeCSIDaemonSetGeneration"
	SpiffeCSISCCGeneration       = "SpiffeCSISCCGeneration"
)

// SpiffeCsiReconciler reconciles a SpiffeCsi object
type SpiffeCsiReconciler struct {
	ctrlClient    customClient.CustomCtrlClient
	ctx           context.Context
	eventRecorder record.EventRecorder
	log           logr.Logger
	scheme        *runtime.Scheme
}

// New returns a new Reconciler instance.
func New(mgr ctrl.Manager) (*SpiffeCsiReconciler, error) {
	c, err := customClient.NewCustomClient(mgr)
	if err != nil {
		return nil, err
	}
	return &SpiffeCsiReconciler{
		ctrlClient:    c,
		ctx:           context.Background(),
		eventRecorder: mgr.GetEventRecorderFor(utils.ZeroTrustWorkloadIdentityManagerSpiffeCsiDriverControllerName),
		log:           ctrl.Log.WithName(utils.ZeroTrustWorkloadIdentityManagerSpiffeCsiDriverControllerName),
		scheme:        mgr.GetScheme(),
	}, nil
}

func (r *SpiffeCsiReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var spiffeCSIDriver v1alpha1.SpiffeCSIDriverConfig
	if err := r.ctrlClient.Get(ctx, req.NamespacedName, &spiffeCSIDriver); err != nil {
		if kerrors.IsNotFound(err) {
			r.log.Info("SpiffeCsiConfig resource not found. Ignoring since object must be deleted or not been created.")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	reconcileStatus := map[string]reconcilerStatus{}
	defer func(reconcileStatus map[string]reconcilerStatus) {
		originalStatus := spiffeCSIDriver.Status.DeepCopy()
		if spiffeCSIDriver.Status.ConditionalStatus.Conditions == nil {
			spiffeCSIDriver.Status.ConditionalStatus = v1alpha1.ConditionalStatus{
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
			apimeta.SetStatusCondition(&spiffeCSIDriver.Status.ConditionalStatus.Conditions, newCondition)
		}
		newConfig := spiffeCSIDriver.DeepCopy()
		if !equality.Semantic.DeepEqual(originalStatus, &spiffeCSIDriver.Status) {
			if err := r.ctrlClient.StatusUpdateWithRetry(ctx, newConfig); err != nil {
				r.log.Error(err, "failed to update status")
			}
		}
	}(reconcileStatus)

	SpiffeCsiSCC := generateSpiffeCSIDriverSCC()
	if err := controllerutil.SetControllerReference(&spiffeCSIDriver, SpiffeCsiSCC, r.scheme); err != nil {
		r.log.Error(err, "failed to set the owner reference for the SCC resource")
		reconcileStatus[SpiffeCSISCCGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpiffeCSISCCGenerationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	err := r.ctrlClient.Create(ctx, SpiffeCsiSCC)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		r.log.Error(err, "Failed to create SpiffeCsiSCC")
		reconcileStatus[SpiffeCSISCCGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpiffeCSISCCGenerationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	reconcileStatus[SpiffeCSISCCGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "SpiffeCSISCCResourceCreated",
		Message: "SpiffeCSISCC resource created",
	}

	spiffeCsiDaemonset := generateSpiffeCsiDriverDaemonSet(spiffeCSIDriver.Spec)
	if err = controllerutil.SetControllerReference(&spiffeCSIDriver, spiffeCsiDaemonset, r.scheme); err != nil {
		r.log.Error(err, "failed to set owner reference for the SCC resource")
		reconcileStatus[SpiffeCSIDaemonSetGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpiffeCSIDaemonSetGenerationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}

	// Create or Update DaemonSet
	var existingSpiffeCsiDaemonSet appsv1.DaemonSet
	err = r.ctrlClient.Get(ctx, types.NamespacedName{Name: spiffeCsiDaemonset.Name, Namespace: spiffeCsiDaemonset.Namespace}, &existingSpiffeCsiDaemonSet)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, spiffeCsiDaemonset); err != nil {
			r.log.Error(err, "Failed to create SpiffeCsiDaemon set")
			reconcileStatus[SpiffeCSIDaemonSetGeneration] = reconcilerStatus{
				Status:  metav1.ConditionFalse,
				Reason:  "SpiffeCSIDaemonSetGenerationFailed",
				Message: err.Error(),
			}
			return ctrl.Result{}, fmt.Errorf("failed to create DaemonSet: %w", err)
		}
		r.log.Info("Created spiffe csi DaemonSet")
	} else if err == nil && needsUpdate(existingSpiffeCsiDaemonSet, *spiffeCsiDaemonset) {
		existingSpiffeCsiDaemonSet.Spec = spiffeCsiDaemonset.Spec
		if err = r.ctrlClient.Update(ctx, &existingSpiffeCsiDaemonSet); err != nil {
			r.log.Error(err, "failed to update spiffe csi daemon set")
			return ctrl.Result{}, fmt.Errorf("failed to update DaemonSet: %w", err)
		}
		r.log.Info("Updated spiffe csi DaemonSet")
	} else if err != nil {
		r.log.Error(err, "Failed to get SpiffeCsiDaemon set")
		reconcileStatus[SpiffeCSIDaemonSetGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpiffeCSIDaemonSetGenerationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	reconcileStatus[SpiffeCSIDaemonSetGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "SpiffeCSIDaemonSetCreated",
		Message: "Spiffe CSI DaemonSet resource created",
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

func (r *SpiffeCsiReconciler) SetupWithManager(mgr ctrl.Manager) error {
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
		For(&v1alpha1.SpiffeCSIDriverConfig{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named(utils.ZeroTrustWorkloadIdentityManagerSpiffeCsiDriverControllerName).
		Watches(&appsv1.DaemonSet{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&securityv1.SecurityContextConstraints{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Complete(r)
	if err != nil {
		return err
	}
	return nil
}

// needsUpdate returns true if DaemonSet needs to be updated.
func needsUpdate(current, desired appsv1.DaemonSet) bool {
	return utils.DaemonSetSpecModified(&desired, &current)
}
