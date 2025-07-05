/*
Copyright 2025.

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

package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/zero-trust-workload-identity-manager/test/e2e/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Zero Trust Workload Identity Manager", Ordered, func() {
	var testCtx context.Context

	BeforeEach(func() {
		var cancel context.CancelFunc
		testCtx, cancel = context.WithTimeout(context.Background(), utils.DefaultTimeout)
		DeferCleanup(cancel)
	})

	Context("when installing the operator", func() {
		It("should create a normal ZeroTrustWorkloadIdentityManager object", func() {
			By("Waiting for all resource generation conditions to be True")
			utils.WaitForZeroTrustWorkloadIdentityManagerConditions(testCtx, k8sClient, 2*time.Minute)
		})

		It("should create a healthy operator deployment", func() {
			By("Waiting for operator deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, 2*time.Minute)
		})

		It("should recover from the pod force deletion", func() {
			By("Deleting operator pod manually")
			err := clientset.CoreV1().Pods(utils.OperatorNamespace).DeleteCollection(testCtx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: utils.OperatorPodLabelSelector,
			})
			Expect(err).NotTo(HaveOccurred(), "pod should be deleted")

			By("Waiting for operator deployment to become Available again")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, 2*time.Minute)
		})
	})
})
