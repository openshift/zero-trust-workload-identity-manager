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

var _ = Describe("UpstreamAuthority Vault", Ordered, Label("upstream-vault"), func() {
	var testCtx context.Context
	var npEnabled bool

	BeforeAll(func() {
		ctx := context.Background()
		serverCheck := &operatorv1alpha1.SpireServer{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: utils.OperandCRName}, serverCheck)).To(Succeed())

		npEnabled = utils.SpireServerHasNetworkPoliciesFeature(ctx, k8sClient)

		setupCtx, cancel := context.WithTimeout(ctx, utils.DatastoreSetupTimeout)
		defer cancel()

		utils.InstallVaultUpstream(setupCtx, k8sClient, clientset, utils.OperatorNamespace)
		utils.PatchSpireServerVaultUpstream(setupCtx, k8sClient, utils.OperatorNamespace)
		utils.WaitForSpireServerReadyAfterVaultChange(setupCtx, k8sClient, clientset, utils.OperatorNamespace)
	})

	AfterAll(func() {
		ctx, cancel := context.WithTimeout(context.Background(), utils.DefaultTimeout)
		defer cancel()
		utils.CleanupVaultUpstream(ctx, k8sClient, clientset, utils.OperatorNamespace)
	})

	BeforeEach(func() {
		var cancel context.CancelFunc
		testCtx, cancel = context.WithTimeout(context.Background(), utils.TestContextTimeout)
		DeferCleanup(cancel)
	})

	It("server.conf should contain the vault UpstreamAuthority plugin", func() {
		var pd *utils.VaultPluginData
		Eventually(func(g Gomega) {
			got, err := utils.GetVaultPluginData(testCtx, clientset, utils.OperatorNamespace)
			g.Expect(err).NotTo(HaveOccurred())
			pd = got
		}).WithTimeout(utils.ShortTimeout).WithPolling(utils.ShortInterval).Should(Succeed())

		Expect(pd.VaultAddr).To(ContainSubstring(utils.VaultServiceName))
		Expect(pd.VaultAddr).To(ContainSubstring("8200"))
	})

	It("should expose vault egress NetworkPolicy when network policies are enabled", func() {
		if !npEnabled {
			Skip("NetworkPoliciesAvailable not reported on SpireServer")
		}
		vaultNP := utils.WaitForNetworkPolicy(testCtx, clientset, utils.OperatorNamespace,
			utils.SpireServerNetworkPolicyVaultEgressName, utils.DefaultTimeout)
		Expect(utils.NetworkPolicyHasEgressTCPPort(vaultNP, int32(utils.VaultServicePort))).To(BeTrue())
	})

	It("workload SVID attestation should succeed with vault upstream CA", Label("attestation"), func() {
		f := utils.SetupAttestationTest(testCtx, k8sClient, clientset, "vault-svid", nil)
		_, _, _, cert, err := utils.ReadAndParseSVIDCertificate(testCtx, f.Namespace, f.PodName, f.AppContainer)
		Expect(err).NotTo(HaveOccurred())
		Expect(cert.NotAfter.After(time.Now())).To(BeTrue())
	})
})
