package bootstrap

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

// BootstrapCR ensures the ZeroTrustWorkloadIdentityManager CR is created if it doesn't exist
func BootstrapCR(ctx context.Context, c client.Client, log logr.Logger) error {
	const (
		retryInterval = 2 * time.Second
		maxRetries    = 3
	)

	log.Info("Bootstrapping ZeroTrustWorkloadIdentityManager CR")

	for i := 0; i < maxRetries; i++ {
		instance := &v1alpha1.ZeroTrustWorkloadIdentityManager{}
		err := c.Get(ctx, types.NamespacedName{Name: "cluster"}, instance)
		if err == nil {
			log.Info("ZeroTrustWorkloadIdentityManager CR already exists")
			return nil
		}

		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get ZeroTrustWorkloadIdentityManager")
			return err
		}

		// Not found, create it
		instance = &v1alpha1.ZeroTrustWorkloadIdentityManager{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster",
				Labels: map[string]string{
					"app.kubernetes.io/app":    "zero-trust-workload-identity-manager",
					utils.AppManagedByLabelKey: utils.AppManagedByLabelValue,
				},
			},
			Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{}, // Empty spec
		}

		if err := c.Create(ctx, instance); err != nil {
			log.Error(err, "Failed to create ZeroTrustWorkloadIdentityManager CR")
		} else {
			log.Info("Successfully created ZeroTrustWorkloadIdentityManager CR")
			return nil
		}

		time.Sleep(retryInterval)
	}
	return nil
}
