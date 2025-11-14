package client

import (
	"context"
	"fmt"
	"reflect"

	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

var (
	// cacheResources is the list of resources that the controller watches,
	// and creates informers for.
	cacheResources = []client.Object{
		&rbacv1.Role{},
		&rbacv1.RoleBinding{},
		&rbacv1.ClusterRole{},
		&rbacv1.ClusterRoleBinding{},
		&storagev1.CSIDriver{},
		&corev1.ServiceAccount{},
		&corev1.Service{},
		&corev1.ConfigMap{},
		&appsv1.Deployment{},
		&appsv1.DaemonSet{},
		&appsv1.StatefulSet{},
		&admissionregistrationv1.ValidatingWebhookConfiguration{},
		&routev1.Route{},
		&spiffev1alpha1.ClusterSPIFFEID{},
	}

	cacheResourceWithoutReqSelectors = []client.Object{
		&v1alpha1.ZeroTrustWorkloadIdentityManager{},
		&v1alpha1.SpireAgent{},
		&v1alpha1.SpiffeCSIDriver{},
		&v1alpha1.SpireServer{},
		&v1alpha1.SpireOIDCDiscoveryProvider{},
	}

	informerResources = []client.Object{
		&corev1.ServiceAccount{},
		&corev1.Service{},
		&rbacv1.Role{},
		&rbacv1.RoleBinding{},
		&rbacv1.ClusterRole{},
		&rbacv1.ClusterRoleBinding{},
		&storagev1.CSIDriver{},
		&corev1.ConfigMap{},
		&appsv1.Deployment{},
		&appsv1.DaemonSet{},
		&appsv1.StatefulSet{},
		&admissionregistrationv1.ValidatingWebhookConfiguration{},
		&v1alpha1.ZeroTrustWorkloadIdentityManager{},
		&v1alpha1.SpireAgent{},
		&v1alpha1.SpiffeCSIDriver{},
		&v1alpha1.SpireServer{},
		&v1alpha1.SpireOIDCDiscoveryProvider{},
		&routev1.Route{},
		&spiffev1alpha1.ClusterSPIFFEID{},
	}
)

type customCtrlClientImpl struct {
	client.Client
	apiReader client.Reader
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate -o fakes . CustomCtrlClient
type CustomCtrlClient interface {
	Get(context.Context, client.ObjectKey, client.Object) error
	List(context.Context, client.ObjectList, ...client.ListOption) error
	StatusUpdate(context.Context, client.Object, ...client.SubResourceUpdateOption) error
	Update(context.Context, client.Object, ...client.UpdateOption) error
	UpdateWithRetry(context.Context, client.Object, ...client.UpdateOption) error
	Create(context.Context, client.Object, ...client.CreateOption) error
	Delete(context.Context, client.Object, ...client.DeleteOption) error
	Patch(context.Context, client.Object, client.Patch, ...client.PatchOption) error
	Exists(context.Context, client.ObjectKey, client.Object) (bool, error)
	CreateOrUpdateObject(ctx context.Context, obj client.Object) error
	StatusUpdateWithRetry(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error
	GetClient() client.Client
}

func NewCustomClient(m manager.Manager) (CustomCtrlClient, error) {
	c, err := BuildCustomClient(m)
	if err != nil {
		return nil, fmt.Errorf("failed to build custom client: %w", err)
	}
	return &customCtrlClientImpl{
		Client:    c,
		apiReader: m.GetAPIReader(),
	}, nil
}

func (c *customCtrlClientImpl) Get(
	ctx context.Context, key client.ObjectKey, obj client.Object,
) error {
	return c.Client.Get(ctx, key, obj)
}

func (c *customCtrlClientImpl) List(
	ctx context.Context, list client.ObjectList, opts ...client.ListOption,
) error {
	return c.Client.List(ctx, list, opts...)
}

func (c *customCtrlClientImpl) Create(
	ctx context.Context, obj client.Object, opts ...client.CreateOption,
) error {
	return c.Client.Create(ctx, obj, opts...)
}

func (c *customCtrlClientImpl) Delete(
	ctx context.Context, obj client.Object, opts ...client.DeleteOption,
) error {
	return c.Client.Delete(ctx, obj, opts...)
}

func (c *customCtrlClientImpl) Update(
	ctx context.Context, obj client.Object, opts ...client.UpdateOption,
) error {
	return c.Client.Update(ctx, obj, opts...)
}

func (c *customCtrlClientImpl) UpdateWithRetry(
	ctx context.Context, obj client.Object, opts ...client.UpdateOption,
) error {
	key := types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := reflect.New(reflect.TypeOf(obj).Elem()).Interface().(client.Object)
		if err := c.Client.Get(ctx, key, current); err != nil {
			return fmt.Errorf("failed to fetch latest %q for update: %w", key, err)
		}
		obj.SetResourceVersion(current.GetResourceVersion())
		if err := c.Client.Update(ctx, obj, opts...); err != nil {
			return fmt.Errorf("failed to update %q resource: %w", key, err)
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (c *customCtrlClientImpl) StatusUpdateWithRetry(
	ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption,
) error {
	key := types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := reflect.New(reflect.TypeOf(obj).Elem()).Interface().(client.Object)
		if err := c.Client.Get(ctx, key, current); err != nil {
			return fmt.Errorf("failed to fetch latest %q for update: %w", key, err)
		}
		obj.SetResourceVersion(current.GetResourceVersion())
		if err := c.Client.Status().Update(ctx, obj, opts...); err != nil {
			return fmt.Errorf("failed to update %q resource: %w", key, err)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (c *customCtrlClientImpl) StatusUpdate(
	ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption,
) error {
	return c.Client.Status().Update(ctx, obj, opts...)
}

func (c *customCtrlClientImpl) Patch(
	ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption,
) error {
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func (c *customCtrlClientImpl) Exists(ctx context.Context, key client.ObjectKey, obj client.Object) (bool, error) {
	if err := c.Client.Get(ctx, key, obj); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CreateOrUpdateObject tries to create the object, updates if already exists
func (c *customCtrlClientImpl) CreateOrUpdateObject(ctx context.Context, obj client.Object) error {
	err := c.Create(ctx, obj)
	if err != nil && errors.IsAlreadyExists(err) {
		return c.Update(ctx, obj)
	}
	return err
}

// GetClient returns the underlying client.Client
func (c *customCtrlClientImpl) GetClient() client.Client {
	return c.Client
}

// NewCacheBuilder returns a cache builder function that configures the manager's cache
// with custom label selectors and informers. This function should be passed to the
// manager's NewCache option to ensure a unified cache is used.
func NewCacheBuilder() (cache.NewCacheFunc, error) {
	spireServerManagedResourceAppManagedReq, err := labels.NewRequirement(utils.AppManagedByLabelKey, selection.Equals, []string{utils.AppManagedByLabelValue})
	if err != nil {
		return nil, err
	}
	managedResourceLabelReqSelector := labels.NewSelector().Add(*spireServerManagedResourceAppManagedReq)

	return func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
		// Configure cache with custom label selectors
		customCacheObjects := map[client.Object]cache.ByObject{}
		for _, resource := range cacheResources {
			customCacheObjects[resource] = cache.ByObject{
				Label: managedResourceLabelReqSelector,
			}
		}
		for _, resource := range cacheResourceWithoutReqSelectors {
			customCacheObjects[resource] = cache.ByObject{}
		}

		// Merge custom cache objects with any existing ones from opts
		if opts.ByObject == nil {
			opts.ByObject = customCacheObjects
		} else {
			for k, v := range customCacheObjects {
				opts.ByObject[k] = v
			}
		}

		opts.ReaderFailOnMissingInformer = true

		// Create the cache with the merged options
		newCache, err := cache.New(config, opts)
		if err != nil {
			return nil, err
		}

		// Pre-register informers for all resources
		for _, resource := range informerResources {
			if _, err := newCache.GetInformer(context.Background(), resource); err != nil {
				return nil, err
			}
		}

		return newCache, nil
	}, nil
}

// BuildCustomClient now uses the manager's unified cache instead of creating a separate one.
// This eliminates the race condition between manager and reconciler caches.
func BuildCustomClient(mgr ctrl.Manager) (client.Client, error) {
	// Use the manager's client directly, which is backed by the unified cache
	return mgr.GetClient(), nil
}
