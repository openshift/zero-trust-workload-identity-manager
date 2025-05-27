package bootstrap

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func BootstrapCR(ctx context.Context, c client.Client, log logr.Logger) error {
	const (
		retryInterval = 2 * time.Second
		maxRetries    = 5
	)

	log.Info("Bootstrapping ZeroTrustWorkloadIdentityManager CR")

	for i := 0; i < maxRetries; i++ {
		instance := &v1alpha1.ZeroTrustWorkloadIdentityManager{}
		err := c.Get(ctx, types.NamespacedName{Name: "cluster"}, instance)
		if err == nil {
			log.Info("ZeroTrustWorkloadIdentityManager CR already exists")
			return nil
		}

		if !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to get ZeroTrustWorkloadIdentityManager")
			if isRetriable(err) {
				log.Info("Retrying due to retriable error")
				time.Sleep(retryInterval)
				continue
			}
			return err // non-retriable error
		}

		// Not found: create it
		instance = &v1alpha1.ZeroTrustWorkloadIdentityManager{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster",
				Labels: map[string]string{
					"app.kubernetes.io/app":    "zero-trust-workload-identity-manager",
					utils.AppManagedByLabelKey: utils.AppManagedByLabelValue,
				},
			},
			Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{},
		}

		err = c.Create(ctx, instance)
		if err == nil {
			log.Info("Successfully created ZeroTrustWorkloadIdentityManager CR")
			return nil
		}

		log.Error(err, "Failed to create ZeroTrustWorkloadIdentityManager CR")
		if isRetriable(err) {
			log.Info("Retrying due to retriable create error")
			time.Sleep(retryInterval)
			continue
		}
		return err
	}

	return errors.New("max retries exceeded")
}

func isRetriable(err error) bool {
	return apierrors.IsServerTimeout(err) ||
		apierrors.IsTimeout(err) ||
		apierrors.IsTooManyRequests(err) ||
		apierrors.IsInternalError(err) ||
		apierrors.IsConflict(err)
}
