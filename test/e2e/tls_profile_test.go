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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/zero-trust-workload-identity-manager/test/e2e/utils"
)

// tlsProfileCase drives one APIServer TLS profile churn + ConfigMap + wire validation.
type tlsProfileCase struct {
	name            string
	patchType       string // "" = no patch (PROFILE-DEFAULT)
	assertAPIServer string // "" skips APIServer type equality in AssertTLSProfileCompliance
}

var tlsProfileCases = []tlsProfileCase{
	{name: "PROFILE-DEFAULT", patchType: "", assertAPIServer: ""},
	{name: "PROFILE-INTERMEDIATE", patchType: utils.APIServerTLSProfileIntermediate, assertAPIServer: utils.APIServerTLSProfileIntermediate},
	{name: "PROFILE-MODERN", patchType: utils.APIServerTLSProfileModern, assertAPIServer: utils.APIServerTLSProfileModern},
	{name: "PROFILE-OLD", patchType: utils.APIServerTLSProfileOld, assertAPIServer: utils.APIServerTLSProfileOld},
}

// Full TLS profile compliance suite (on-demand via make test-e2e-tls).
// Assumes operator + operands are already installed and Ready on the target cluster.
var _ = Describe("TLS Profile Compliance", Label("tls"), Ordered, func() {
	BeforeAll(func() {
		ctx, cancel := context.WithTimeout(context.Background(), utils.TestContextTimeout)
		defer cancel()

		if !utils.IsAPIServerClusterAccessible(ctx, configClient) {
			Skip("cluster APIServer config not accessible; TLS tests require OpenShift")
		}

		utils.AssertAllOperandsPodsReady(ctx, k8sClient, clientset)
		utils.EnsureOpenSSLProbePod(ctx, k8sClient, clientset)
	})

	AfterAll(func() {
		ctx, cancel := context.WithTimeout(context.Background(), utils.TLSProfileRolloutTimeout)
		defer cancel()
		Expect(utils.RestoreAPIServerIntermediate(ctx, configClient)).To(Succeed())
		utils.WaitForTLSProfileRolloutComplete(ctx, k8sClient, clientset, utils.MinTLSVersionK8sTLS12, true)
	})

	for _, tc := range tlsProfileCases {
		tc := tc

		Context(tc.name, func() {
			It("applies profile and validates ConfigMaps and wire TLS", func() {
				rolloutCtx, cancel := context.WithTimeout(context.Background(), utils.TLSProfileRolloutTimeout)
				defer cancel()

				mapKey := tc.patchType
				if mapKey == "" {
					mapKey = "" // DEFAULT → Intermediate-equivalent mapping
				}
				minVersion, requireCiphers, ok := utils.ExpectedOperandTLSForAPIServerProfile(mapKey)
				Expect(ok).To(BeTrue(), "profile %q must be mapped for full TLS suite", mapKey)

				utils.ApplyTLSProfileAndWaitForRollout(rolloutCtx, configClient, k8sClient, clientset,
					tc.patchType, minVersion, requireCiphers)

				assertCtx, assertCancel := context.WithTimeout(context.Background(), utils.DefaultTimeout)
				defer assertCancel()
				utils.AssertTLSProfileCompliance(assertCtx, configClient, clientset,
					tc.assertAPIServer, minVersion, requireCiphers)

				wireCtx, wireCancel := context.WithTimeout(context.Background(), utils.TLSWireAssertTimeout)
				defer wireCancel()
				utils.AssertTLSWireCompliance(wireCtx, clientset, minVersion)
			})
		})

		// FUNC-005 runs immediately after Modern while the cluster is still on Modern.
		if tc.name == "PROFILE-MODERN" {
			It("FUNC-005: ClusterSPIFFEID apply and delete under Modern profile", func() {
				ctx, cancel := context.WithTimeout(context.Background(), utils.TestContextTimeout)
				defer cancel()
				utils.AssertClusterSPIFFEIDWorksUnderCurrentTLS(ctx, k8sClient, clientset)
			})
		}
	}
})
