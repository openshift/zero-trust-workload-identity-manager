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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ocpconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/test/e2e/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// tlsProfileCase drives one APIServer TLS profile churn + ConfigMap + tls-scanner validation.
type tlsProfileCase struct {
	name            string
	patchType       string // "" = no patch (PROFILE-DEFAULT)
	assertAPIServer string // "" skips APIServer type equality in AssertTLSProfileCompliance
	checkSPIFFEID   bool
}

var tlsProfileCases = []tlsProfileCase{
	{name: "PROFILE-DEFAULT", patchType: "", assertAPIServer: ""},
	{name: "PROFILE-INTERMEDIATE", patchType: utils.APIServerTLSProfileIntermediate, assertAPIServer: utils.APIServerTLSProfileIntermediate},
	{name: "PROFILE-MODERN", patchType: utils.APIServerTLSProfileModern, assertAPIServer: utils.APIServerTLSProfileModern, checkSPIFFEID: true},
	{name: "PROFILE-OLD", patchType: utils.APIServerTLSProfileOld, assertAPIServer: utils.APIServerTLSProfileOld},
	// TODO(Phase-2): Add PROFILE-CUSTOM once tls-scanner supports --tls-profile-type Custom
	// or we can verify the auto-detect path (omit flag → scanner reads APIServer CR) in CI.
}

// Full TLS profile compliance suite (on-demand via make test-e2e-tls).
// Assumes operator + operands are already installed and Ready on the target cluster.
var _ = Describe("TLS Profile Compliance", Label("tls"), Ordered, func() {
	var (
		savedAPIServerTLSProfile *ocpconfigv1.TLSSecurityProfile
		tlsSuiteReady            bool
	)

	BeforeAll(func() {
		ctx, cancel := context.WithTimeout(context.Background(), utils.MachineConfigReadyTimeout+utils.DefaultTimeout)
		defer cancel()

		if !utils.IsAPIServerClusterAccessible(ctx, configClient) {
			Skip("cluster APIServer config not accessible; TLS tests require OpenShift")
		}

		scannerImage := utils.TLSScannerImage()
		if scannerImage == "" {
			Fail("TLS_SCANNER_IMAGE must be set for TLS profile suite")
		}
		fmt.Fprintf(GinkgoWriter, "tls-scanner image: %s\n", scannerImage)

		apiServer, err := configClient.APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to read APIServer cluster config for TLS profile backup")
		if apiServer.Spec.TLSSecurityProfile != nil {
			savedAPIServerTLSProfile = apiServer.Spec.TLSSecurityProfile.DeepCopy()
		}
		tlsSuiteReady = true

		utils.WaitForClusterMachinesReady(ctx, k8sClient, clientset)
		utils.AssertAllOperandsPodsReady(ctx, k8sClient, clientset)
		utils.EnsureScannerRBAC(ctx, clientset)
	})

	// Restore once at the end. Restoring after every case would reboot the nodes again
	// and does not fit a cluster that lives for about 3 hours.
	AfterAll(func() {
		if !tlsSuiteReady {
			return
		}
		restoreSavedAPIServerTLSProfile(savedAPIServerTLSProfile)
	})

	for _, tc := range tlsProfileCases {
		tc := tc

		It(tc.name+": applies profile and validates ConfigMaps and tls-scanner compliance", func() {
			rolloutCtx, cancel := context.WithTimeout(context.Background(), utils.TLSProfileApplyTimeout)
			defer cancel()

			minVersion, requireCiphers, ok := utils.ExpectedOperandTLSForAPIServerProfile(tc.patchType)
			Expect(ok).To(BeTrue(), "profile %q must be mapped for full TLS suite", tc.patchType)

			utils.ApplyTLSProfileAndWaitForRollout(rolloutCtx, configClient, k8sClient, clientset,
				tc.patchType, minVersion, requireCiphers)

			assertCtx, assertCancel := context.WithTimeout(context.Background(), utils.DefaultTimeout)
			defer assertCancel()
			utils.AssertTLSProfileCompliance(assertCtx, configClient, clientset,
				tc.assertAPIServer, minVersion, requireCiphers)

			// Run tls-scanner to validate wire-level TLS compliance.
			scannerProfileType := tc.assertAPIServer
			if scannerProfileType == "" {
				// PROFILE-DEFAULT maps to Intermediate for scanner expectations.
				scannerProfileType = utils.APIServerTLSProfileIntermediate
			}
			scannerCtx, scannerCancel := context.WithTimeout(context.Background(), utils.TLSScannerRunTimeout+utils.TLSScannerReadyTimeout)
			defer scannerCancel()
			Expect(utils.RunTLSScanner(scannerCtx, clientset, scannerProfileType)).
				To(Succeed(), "tls-scanner failed for profile %s", tc.name)

			if tc.checkSPIFFEID {
				spiffeCtx, spiffeCancel := context.WithTimeout(context.Background(), utils.TestContextTimeout)
				defer spiffeCancel()
				utils.AssertClusterSPIFFEIDWorksUnderCurrentTLS(spiffeCtx, k8sClient, clientset)
			}
		})
	}

	// ERR-001: invalid Custom TLS profiles must be rejected by APIServer admission.
	// Uses JSON Patch (replace) so the invalid shape fully replaces any existing
	// profile — merge patch would silently merge into an active Custom profile,
	// turning some of these into no-ops.
	Context("ERR-001: invalid Custom profiles rejected by admission", func() {
		type invalidCustomCase struct {
			name  string
			value string // JSON value for the tlsSecurityProfile replacement
		}

		invalidCases := []invalidCustomCase{
			{
				// Invalid minTLSVersion string — not a recognised enum value.
				name:  "ERR-001-B: invalid minTLSVersion value",
				value: `{"type":"Custom","custom":{"minTLSVersion":"TLSv999","ciphers":["ECDHE-RSA-AES128-GCM-SHA256","ECDHE-ECDSA-AES128-GCM-SHA256"]}}`,
			},
			{
				// TLS 1.3 cipher names without minTLSVersion and without the HTTP/2-required
				// TLS 1.2 ECDHE-GCM ciphers. APIServer admission rejects at the API layer.
				name:  "ERR-001-C: missing minTLSVersion with TLS 1.3 ciphers only",
				value: `{"type":"Custom","custom":{"ciphers":["TLS_AES_128_GCM_SHA256","TLS_AES_256_GCM_SHA384","TLS_CHACHA20_POLY1305_SHA256"]}}`,
			},
		}

		for _, ic := range invalidCases {
			ic := ic

			It(ic.name, func() {
				ctx, cancel := context.WithTimeout(context.Background(), utils.DefaultTimeout)
				defer cancel()

				By("Reading current APIServer TLS profile type")
				profileBefore, err := utils.GetAPIServerTLSProfileType(ctx, configClient)
				Expect(err).NotTo(HaveOccurred(), "failed to read APIServer profile before invalid patch")
				fmt.Fprintf(GinkgoWriter, "profile before invalid patch: %q\n", profileBefore)

				// JSON Patch replaces the entire tlsSecurityProfile object so the
				// invalid shape reaches admission even when a valid Custom profile
				// is already active (e.g. from a prior test run).
				patchJSON := fmt.Sprintf(
					`[{"op":"replace","path":"/spec/tlsSecurityProfile","value":%s}]`,
					ic.value,
				)

				By(fmt.Sprintf("Attempting invalid Custom patch: %s", ic.name))
				_, err = configClient.APIServers().Patch(ctx, "cluster",
					types.JSONPatchType, []byte(patchJSON), metav1.PatchOptions{})
				Expect(err).To(HaveOccurred(),
					"APIServer admission must reject invalid Custom profile: %s", ic.name)
				fmt.Fprintf(GinkgoWriter, "admission rejected as expected: %v\n", err)

				By("Verifying APIServer profile is unchanged after rejected patch")
				profileAfter, readErr := utils.GetAPIServerTLSProfileType(ctx, configClient)
				Expect(readErr).NotTo(HaveOccurred(), "failed to read APIServer profile after invalid patch")
				Expect(profileAfter).To(Equal(profileBefore),
					"APIServer profile should not change after rejected patch")

				By("Verifying operator is healthy after invalid patch attempt")
				utils.AssertAllOperandsPodsReady(ctx, k8sClient, clientset)
			})
		}
	})
})

func restoreSavedAPIServerTLSProfile(saved *ocpconfigv1.TLSSecurityProfile) {
	ctx, cancel := context.WithTimeout(context.Background(), utils.TLSProfileRestoreTimeout)
	defer cancel()
	Expect(utils.RestoreAPIServerTLSSecurityProfile(ctx, configClient, saved)).To(Succeed(),
		"failed to restore original APIServer TLS profile after TLS profile case")

	minVersion, requireCiphers, _ := utils.ExpectedOperandTLSForAPIServerProfile(utils.SavedProfileType(saved))
	utils.WaitForTLSProfileRolloutComplete(ctx, k8sClient, clientset, minVersion, requireCiphers)
	utils.WaitForClusterMachinesReady(ctx, k8sClient, clientset)
}
