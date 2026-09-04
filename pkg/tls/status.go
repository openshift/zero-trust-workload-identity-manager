/*
Copyright 2026 Red Hat, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tls

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	customClient "github.com/openshift/zero-trust-workload-identity-manager/pkg/client"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ztwimClusterName = "cluster"

// ReportTLSResolutionFailure best-effort updates Ready=False on the ZTWIM cluster CR.
func ReportTLSResolutionFailure(ctx context.Context, c client.Client, log logr.Logger, err error) {
	var ztwim v1alpha1.ZeroTrustWorkloadIdentityManager
	key := types.NamespacedName{Name: ztwimClusterName}
	if getErr := c.Get(ctx, key, &ztwim); getErr != nil {
		if apierrors.IsNotFound(getErr) {
			log.V(1).Info("ZTWIM cluster CR not found, skipping TLS failure status update")
			return
		}
		log.Error(getErr, "failed to get ZTWIM cluster CR for TLS failure status update")
		return
	}

	sm := status.NewManager(customClient.NewCustomClientFromClient(c))
	sm.AddCondition(v1alpha1.Ready, v1alpha1.ReasonFailed, "failed to resolve cluster TLS profile", metav1.ConditionFalse)
	if patchErr := sm.ApplyStatus(ctx, &ztwim, func() *v1alpha1.ConditionalStatus {
		return &ztwim.Status.ConditionalStatus
	}); patchErr != nil {
		log.Error(patchErr, "failed to update ZTWIM cluster CR status for TLS resolution failure")
	}
}
