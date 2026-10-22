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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	operatorv1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/test/e2e/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// registerExternalMySQLDatastoreE2E is invoked from the main Ordered e2e Describe so
// install/operand setup runs first; Label("datastore-mysql") allows focused runs only.
func registerExternalMySQLDatastoreE2E() {
	Context("External MySQL datastore", Ordered, Label("datastore-mysql"), func() {
		var testCtx context.Context
		var npEnabled bool

		BeforeAll(func() {
			ctx := context.Background()
			serverCheck := &operatorv1alpha1.SpireServer{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: utils.OperandCRName}, serverCheck)).To(Succeed())

			npEnabled = utils.SpireServerHasNetworkPoliciesFeature(ctx, k8sClient)

			setupCtx, cancel := context.WithTimeout(ctx, utils.DatastoreSetupTimeout)
			defer cancel()

			utils.InstallMySQLDatastore(setupCtx, k8sClient, clientset, utils.OperatorNamespace)
			utils.PatchSpireServerMySQLDatastore(setupCtx, k8sClient, utils.OperatorNamespace)
			utils.WaitForSpireServerReadyAfterDatastoreChange(setupCtx, k8sClient, clientset, utils.OperatorNamespace)
		})

		AfterAll(func() {
			ctx, cancel := context.WithTimeout(context.Background(), utils.DatastoreSetupTimeout)
			defer cancel()
			utils.CleanupMySQLDatastore(ctx, k8sClient, clientset, utils.OperatorNamespace)
		})

		BeforeEach(func() {
			var cancel context.CancelFunc
			testCtx, cancel = context.WithTimeout(context.Background(), utils.TestContextTimeout)
			DeferCleanup(cancel)
		})

		It("server.conf should configure mysql sql DataStore plugin", func() {
			var pd *utils.SQLDatastorePluginData
			Eventually(func(g Gomega) {
				got, err := utils.GetSQLDatastorePluginData(testCtx, clientset, utils.OperatorNamespace)
				g.Expect(err).NotTo(HaveOccurred())
				pd = got
			}).WithTimeout(utils.ShortTimeout).WithPolling(utils.ShortInterval).Should(Succeed())

			Expect(pd.DatabaseType).To(Equal("mysql"))
			Expect(pd.ConnectionString).To(ContainSubstring(":3307"))
		})

		It("should expose database egress NetworkPolicy when network policies are enabled", func() {
			if !npEnabled {
				Skip("NetworkPoliciesAvailable not reported on SpireServer")
			}
			dbNP := utils.WaitForNetworkPolicy(testCtx, clientset, utils.OperatorNamespace,
				utils.SpireServerNetworkPolicyDatabaseEgressName, utils.DefaultTimeout)
			Expect(utils.NetworkPolicyHasEgressTCPPort(dbNP, utils.MySQLServicePort)).To(BeTrue())
			Expect(utils.NetworkPolicyHasEgressTCPPort(dbNP, 3306)).To(BeTrue(), "OVN matches Service targetPort for egress")
		})

		It("workload SVID attestation should succeed with mysql datastore", Label("attestation"), func() {
			f := utils.SetupAttestationTest(testCtx, k8sClient, clientset, "mysql-svid", nil)
			_, _, _, cert, err := utils.ReadAndParseSVIDCertificate(testCtx, f.Namespace, f.PodName, f.AppContainer)
			Expect(err).NotTo(HaveOccurred())
			Expect(cert.NotAfter.After(time.Now())).To(BeTrue())
		})
	})
}
