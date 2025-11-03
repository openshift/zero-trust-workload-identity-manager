package spiffe_csi_driver

import (
	"context"
	"fmt"
	"reflect"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	customClient "github.com/openshift/zero-trust-workload-identity-manager/pkg/client"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

const (
	DaemonSetAvailable                  = "DaemonSetAvailable"
	SecurityContextConstraintsAvailable = "SecurityContextConstraintsAvailable"
	ServiceAccountAvailable             = "ServiceAccountAvailable"
	CSIDriverAvailable                  = "CSIDriverAvailable"
)

// SpiffeCsiReconciler reconciles a SpiffeCsi object
type SpiffeCsiReconciler struct {
	ctrlClient     customClient.CustomCtrlClient
	ctx            context.Context
	eventRecorder  record.EventRecorder
	log            logr.Logger
	scheme         *runtime.Scheme
	createOnlyMode bool
}

// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch;create;update;patch;delete

// New returns a new Reconciler instance.
func New(mgr ctrl.Manager) (*SpiffeCsiReconciler, error) {
	c, err := customClient.NewCustomClient(mgr)
	if err != nil {
		return nil, err
	}
	return &SpiffeCsiReconciler{
		ctrlClient:     c,
		ctx:            context.Background(),
		eventRecorder:  mgr.GetEventRecorderFor(utils.ZeroTrustWorkloadIdentityManagerSpiffeCsiDriverControllerName),
		log:            ctrl.Log.WithName(utils.ZeroTrustWorkloadIdentityManagerSpiffeCsiDriverControllerName),
		scheme:         mgr.GetScheme(),
		createOnlyMode: false,
	}, nil
}

func (r *SpiffeCsiReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var spiffeCSIDriver v1alpha1.SpiffeCSIDriver
	if err := r.ctrlClient.Get(ctx, req.NamespacedName, &spiffeCSIDriver); err != nil {
		if kerrors.IsNotFound(err) {
			r.log.Info("SpiffeCsiDriver resource not found. Ignoring since object must be deleted or not been created.")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	statusMgr := status.NewManager(r.ctrlClient)
	defer func() {
		if err := statusMgr.ApplyStatus(ctx, &spiffeCSIDriver, func() *v1alpha1.ConditionalStatus {
			return &spiffeCSIDriver.Status.ConditionalStatus
		}); err != nil {
			r.log.Error(err, "failed to update status")
		}
	}()

	// Handle create-only mode
	createOnlyMode := r.handleCreateOnlyMode(&spiffeCSIDriver, statusMgr)

	// Reconcile static resources (ServiceAccount, CSI Driver)
	if err := r.reconcileServiceAccount(ctx, &spiffeCSIDriver, statusMgr, createOnlyMode); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileCSIDriver(ctx, &spiffeCSIDriver, statusMgr, createOnlyMode); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile SCC
	if err := r.reconcileSCC(ctx, &spiffeCSIDriver, statusMgr); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile DaemonSet
	if err := r.reconcileDaemonSet(ctx, &spiffeCSIDriver, statusMgr, createOnlyMode); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
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

	// Use component-specific predicate to only reconcile for csi component resources
	controllerManagedResourcePredicates := builder.WithPredicates(utils.ControllerManagedResourcesForComponent(utils.ComponentCSI))

	err := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.SpiffeCSIDriver{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named(utils.ZeroTrustWorkloadIdentityManagerSpiffeCsiDriverControllerName).
		Watches(&appsv1.DaemonSet{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.ServiceAccount{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&storagev1.CSIDriver{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&securityv1.SecurityContextConstraints{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Complete(r)
	if err != nil {
		return err
	}
	return nil
}

// handleCreateOnlyMode checks and updates the create-only mode status
func (r *SpiffeCsiReconciler) handleCreateOnlyMode(driver *v1alpha1.SpiffeCSIDriver, statusMgr *status.Manager) bool {
	createOnlyMode := utils.IsInCreateOnlyMode(driver, &r.createOnlyMode)
	if createOnlyMode {
		r.log.Info("Running in create-only mode - will create resources if they don't exist but skip updates")
		statusMgr.AddCondition(utils.CreateOnlyModeStatusType, utils.CreateOnlyModeEnabled,
			"Create-only mode is enabled via ztwim.openshift.io/create-only annotation",
			metav1.ConditionTrue)
	} else {
		existingCondition := apimeta.FindStatusCondition(driver.Status.ConditionalStatus.Conditions, utils.CreateOnlyModeStatusType)
		if existingCondition != nil && existingCondition.Status == metav1.ConditionTrue {
			statusMgr.AddCondition(utils.CreateOnlyModeStatusType, utils.CreateOnlyModeDisabled,
				"Create-only mode is disabled",
				metav1.ConditionFalse)
		}
	}
	return createOnlyMode
}

// reconcileSCC reconciles the Spiffe CSI Driver Security Context Constraints
func (r *SpiffeCsiReconciler) reconcileSCC(ctx context.Context, driver *v1alpha1.SpiffeCSIDriver, statusMgr *status.Manager) error {
	SpiffeCsiSCC := generateSpiffeCSIDriverSCC()
	if err := controllerutil.SetControllerReference(driver, SpiffeCsiSCC, r.scheme); err != nil {
		r.log.Error(err, "failed to set the owner reference for the SCC resource")
		statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpiffeCSISCCGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	err := r.ctrlClient.Create(ctx, SpiffeCsiSCC)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		r.log.Error(err, "Failed to create SpiffeCsiSCC")
		statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpiffeCSISCCGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpiffeCSISCCResourceCreated",
		"SpiffeCSISCC resource created",
		metav1.ConditionTrue)
	return nil
}

// reconcileDaemonSet reconciles the Spiffe CSI Driver DaemonSet
func (r *SpiffeCsiReconciler) reconcileDaemonSet(ctx context.Context, driver *v1alpha1.SpiffeCSIDriver, statusMgr *status.Manager, createOnlyMode bool) error {
	spiffeCsiDaemonset := generateSpiffeCsiDriverDaemonSet(driver.Spec)
	if err := controllerutil.SetControllerReference(driver, spiffeCsiDaemonset, r.scheme); err != nil {
		r.log.Error(err, "failed to set owner reference for the DaemonSet resource")
		statusMgr.AddCondition(DaemonSetAvailable, "SpiffeCSIDaemonSetGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	var existingSpiffeCsiDaemonSet appsv1.DaemonSet
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: spiffeCsiDaemonset.Name, Namespace: spiffeCsiDaemonset.Namespace}, &existingSpiffeCsiDaemonSet)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, spiffeCsiDaemonset); err != nil {
			r.log.Error(err, "Failed to create SpiffeCsiDaemon set")
			statusMgr.AddCondition(DaemonSetAvailable, "SpiffeCSIDaemonSetCreationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return fmt.Errorf("failed to create DaemonSet: %w", err)
		}
		r.log.Info("Created spiffe csi DaemonSet")
	} else if err == nil && needsUpdate(existingSpiffeCsiDaemonSet, *spiffeCsiDaemonset) {
		if createOnlyMode {
			r.log.Info("Skipping DaemonSet update due to create-only mode")
		} else {
			spiffeCsiDaemonset.ResourceVersion = existingSpiffeCsiDaemonSet.ResourceVersion
			if err = r.ctrlClient.Update(ctx, spiffeCsiDaemonset); err != nil {
				r.log.Error(err, "failed to update spiffe csi daemon set")
				statusMgr.AddCondition(DaemonSetAvailable, "SpiffeCSIDaemonSetUpdateFailed",
					err.Error(),
					metav1.ConditionFalse)
				return fmt.Errorf("failed to update DaemonSet: %w", err)
			}
			r.log.Info("Updated spiffe csi DaemonSet")
		}
	} else if err != nil {
		r.log.Error(err, "Failed to get SpiffeCsiDaemon set")
		statusMgr.AddCondition(DaemonSetAvailable, "SpiffeCSIDaemonSetGetFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	// Check DaemonSet health/readiness
	statusMgr.CheckDaemonSetHealth(ctx, spiffeCsiDaemonset.Name, spiffeCsiDaemonset.Namespace, DaemonSetAvailable)

	return nil
}

// needsUpdate returns true if DaemonSet needs to be updated.
func needsUpdate(current, desired appsv1.DaemonSet) bool {
	if utils.DaemonSetSpecModified(&desired, &current) {
		return true
	} else if !reflect.DeepEqual(current.Labels, desired.Labels) {
		return true
	}
	return false
}
