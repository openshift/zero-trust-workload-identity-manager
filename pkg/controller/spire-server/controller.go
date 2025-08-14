package spire_server

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	SpireServerStatefulSetGeneration          = "SpireServerStatefulSetGeneration"
	SpireServerConfigMapGeneration            = "SpireServerConfigMapGeneration"
	SpireControllerManagerConfigMapGeneration = "SpireControllerManagerConfigMapGeneration"
	SpireBundleConfigMapGeneration            = "SpireBundleConfigMapGeneration"
)

type reconcilerStatus struct {
	Status  metav1.ConditionStatus
	Message string
	Reason  string
}

// SpireServerReconciler reconciles a SpireServer object
type SpireServerReconciler struct {
	ctrlClient    customClient.CustomCtrlClient
	ctx           context.Context
	eventRecorder record.EventRecorder
	log           logr.Logger
	scheme        *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// New returns a new Reconciler instance.
func New(mgr ctrl.Manager) (*SpireServerReconciler, error) {
	c, err := customClient.NewCustomClient(mgr)
	if err != nil {
		return nil, err
	}
	return &SpireServerReconciler{
		ctrlClient:    c,
		ctx:           context.Background(),
		eventRecorder: mgr.GetEventRecorderFor(utils.ZeroTrustWorkloadIdentityManagerSpireServerControllerName),
		log:           ctrl.Log.WithName(utils.ZeroTrustWorkloadIdentityManagerSpireServerControllerName),
		scheme:        mgr.GetScheme(),
	}, nil
}

func (r *SpireServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if utils.IsReconciliationPaused() {
		r.log.Info("Reconciliation paused by environment flag", "env", utils.ReconciliationPausedEnv)
		return ctrl.Result{}, nil
	}
	var server v1alpha1.SpireServer
	if err := r.ctrlClient.Get(ctx, req.NamespacedName, &server); err != nil {
		if kerrors.IsNotFound(err) {
			r.log.Info("SpireServer resource not found. Ignoring since object must be deleted or not been created.")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	reconcileStatus := map[string]reconcilerStatus{}
	defer func(reconcileStatus map[string]reconcilerStatus) {
		originalStatus := server.Status.DeepCopy()
		if server.Status.ConditionalStatus.Conditions == nil {
			server.Status.ConditionalStatus = v1alpha1.ConditionalStatus{
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
			apimeta.SetStatusCondition(&server.Status.ConditionalStatus.Conditions, newCondition)
		}
		newConfig := server.DeepCopy()
		if !equality.Semantic.DeepEqual(originalStatus, &server.Status) {
			if err := r.ctrlClient.StatusUpdateWithRetry(ctx, newConfig); err != nil {
				r.log.Error(err, "failed to update status")
			}
		}
	}(reconcileStatus)

	if server.Spec.JwtIssuer == "" {
		server.Spec.JwtIssuer = "https://oidc-discovery." + server.Spec.TrustDomain
	}

	spireServerConfigMap, err := GenerateSpireServerConfigMap(&server.Spec)
	if err != nil {
		r.log.Error(err, "failed to generate spire server config map")
		reconcileStatus[SpireServerConfigMapGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireServerConfigMapGenerationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	// Set owner reference so GC cleans up when CR is deleted
	if err = controllerutil.SetControllerReference(&server, spireServerConfigMap, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		reconcileStatus[SpireServerConfigMapGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireServerConfigMapGenerationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}

	var existingSpireServerCM corev1.ConfigMap
	err = r.ctrlClient.Get(ctx, types.NamespacedName{Name: spireServerConfigMap.Name, Namespace: spireServerConfigMap.Namespace}, &existingSpireServerCM)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, spireServerConfigMap); err != nil {
			reconcileStatus[SpireServerConfigMapGeneration] = reconcilerStatus{
				Status:  metav1.ConditionFalse,
				Reason:  "SpireServerConfigMapGenerationFailed",
				Message: err.Error(),
			}
			return ctrl.Result{}, fmt.Errorf("failed to create ConfigMap: %w", err)
		}
		r.log.Info("Created spire server ConfigMap")
	} else if err == nil && existingSpireServerCM.Data["server.conf"] != spireServerConfigMap.Data["server.conf"] {
		existingSpireServerCM.Data = spireServerConfigMap.Data
		if err = r.ctrlClient.Update(ctx, &existingSpireServerCM); err != nil {
			reconcileStatus[SpireServerConfigMapGeneration] = reconcilerStatus{
				Status:  metav1.ConditionFalse,
				Reason:  "SpireServerConfigMapGenerationFailed",
				Message: err.Error(),
			}
			return ctrl.Result{}, fmt.Errorf("failed to update ConfigMap: %w", err)
		}
		r.log.Info("Updated ConfigMap with new config")
	} else if err != nil {
		reconcileStatus[SpireServerConfigMapGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireServerConfigMapGenerationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	reconcileStatus[SpireServerConfigMapGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "SpireConfigMapResourceCreated",
		Message: "SpireServer config map resources applied",
	}

	spireServerConfJSON, err := marshalToJSON(generateServerConfMap(&server.Spec))
	if err != nil {
		r.log.Error(err, "failed to marshal spire server config map to JSON")
		return ctrl.Result{}, err
	}

	spireServerConfigMapHash := generateConfigHash(spireServerConfJSON)

	spireControllerManagerConfig, err := generateSpireControllerManagerConfigYaml(&server.Spec)
	if err != nil {
		r.log.Error(err, "Failed to generate spire controller manager config")
		reconcileStatus[SpireControllerManagerConfigMapGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireControllerManagerConfigMapGenerationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	spireControllerManagerConfigMap := generateControllerManagerConfigMap(spireControllerManagerConfig)
	// Set owner reference so GC cleans up when CR is deleted
	if err = controllerutil.SetControllerReference(&server, spireControllerManagerConfigMap, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on spire controller manager config")
		reconcileStatus[SpireControllerManagerConfigMapGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireControllerManagerConfigMapGenerationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}

	var existingSpireControllerManagerCM corev1.ConfigMap
	err = r.ctrlClient.Get(ctx, types.NamespacedName{Name: spireControllerManagerConfigMap.Name, Namespace: spireControllerManagerConfigMap.Namespace}, &existingSpireControllerManagerCM)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, spireControllerManagerConfigMap); err != nil {
			r.log.Error(err, "failed to create spire controller manager config map")
			reconcileStatus[SpireControllerManagerConfigMapGeneration] = reconcilerStatus{
				Status:  metav1.ConditionFalse,
				Reason:  "SpireControllerManagerConfigMapGenerationFailed",
				Message: err.Error(),
			}
			return ctrl.Result{}, fmt.Errorf("failed to create ConfigMap: %w", err)
		}
		r.log.Info("Created spire controller manager ConfigMap")
	} else if err == nil && existingSpireControllerManagerCM.Data["controller-manager-config.yaml"] != existingSpireControllerManagerCM.Data["controller-manager-config.yaml"] {
		existingSpireControllerManagerCM.Data = spireControllerManagerConfigMap.Data
		if err = r.ctrlClient.Update(ctx, &existingSpireControllerManagerCM); err != nil {
			reconcileStatus[SpireControllerManagerConfigMapGeneration] = reconcilerStatus{
				Status:  metav1.ConditionFalse,
				Reason:  "SpireControllerManagerConfigMapGenerationFailed",
				Message: err.Error(),
			}
			return ctrl.Result{}, fmt.Errorf("failed to update ConfigMap: %w", err)
		}
		r.log.Info("Updated ConfigMap with new config")
	} else if err != nil {
		r.log.Error(err, "failed to update spire controller manager config map")
		return ctrl.Result{}, err
	}

	reconcileStatus[SpireControllerManagerConfigMapGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "SpireControllerManagerConfigMapCreated",
		Message: "spire controller manager config map resources applied",
	}

	spireControllerManagerConfigMapHash := generateConfigHashFromString(spireControllerManagerConfig)

	spireBundleCM, err := generateSpireBundleConfigMap(&server.Spec)
	if err != nil {
		r.log.Error(err, "failed to generate spire bundle config map")
		reconcileStatus[SpireBundleConfigMapGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireBundleConfigMapGenerationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	if err := controllerutil.SetControllerReference(&server, spireBundleCM, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on spire bundle config")
		reconcileStatus[SpireBundleConfigMapGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireBundleConfigMapGenerationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	err = r.ctrlClient.Create(ctx, spireBundleCM)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		r.log.Error(err, "failed to create spire bundle config map")
		reconcileStatus[SpireBundleConfigMapGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireBundleConfigMapGenerationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, fmt.Errorf("failed to create spire-bundle ConfigMap: %w", err)
	}

	reconcileStatus[SpireBundleConfigMapGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "SpireBundleConfigMapCreated",
		Message: "spire bundle config map resources applied",
	}

	sts := GenerateSpireServerStatefulSet(&server.Spec, spireServerConfigMapHash, spireControllerManagerConfigMapHash)
	if err := controllerutil.SetControllerReference(&server, sts, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on spire server stateful set resource")
		reconcileStatus[SpireServerStatefulSetGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireServerStatefulSetGenerationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}

	// 5. Create or Update StatefulSet
	var existingSTS appsv1.StatefulSet
	err = r.ctrlClient.Get(ctx, types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, &existingSTS)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, sts); err != nil {
			reconcileStatus[SpireServerStatefulSetGeneration] = reconcilerStatus{
				Status:  metav1.ConditionFalse,
				Reason:  "SpireServerStatefulSetGenerationFailed",
				Message: err.Error(),
			}
			return ctrl.Result{}, fmt.Errorf("failed to create StatefulSet: %w", err)
		}
		r.log.Info("Created spire server StatefulSet")
	} else if err == nil && needsUpdate(existingSTS, *sts) {
		if err = r.ctrlClient.Update(ctx, sts); err != nil {
			reconcileStatus[SpireServerStatefulSetGeneration] = reconcilerStatus{
				Status:  metav1.ConditionFalse,
				Reason:  "SpireServerStatefulSetGenerationFailed",
				Message: err.Error(),
			}
			return ctrl.Result{}, fmt.Errorf("failed to update StatefulSet: %w", err)
		}
		r.log.Info("Updated spire server StatefulSet")
	} else if err != nil {
		r.log.Error(err, "failed to update spire server stateful set resource")
		return ctrl.Result{}, err
	}
	reconcileStatus[SpireServerStatefulSetGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "SpireServerStatefulSetCreated",
		Message: "spire server stateful set resources applied",
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

func (r *SpireServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
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
		For(&v1alpha1.SpireServer{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named(utils.ZeroTrustWorkloadIdentityManagerSpireServerControllerName).
		Watches(&appsv1.StatefulSet{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Complete(r)
	if err != nil {
		return err
	}
	return nil
}

// needsUpdate returns true if StatefulSet needs to be updated based on config checksum
func needsUpdate(current, desired appsv1.StatefulSet) bool {
	if current.Spec.Template.Annotations[spireServerStatefulSetSpireServerConfigHashAnnotationKey] != desired.Spec.Template.Annotations[spireServerStatefulSetSpireServerConfigHashAnnotationKey] {
		return true
	} else if current.Spec.Template.Annotations[spireServerStatefulSetSpireControllerMangerConfigHashAnnotationKey] != desired.Spec.Template.Annotations[spireServerStatefulSetSpireControllerMangerConfigHashAnnotationKey] {
		return true
	} else if utils.StatefulSetSpecModified(&desired, &current) {
		return true
	}
	return false
}
