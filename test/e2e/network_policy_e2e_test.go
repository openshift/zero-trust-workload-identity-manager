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
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	operatorv1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/test/e2e/utils"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("SPIRE-220 operand NetworkPolicies", Ordered, Label("network-policy"), func() {
	var testCtx context.Context
	var subscriptionName string
	var npFeatureDetected bool

	BeforeAll(func() {
		ctx := context.Background()
		var err error
		subscriptionName, _, err = utils.FindOperatorSubscription(ctx, k8sClient, utils.OperatorNamespace, utils.OperatorSubscriptionNameFragment)
		Expect(err).NotTo(HaveOccurred(), "no Subscription matching '%s' in namespace '%s'",
			utils.OperatorSubscriptionNameFragment, utils.OperatorNamespace)

		server := &operatorv1alpha1.SpireServer{}
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: utils.OperandCRName}, server); err != nil {
			Skip("SpireServer cluster CR not found — run install context first")
		}
		for _, c := range server.Status.Conditions {
			if c.Type == operatorv1alpha1.NetworkPoliciesAvailable {
				npFeatureDetected = true
				break
			}
		}
		if !npFeatureDetected {
			Skip("NetworkPoliciesAvailable not reported — deploy operator build with SPIRE-220 network policies")
		}
	})

	BeforeEach(func() {
		var cancel context.CancelFunc
		testCtx, cancel = context.WithTimeout(context.Background(), utils.TestContextTimeout)
		DeferCleanup(cancel)
	})

	It("J1: baseline ztwim-sys policies, owner references, and appliedPolicies status", func() {
		ns := utils.OperatorNamespace

		utils.WaitForOperandNetworkPoliciesCondition(testCtx, k8sClient, "SpireServer", utils.OperandCRName,
			metav1.ConditionTrue, "", utils.DefaultTimeout)
		utils.WaitForOperandNetworkPoliciesCondition(testCtx, k8sClient, "SpireAgent", utils.OperandCRName,
			metav1.ConditionTrue, "", utils.DefaultTimeout)
		utils.WaitForOperandNetworkPoliciesCondition(testCtx, k8sClient, "SpireOIDCDiscoveryProvider", utils.OperandCRName,
			metav1.ConditionTrue, "", utils.DefaultTimeout)

		serverPolicies := []string{
			utils.SpireServerNetworkPolicyIngressName,
			utils.SpireServerNetworkPolicyEgressBaselineName,
			utils.SpireServerNetworkPolicyEgressAllowAllName,
		}
		for _, name := range serverPolicies {
			np := utils.WaitForNetworkPolicy(testCtx, clientset, ns, name, utils.DefaultTimeout)
			utils.ExpectNetworkPolicyOwner(np, "SpireServer", utils.OperandCRName)
			switch name {
			case utils.SpireServerNetworkPolicyEgressBaselineName:
				Expect(utils.NetworkPolicyAllowsAllEgress(np)).To(BeFalse())
				Expect(utils.NetworkPolicyHasEgressTCPPort(np, 6443)).To(BeTrue())
			case utils.SpireServerNetworkPolicyEgressAllowAllName:
				Expect(utils.NetworkPolicyAllowsAllEgress(np)).To(BeTrue())
			}
		}

		agentPolicies := []string{
			utils.SpireAgentNetworkPolicyIngressName,
			utils.SpireAgentNetworkPolicyEgressBaselineName,
			utils.SpireAgentNetworkPolicyEgressAllowAllName,
		}
		for _, name := range agentPolicies {
			np := utils.WaitForNetworkPolicy(testCtx, clientset, ns, name, utils.DefaultTimeout)
			utils.ExpectNetworkPolicyOwner(np, "SpireAgent", utils.OperandCRName)
			switch name {
			case utils.SpireAgentNetworkPolicyEgressBaselineName:
				Expect(utils.NetworkPolicyAllowsAllEgress(np)).To(BeFalse())
				Expect(utils.NetworkPolicyHasEgressTCPPort(np, 8081)).To(BeTrue())
			case utils.SpireAgentNetworkPolicyEgressAllowAllName:
				Expect(utils.NetworkPolicyAllowsAllEgress(np)).To(BeTrue())
			}
		}

		oidcNP := utils.WaitForNetworkPolicy(testCtx, clientset, ns, utils.SpireOIDCNetworkPolicyIngressName, utils.DefaultTimeout)
		utils.ExpectNetworkPolicyOwner(oidcNP, "SpireOIDCDiscoveryProvider", utils.OperandCRName)

		server := &operatorv1alpha1.SpireServer{}
		Expect(k8sClient.Get(testCtx, client.ObjectKey{Name: utils.OperandCRName}, server)).To(Succeed())
		Expect(server.Status.NetworkPolicy).NotTo(BeNil())
		Expect(server.Status.NetworkPolicy.ManagementState).To(Equal(operatorv1alpha1.NetworkPolicyManagementManaged))
		for _, name := range serverPolicies {
			Expect(server.Status.NetworkPolicy.AppliedPolicies).To(ContainElement(name))
		}
	})

	It("J2: SVID attestation and metrics endpoints remain healthy with policies", func() {
		f := utils.SetupAttestationTest(testCtx, k8sClient, clientset, "np-svid", nil)
		_, _, _, cert, err := utils.ReadAndParseSVIDCertificate(testCtx, f.Namespace, f.PodName, f.AppContainer)
		Expect(err).NotTo(HaveOccurred())
		Expect(cert.NotAfter.After(time.Now())).To(BeTrue())

		By("Verifying SPIRE server pods are running with network policies applied")
		utils.WaitForStatefulSetReady(testCtx, clientset, utils.SpireServerStatefulSetName, utils.OperatorNamespace, utils.DefaultTimeout)

		_, err = clientset.CoreV1().Services(utils.OperatorNamespace).Get(testCtx, utils.OperatorMetricsServiceName, metav1.GetOptions{})
		if err != nil {
			Skip("operator metrics Service not present — typical for local/non-OLM operator deploy")
		}
		Eventually(func() bool {
			endpoints, err := clientset.CoreV1().Endpoints(utils.OperatorNamespace).Get(testCtx,
				utils.OperatorMetricsServiceName, metav1.GetOptions{})
			if err != nil {
				return false
			}
			for _, subset := range endpoints.Subsets {
				if len(subset.Addresses) > 0 {
					return true
				}
			}
			return false
		}).WithTimeout(utils.DefaultTimeout).WithPolling(utils.ShortInterval).Should(BeTrue())
	})

	It("J3: managedNetworkPolicy false then true on SpireServer", func() {
		server := &operatorv1alpha1.SpireServer{}
		Expect(k8sClient.Get(testCtx, client.ObjectKey{Name: utils.OperandCRName}, server)).To(Succeed())

		err := utils.UpdateCRWithRetry(testCtx, k8sClient, server, func() {
			server.Spec.ManagedNetworkPolicy = "false"
		})
		Expect(err).NotTo(HaveOccurred())

		utils.WaitForOperandNetworkPoliciesCondition(testCtx, k8sClient, "SpireServer", utils.OperandCRName,
			metav1.ConditionFalse, operatorv1alpha1.ReasonManagedNetworkPolicyDisabled, utils.DefaultTimeout)

		Eventually(func(g Gomega) {
			s := &operatorv1alpha1.SpireServer{}
			g.Expect(k8sClient.Get(testCtx, client.ObjectKey{Name: utils.OperandCRName}, s)).To(Succeed())
			g.Expect(s.Status.NetworkPolicy).NotTo(BeNil())
			g.Expect(s.Status.NetworkPolicy.ManagementState).To(Equal(operatorv1alpha1.NetworkPolicyManagementUnmanaged))
		}).WithTimeout(utils.ShortTimeout).WithPolling(utils.ShortInterval).Should(Succeed())

		err = utils.UpdateCRWithRetry(testCtx, k8sClient, server, func() {
			server.Spec.ManagedNetworkPolicy = "true"
		})
		Expect(err).NotTo(HaveOccurred())

		utils.WaitForOperandNetworkPoliciesCondition(testCtx, k8sClient, "SpireServer", utils.OperandCRName,
			metav1.ConditionTrue, "", utils.DefaultTimeout)
		utils.WaitForNetworkPolicy(testCtx, clientset, utils.OperatorNamespace, utils.SpireServerNetworkPolicyIngressName, utils.DefaultTimeout)
	})

	It("J4: ZTWIM NetworkPoliciesApplied with partial agent opt-out", func() {
		agent := &operatorv1alpha1.SpireAgent{}
		Expect(k8sClient.Get(testCtx, client.ObjectKey{Name: utils.OperandCRName}, agent)).To(Succeed())

		err := utils.UpdateCRWithRetry(testCtx, k8sClient, agent, func() {
			agent.Spec.ManagedNetworkPolicy = "false"
		})
		Expect(err).NotTo(HaveOccurred())

		utils.WaitForOperandNetworkPoliciesCondition(testCtx, k8sClient, "SpireAgent", utils.OperandCRName,
			metav1.ConditionFalse, operatorv1alpha1.ReasonManagedNetworkPolicyDisabled, utils.DefaultTimeout)

		utils.WaitForZeroTrustWorkloadIdentityManagerConditions(testCtx, k8sClient, utils.OperandCRName, map[string]metav1.ConditionStatus{
			operatorv1alpha1.NetworkPoliciesApplied: metav1.ConditionTrue,
		}, utils.DefaultTimeout)

		err = utils.UpdateCRWithRetry(testCtx, k8sClient, agent, func() {
			agent.Spec.ManagedNetworkPolicy = "true"
		})
		Expect(err).NotTo(HaveOccurred())
		utils.WaitForOperandNetworkPoliciesCondition(testCtx, k8sClient, "SpireAgent", utils.OperandCRName,
			metav1.ConditionTrue, "", utils.DefaultTimeout)
	})

	It("J5: federation network policies when federation is enabled", func() {
		server := &operatorv1alpha1.SpireServer{}
		Expect(k8sClient.Get(testCtx, client.ObjectKey{Name: utils.OperandCRName}, server)).To(Succeed())

		if server.Spec.Federation == nil {
			err := utils.UpdateCRWithRetry(testCtx, k8sClient, server, func() {
				server.Spec.Federation = &operatorv1alpha1.FederationConfig{
					BundleEndpoint: operatorv1alpha1.BundleEndpointConfig{
						Profile:     operatorv1alpha1.HttpsSpiffeProfile,
						RefreshHint: 300,
					},
					ManagedRoute: "false",
				}
			})
			Expect(err).NotTo(HaveOccurred())
		}

		utils.WaitForNetworkPolicy(testCtx, clientset, utils.OperatorNamespace, utils.SpireServerNetworkPolicyFederationIngressName, utils.DefaultTimeout)
		utils.WaitForNetworkPolicyAbsent(testCtx, clientset, utils.OperatorNamespace, utils.SpireServerNetworkPolicyFederationEgressName, utils.DefaultTimeout)

		// Federation cannot be cleared once set (CEL); NP removal on disable is covered in unit tests.
	})

	It("J6: allow-all egress baseline with postgres and vault spec (no port-scoped egress policies)", func() {
		server := &operatorv1alpha1.SpireServer{}
		Expect(k8sClient.Get(testCtx, client.ObjectKey{Name: utils.OperandCRName}, server)).To(Succeed())

		DeferCleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), utils.DefaultTimeout)
			defer cancel()
			s := &operatorv1alpha1.SpireServer{}
			if k8sClient.Get(ctx, client.ObjectKey{Name: utils.OperandCRName}, s) != nil {
				return
			}
			_ = utils.UpdateCRWithRetry(ctx, k8sClient, s, func() {
				s.Spec.Datastore.DatabaseType = "sqlite3"
				s.Spec.Datastore.ConnectionString = "/run/spire/data/datastore.sqlite3"
				s.Spec.UpstreamAuthority = nil
			})
		})

		err := utils.UpdateCRWithRetry(testCtx, k8sClient, server, func() {
			server.Spec.Datastore.DatabaseType = "postgres"
			server.Spec.Datastore.ConnectionString = "dbname=spire user=spire host=postgres.example.com port=15432 sslmode=disable"
			server.Spec.UpstreamAuthority = &operatorv1alpha1.UpstreamAuthorityConfig{
				Vault: &operatorv1alpha1.UpstreamAuthorityVault{
					VaultAddr:          "https://vault.example.com:18200",
					InsecureSkipVerify: true,
					K8sAuth: &operatorv1alpha1.VaultK8sAuthConfig{
						K8sAuthRoleName: "spire-server",
					},
				},
			}
		})
		Expect(err).NotTo(HaveOccurred())

		utils.WaitForNetworkPolicyAbsent(testCtx, clientset, utils.OperatorNamespace, utils.SpireServerNetworkPolicyDatabaseEgressName, utils.DefaultTimeout)
		utils.WaitForNetworkPolicyAbsent(testCtx, clientset, utils.OperatorNamespace, utils.SpireServerNetworkPolicyVaultEgressName, utils.DefaultTimeout)

		egressNP := utils.WaitForNetworkPolicy(testCtx, clientset, utils.OperatorNamespace, utils.SpireServerNetworkPolicyEgressAllowAllName, utils.DefaultTimeout)
		Expect(utils.NetworkPolicyAllowsAllEgress(egressNP)).To(BeTrue())
	})

	It("J7: no network policies target SPIFFE CSI driver pods", func() {
		pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{
			LabelSelector: utils.SpiffeCSIDriverPodLabel,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(pods.Items).NotTo(BeEmpty(), "CSI driver pods should exist")

		csiLabels := pods.Items[0].Labels

		npList, err := clientset.NetworkingV1().NetworkPolicies(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		for _, np := range npList.Items {
			if !strings.HasPrefix(np.Name, "ztwim-sys-") {
				continue
			}
			Expect(utils.NetworkPolicySelectsPodLabels(&np, map[string]string{
				"app.kubernetes.io/name": csiLabels["app.kubernetes.io/name"],
			})).To(BeFalse(), "policy %s should not select CSI pods", np.Name)
		}
	})

	It("J8: no port-scoped proxy egress policies (allow-all baseline covers proxy)", func() {
		if os.Getenv("RUN_PROXY_NP_E2E") != "true" {
			Skip("set RUN_PROXY_NP_E2E=true to run proxy NetworkPolicy e2e (mutates Subscription env)")
		}

		const httpProxy = "http://proxy.example.com:3128"
		const httpsProxy = "http://proxy.example.com:8443"

		DeferCleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), utils.DefaultTimeout)
			defer cancel()
			_ = utils.PatchSubscriptionEnv(ctx, k8sClient, utils.OperatorNamespace, subscriptionName, "HTTP_PROXY", "")
			_ = utils.PatchSubscriptionEnv(ctx, k8sClient, utils.OperatorNamespace, subscriptionName, "HTTPS_PROXY", "")
		})

		Expect(utils.PatchSubscriptionEnv(testCtx, k8sClient, utils.OperatorNamespace, subscriptionName, "HTTP_PROXY", httpProxy)).To(Succeed())
		Expect(utils.PatchSubscriptionEnv(testCtx, k8sClient, utils.OperatorNamespace, subscriptionName, "HTTPS_PROXY", httpsProxy)).To(Succeed())

		utils.WaitForDeploymentAvailable(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)

		utils.WaitForNetworkPolicyAbsent(testCtx, clientset, utils.OperatorNamespace, utils.SpireServerNetworkPolicyProxyEgressName, utils.DefaultTimeout)
		utils.WaitForNetworkPolicyAbsent(testCtx, clientset, utils.OperatorNamespace, utils.SpireAgentNetworkPolicyProxyEgressName, utils.DefaultTimeout)
	})

	It("NEG-001: invalid managedNetworkPolicy value is rejected", func() {
		agent := &operatorv1alpha1.SpireAgent{}
		Expect(k8sClient.Get(testCtx, client.ObjectKey{Name: utils.OperandCRName}, agent)).To(Succeed())

		err := utils.UpdateCRWithRetry(testCtx, k8sClient, agent, func() {
			agent.Spec.ManagedNetworkPolicy = "maybe"
		})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err) || strings.Contains(err.Error(), "managedNetworkPolicy")).To(BeTrue())
	})
})
