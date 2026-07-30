/*
Copyright 2026.

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
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	operatorv1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/test/e2e/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const gcIntervalExpectedValue = "gcInterval: 10000000000"

var _ = Describe("SPIRE-617: gcInterval Fix", Ordered, func() {
	var testCtx context.Context

	BeforeAll(func() {
		testCtx = context.Background()
	})

	// ─── Journey 1: gcInterval ConfigMap Verification and SpireServer Health ───

	It("should have gcInterval set to 10s in controller-manager ConfigMap and SpireServer Ready", func() {
		By("Waiting for SpireServer ControllerManagerConfigAvailable=True and Ready=True")
		utils.WaitForSpireServerConditions(testCtx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
			"ControllerManagerConfigAvailable": metav1.ConditionTrue,
			"Ready":                            metav1.ConditionTrue,
		}, utils.DefaultTimeout)

		By("Verifying StatefulSet spire-server has ready replicas")
		utils.WaitForStatefulSetReady(testCtx, clientset, utils.SpireServerStatefulSetName, utils.OperatorNamespace, utils.DefaultTimeout)

		By("Retrieving spire-controller-manager ConfigMap")
		cm, err := clientset.CoreV1().ConfigMaps(utils.OperatorNamespace).Get(testCtx, utils.SpireControllerManagerConfigMapName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to get spire-controller-manager ConfigMap")

		configData, exists := cm.Data[utils.SpireControllerManagerConfigKey]
		Expect(exists).To(BeTrue(), "%s key should exist in ConfigMap data", utils.SpireControllerManagerConfigKey)

		By("Asserting gcInterval is set to 10s (10000000000 nanoseconds)")
		Expect(configData).To(ContainSubstring(gcIntervalExpectedValue),
			"controller-manager config should contain gcInterval set to 10s")

		By("Asserting gcInterval is NOT zero")
		Expect(configData).NotTo(ContainSubstring("gcInterval: 0\n"),
			"gcInterval must not be zero — this was the root cause of SPIRE-617")
	})

	// ─── Journey 2: gcInterval Resilience — Drift Correction and ConfigMap Deletion Recovery ───

	It("should restore gcInterval after ConfigMap drift and deletion", func() {
		By("BEFORE check: verifying gcInterval is correct in controller-manager ConfigMap")
		cm, err := clientset.CoreV1().ConfigMaps(utils.OperatorNamespace).Get(testCtx, utils.SpireControllerManagerConfigMapName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to get spire-controller-manager ConfigMap")
		configData := cm.Data[utils.SpireControllerManagerConfigKey]
		Expect(configData).To(ContainSubstring(gcIntervalExpectedValue),
			"initial ConfigMap should contain correct gcInterval")

		By("Drift injection: patching ConfigMap to set gcInterval to 0")
		driftedData := strings.Replace(configData, "gcInterval: 10000000000", "gcInterval: 0", 1)
		Expect(driftedData).NotTo(Equal(configData), "drift patch should have changed the data")
		cm.Data[utils.SpireControllerManagerConfigKey] = driftedData
		_, err = clientset.CoreV1().ConfigMaps(utils.OperatorNamespace).Update(testCtx, cm, metav1.UpdateOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to patch ConfigMap with drifted gcInterval")

		By("Drift recovery: waiting for operator to restore gcInterval to 10s")
		Eventually(func() bool {
			cm, err := clientset.CoreV1().ConfigMaps(utils.OperatorNamespace).Get(testCtx, utils.SpireControllerManagerConfigMapName, metav1.GetOptions{})
			if err != nil {
				fmt.Fprintf(GinkgoWriter, "failed to get ConfigMap: %v\n", err)
				return false
			}
			return strings.Contains(cm.Data[utils.SpireControllerManagerConfigKey], gcIntervalExpectedValue)
		}).WithPolling(utils.ShortInterval).WithTimeout(utils.ShortTimeout).Should(BeTrue(),
			"operator should restore gcInterval to 10s after drift")

		By("Verifying SpireServer remains Ready after drift recovery")
		spireServer := &operatorv1alpha1.SpireServer{}
		err = k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireServer)
		Expect(err).NotTo(HaveOccurred(), "failed to get SpireServer CR")

		By("Destructive action: deleting the entire controller-manager ConfigMap")
		err = clientset.CoreV1().ConfigMaps(utils.OperatorNamespace).Delete(testCtx, utils.SpireControllerManagerConfigMapName, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to delete spire-controller-manager ConfigMap")

		By("Deletion recovery: waiting for operator to recreate ConfigMap with correct gcInterval")
		Eventually(func() bool {
			cm, err := clientset.CoreV1().ConfigMaps(utils.OperatorNamespace).Get(testCtx, utils.SpireControllerManagerConfigMapName, metav1.GetOptions{})
			if err != nil {
				fmt.Fprintf(GinkgoWriter, "waiting for ConfigMap recreation: %v\n", err)
				return false
			}
			data, exists := cm.Data[utils.SpireControllerManagerConfigKey]
			if !exists {
				fmt.Fprintf(GinkgoWriter, "ConfigMap recreated but config key not yet present\n")
				return false
			}
			return strings.Contains(data, gcIntervalExpectedValue)
		}).WithPolling(utils.ShortInterval).WithTimeout(utils.DefaultTimeout).Should(BeTrue(),
			"operator should recreate ConfigMap with gcInterval=10s (not zero)")

		By("Verifying SpireServer recovers to Ready=True after ConfigMap recreation")
		utils.WaitForSpireServerConditions(testCtx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
			"ControllerManagerConfigAvailable": metav1.ConditionTrue,
			"Ready":                            metav1.ConditionTrue,
		}, utils.DefaultTimeout)
	})
})
