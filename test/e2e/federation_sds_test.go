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
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/test/e2e/utils"
	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Federation SDS E2E", Label("federation", "sds"), Ordered, func() {
	var (
		testCtx        context.Context
		cancelCtx      context.CancelFunc
		trustDomainA   string
		trustDomainB   string
		appsDomainA    string
		appsDomainB    string
		bundleRouteA   string
		bundleRouteB   string
		clusterSvcIPB  string
	)

	BeforeAll(func() {
		if os.Getenv("KUBECONFIG_CLUSTER_B") == "" {
			Skip("KUBECONFIG_CLUSTER_B not set, skipping federation tests")
		}
		Expect(k8sClientB).NotTo(BeNil(), "Cluster B client must be initialized")
		Expect(clientsetB).NotTo(BeNil(), "Cluster B clientset must be initialized")

		testCtx, cancelCtx = context.WithTimeout(context.Background(), 30*time.Minute)
		DeferCleanup(cancelCtx)

		By("Getting Cluster A apps domain")
		baseDomainA, err := utils.GetClusterBaseDomain(testCtx, configClient)
		Expect(err).NotTo(HaveOccurred(), "failed to get Cluster A base domain")
		appsDomainA = fmt.Sprintf("apps.%s", baseDomainA)
		trustDomainA = fmt.Sprintf("cluster-a.%s", appsDomainA)

		By("Getting Cluster B apps domain")
		baseDomainB, err := utils.GetClusterBaseDomain(testCtx, configClientB)
		Expect(err).NotTo(HaveOccurred(), "failed to get Cluster B base domain")
		appsDomainB = fmt.Sprintf("apps.%s", baseDomainB)
		trustDomainB = fmt.Sprintf("cluster-b.%s", appsDomainB)

		fmt.Fprintf(GinkgoWriter, "Cluster A: trustDomain=%s, appsDomain=%s\n", trustDomainA, appsDomainA)
		fmt.Fprintf(GinkgoWriter, "Cluster B: trustDomain=%s, appsDomain=%s\n", trustDomainB, appsDomainB)
	})

	Context("Infrastructure", func() {
		It("configures federation on Cluster A (ZTWIM + SpireServer with federation spec)", func() {
			By("Creating ZeroTrustWorkloadIdentityManager on Cluster A")
			utils.CreateFederationZTWIM(testCtx, k8sClient, trustDomainA, "cluster-a")

			By("Creating SpireServer with federation on Cluster A")
			utils.CreateFederationSpireServer(testCtx, k8sClient, trustDomainA, appsDomainA)

			By("Creating SpireAgent on Cluster A")
			utils.CreateFederationSpireAgent(testCtx, k8sClient)

			By("Creating SpiffeCSIDriver on Cluster A")
			utils.CreateFederationSpiffeCSIDriver(testCtx, k8sClient)
		})

		It("configures federation on Cluster B (ZTWIM + SpireServer with federation spec)", func() {
			By("Creating ZeroTrustWorkloadIdentityManager on Cluster B")
			utils.CreateFederationZTWIM(testCtx, k8sClientB, trustDomainB, "cluster-b")

			By("Creating SpireServer with federation on Cluster B")
			utils.CreateFederationSpireServer(testCtx, k8sClientB, trustDomainB, appsDomainB)

			By("Creating SpireAgent on Cluster B")
			utils.CreateFederationSpireAgent(testCtx, k8sClientB)

			By("Creating SpiffeCSIDriver on Cluster B")
			utils.CreateFederationSpiffeCSIDriver(testCtx, k8sClientB)
		})

		It("waits for SPIRE server/agent ready on both clusters", func() {
			By("Waiting for SPIRE ready on Cluster A")
			utils.WaitForSpireReady(testCtx, k8sClient, clientset, utils.FederationTimeout)

			By("Waiting for SPIRE ready on Cluster B")
			utils.WaitForSpireReady(testCtx, k8sClientB, clientsetB, utils.FederationTimeout)
		})
	})

	Context("Federation Routes", func() {
		It("creates federation routes on both clusters", func() {
			By("Waiting for federation route on Cluster A")
			Eventually(func() string {
				route := &routev1.Route{}
				err := k8sClient.Get(testCtx, types.NamespacedName{
					Name:      utils.FederationRouteName,
					Namespace: utils.OperatorNamespace,
				}, route)
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "waiting for federation route on Cluster A: %v\n", err)
					return ""
				}
				return route.Spec.Host
			}).WithTimeout(utils.ShortTimeout).WithPolling(utils.DefaultInterval).ShouldNot(BeEmpty(),
				"federation route should have a host on Cluster A")

			route := &routev1.Route{}
			Expect(k8sClient.Get(testCtx, types.NamespacedName{
				Name:      utils.FederationRouteName,
				Namespace: utils.OperatorNamespace,
			}, route)).To(Succeed())
			bundleRouteA = route.Spec.Host
			fmt.Fprintf(GinkgoWriter, "Cluster A federation route: %s\n", bundleRouteA)

			By("Waiting for federation route on Cluster B")
			Eventually(func() string {
				routeB := &routev1.Route{}
				err := k8sClientB.Get(testCtx, types.NamespacedName{
					Name:      utils.FederationRouteName,
					Namespace: utils.OperatorNamespace,
				}, routeB)
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "waiting for federation route on Cluster B: %v\n", err)
					return ""
				}
				return routeB.Spec.Host
			}).WithTimeout(utils.ShortTimeout).WithPolling(utils.DefaultInterval).ShouldNot(BeEmpty(),
				"federation route should have a host on Cluster B")

			routeB := &routev1.Route{}
			Expect(k8sClientB.Get(testCtx, types.NamespacedName{
				Name:      utils.FederationRouteName,
				Namespace: utils.OperatorNamespace,
			}, routeB)).To(Succeed())
			bundleRouteB = routeB.Spec.Host
			fmt.Fprintf(GinkgoWriter, "Cluster B federation route: %s\n", bundleRouteB)

			Expect(bundleRouteA).NotTo(BeEmpty())
			Expect(bundleRouteB).NotTo(BeEmpty())
		})
	})

	Context("Bidirectional Federation", func() {
		It("applies bidirectional ClusterFederatedTrustDomain", func() {
			By("Creating ClusterFederatedTrustDomain on Cluster A pointing to Cluster B")
			cftdA := &spiffev1alpha1.ClusterFederatedTrustDomain{
				ObjectMeta: metav1.ObjectMeta{Name: "federation-cluster-b"},
				Spec: spiffev1alpha1.ClusterFederatedTrustDomainSpec{
					TrustDomain:       trustDomainB,
					BundleEndpointURL: fmt.Sprintf("https://%s:%d", bundleRouteB, utils.FederationRoutePort),
					BundleEndpointProfile: spiffev1alpha1.BundleEndpointProfile{
						Type:             spiffev1alpha1.HTTPSSPIFFEProfileType,
						EndpointSPIFFEID: fmt.Sprintf("spiffe://%s/spire/server", trustDomainB),
					},
				},
			}
			Expect(k8sClient.Create(testCtx, cftdA)).To(Succeed(),
				"failed to create ClusterFederatedTrustDomain on Cluster A")
			DeferCleanup(func(ctx context.Context) {
				_ = k8sClient.Delete(ctx, cftdA)
			})

			By("Creating ClusterFederatedTrustDomain on Cluster B pointing to Cluster A")
			cftdB := &spiffev1alpha1.ClusterFederatedTrustDomain{
				ObjectMeta: metav1.ObjectMeta{Name: "federation-cluster-a"},
				Spec: spiffev1alpha1.ClusterFederatedTrustDomainSpec{
					TrustDomain:       trustDomainA,
					BundleEndpointURL: fmt.Sprintf("https://%s:%d", bundleRouteA, utils.FederationRoutePort),
					BundleEndpointProfile: spiffev1alpha1.BundleEndpointProfile{
						Type:             spiffev1alpha1.HTTPSSPIFFEProfileType,
						EndpointSPIFFEID: fmt.Sprintf("spiffe://%s/spire/server", trustDomainA),
					},
				},
			}
			Expect(k8sClientB.Create(testCtx, cftdB)).To(Succeed(),
				"failed to create ClusterFederatedTrustDomain on Cluster B")
			DeferCleanup(func(ctx context.Context) {
				_ = k8sClientB.Delete(ctx, cftdB)
			})
		})

		It("exchanges trust bundles over HTTPS federation", func() {
			By("Waiting for federation bundles to propagate")
			time.Sleep(utils.FederationBundlePropagation)

			By("Verifying federated bundle on Cluster A agent")
			Eventually(func() bool {
				pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{
					LabelSelector: utils.SpireAgentPodLabel,
				})
				if err != nil || len(pods.Items) == 0 {
					return false
				}
				agentPod := pods.Items[0].Name
				stdout, _, err := utils.ExecInPod(testCtx, utils.OperatorNamespace, agentPod, "spire-agent",
					[]string{"/opt/spire/bin/spire-agent", "api", "fetch", "bundle",
						"-socketPath", "unix:///tmp/spire-agent/public/spire-agent.sock"})
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "spire-agent bundle fetch on A failed: %v\n", err)
					return false
				}
				hasFederatedBundle := strings.Contains(stdout, trustDomainB)
				if !hasFederatedBundle {
					fmt.Fprintf(GinkgoWriter, "Cluster A agent does not yet have bundle from B\n")
				}
				return hasFederatedBundle
			}).WithTimeout(utils.FederationTimeout).WithPolling(30*time.Second).Should(BeTrue(),
				"Cluster A should have federated bundle from Cluster B")

			By("Verifying federated bundle on Cluster B agent")
			Eventually(func() bool {
				pods, err := clientsetB.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{
					LabelSelector: utils.SpireAgentPodLabel,
				})
				if err != nil || len(pods.Items) == 0 {
					return false
				}
				agentPod := pods.Items[0].Name
				stdout, _, err := utils.ExecInPodOnClusterB(testCtx, utils.OperatorNamespace, agentPod, "spire-agent",
					[]string{"/opt/spire/bin/spire-agent", "api", "fetch", "bundle",
						"-socketPath", "unix:///tmp/spire-agent/public/spire-agent.sock"})
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "spire-agent bundle fetch on B failed: %v\n", err)
					return false
				}
				hasFederatedBundle := strings.Contains(stdout, trustDomainA)
				if !hasFederatedBundle {
					fmt.Fprintf(GinkgoWriter, "Cluster B agent does not yet have bundle from A\n")
				}
				return hasFederatedBundle
			}).WithTimeout(utils.FederationTimeout).WithPolling(30*time.Second).Should(BeTrue(),
				"Cluster B should have federated bundle from Cluster A")
		})
	})

	Context("SDS Configuration", func() {
		It("injects expected SDS config in spire-agent ConfigMap on both clusters", func() {
			By("Checking spire-agent ConfigMap on Cluster A for SDS config")
			cmA, err := clientset.CoreV1().ConfigMaps(utils.OperatorNamespace).Get(testCtx, utils.SpireAgentConfigMapName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to get spire-agent ConfigMap on Cluster A")

			agentConfA := cmA.Data[utils.SpireAgentConfigKey]
			Expect(agentConfA).NotTo(BeEmpty(), "agent config should not be empty on Cluster A")
			Expect(agentConfA).To(ContainSubstring("default_all_bundles_name"),
				"Cluster A agent config should contain default_all_bundles_name")
			Expect(agentConfA).To(ContainSubstring("ROOTCA"),
				"Cluster A agent config default_all_bundles_name should be ROOTCA")
			fmt.Fprintf(GinkgoWriter, "[PASS] Cluster A: SDS default_all_bundles_name=ROOTCA present\n")

			By("Checking spire-agent ConfigMap on Cluster B for SDS config")
			cmB, err := clientsetB.CoreV1().ConfigMaps(utils.OperatorNamespace).Get(testCtx, utils.SpireAgentConfigMapName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to get spire-agent ConfigMap on Cluster B")

			agentConfB := cmB.Data[utils.SpireAgentConfigKey]
			Expect(agentConfB).NotTo(BeEmpty(), "agent config should not be empty on Cluster B")
			Expect(agentConfB).To(ContainSubstring("default_all_bundles_name"),
				"Cluster B agent config should contain default_all_bundles_name")
			Expect(agentConfB).To(ContainSubstring("ROOTCA"),
				"Cluster B agent config default_all_bundles_name should be ROOTCA")
			fmt.Fprintf(GinkgoWriter, "[PASS] Cluster B: SDS default_all_bundles_name=ROOTCA present\n")
		})

		It("serves ROOTCA with local and federated bundles via agent SDS", func() {
			By("Verifying default_bundle_name is null on Cluster A (routes ROOTCA to buildAll)")
			cmA, err := clientset.CoreV1().ConfigMaps(utils.OperatorNamespace).Get(testCtx, utils.SpireAgentConfigMapName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			agentConfA := cmA.Data[utils.SpireAgentConfigKey]
			Expect(agentConfA).To(ContainSubstring(`"default_bundle_name"`),
				"should have default_bundle_name key")

			By("Verifying default_bundle_name is null on Cluster B")
			cmB, err := clientsetB.CoreV1().ConfigMaps(utils.OperatorNamespace).Get(testCtx, utils.SpireAgentConfigMapName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			agentConfB := cmB.Data[utils.SpireAgentConfigKey]
			Expect(agentConfB).To(ContainSubstring(`"default_bundle_name"`),
				"should have default_bundle_name key")
		})
	})

	Context("Cross-cluster mTLS proof", func() {
		BeforeAll(func() {
			By("Setting up mTLS test namespace on Cluster B (server)")
			utils.SetupFederationNamespace(testCtx, k8sClientB, utils.MTLSTestNamespaceB, utils.MTLSServerSAName)
			DeferCleanup(func(ctx context.Context) {
				ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: utils.MTLSTestNamespaceB}}
				_ = k8sClientB.Delete(ctx, ns)
			})

			By("Creating ClusterSPIFFEID for server on Cluster B")
			utils.CreateFederationClusterSPIFFEID(testCtx, k8sClientB,
				"federation-mtls-server", utils.MTLSTestNamespaceB, utils.MTLSServerAppLabel)
			DeferCleanup(func(ctx context.Context) {
				cspiffeID := &spiffev1alpha1.ClusterSPIFFEID{ObjectMeta: metav1.ObjectMeta{Name: "federation-mtls-server"}}
				_ = k8sClientB.Delete(ctx, cspiffeID)
			})

			By("Deploying mTLS server pod on Cluster B")
			serverPod := utils.NewMTLSServerPod(utils.MTLSServerPodName, utils.MTLSTestNamespaceB, utils.MTLSServerSAName)
			Expect(k8sClientB.Create(testCtx, serverPod)).To(Succeed())
			utils.WaitForPodReady(testCtx, clientsetB, utils.MTLSServerPodName, utils.MTLSTestNamespaceB, utils.DefaultTimeout)
			utils.WaitForSVIDsReadyOnClusterB(testCtx, utils.MTLSTestNamespaceB, utils.MTLSServerPodName, "tls-server", utils.DefaultTimeout)

			By("Creating a Service for the mTLS server on Cluster B")
			svc := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mtls-server",
					Namespace: utils.MTLSTestNamespaceB,
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{"app": utils.MTLSServerAppLabel},
					Ports: []corev1.ServicePort{
						{Name: "tls", Port: int32(utils.MTLSServerPort), Protocol: corev1.ProtocolTCP},
					},
				},
			}
			Expect(k8sClientB.Create(testCtx, svc)).To(Succeed())

			By("Getting Cluster B service ClusterIP for direct access")
			Eventually(func() string {
				s := &corev1.Service{}
				if err := k8sClientB.Get(testCtx, client.ObjectKeyFromObject(svc), s); err != nil {
					return ""
				}
				return s.Spec.ClusterIP
			}).WithTimeout(utils.ShortTimeout).WithPolling(utils.ShortInterval).ShouldNot(BeEmpty())
			s := &corev1.Service{}
			Expect(k8sClientB.Get(testCtx, client.ObjectKeyFromObject(svc), s)).To(Succeed())
			clusterSvcIPB = s.Spec.ClusterIP

			By("Setting up mTLS test namespace on Cluster A (client)")
			utils.SetupFederationNamespace(testCtx, k8sClient, utils.MTLSTestNamespaceA, utils.MTLSClientSAName)
			DeferCleanup(func(ctx context.Context) {
				ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: utils.MTLSTestNamespaceA}}
				_ = k8sClient.Delete(ctx, ns)
			})

			By("Creating ClusterSPIFFEID for client on Cluster A")
			utils.CreateFederationClusterSPIFFEID(testCtx, k8sClient,
				"federation-mtls-client", utils.MTLSTestNamespaceA, utils.MTLSClientAppLabel)
			DeferCleanup(func(ctx context.Context) {
				cspiffeID := &spiffev1alpha1.ClusterSPIFFEID{ObjectMeta: metav1.ObjectMeta{Name: "federation-mtls-client"}}
				_ = k8sClient.Delete(ctx, cspiffeID)
			})

			By("Deploying mTLS client pod on Cluster A")
			clientPod := utils.NewMTLSClientPod(utils.MTLSClientPodName, utils.MTLSTestNamespaceA, utils.MTLSClientSAName)
			Expect(k8sClient.Create(testCtx, clientPod)).To(Succeed())
			utils.WaitForPodReady(testCtx, clientset, utils.MTLSClientPodName, utils.MTLSTestNamespaceA, utils.DefaultTimeout)
			utils.WaitForSVIDsReady(testCtx, utils.MTLSTestNamespaceA, utils.MTLSClientPodName, "tls-client", utils.DefaultTimeout)
		})

		It("establishes cross-cluster mTLS between client and server", func() {
			By("Attempting mTLS connection from Cluster A client to Cluster B server via route")
			serverEndpoint := bundleRouteB
			if serverEndpoint == "" {
				// Fallback: if federation route is same-cluster accessible, use service IP
				serverEndpoint = clusterSvcIPB
			}

			// The client pod on Cluster A connects to the server on Cluster B.
			// Since the clusters are separate, use the Route host (publicly accessible).
			// The Route host resolves externally and the server's SPIRE cert covers it.
			Eventually(func() error {
				stdout, stderr, err := utils.AttemptMTLSConnection(
					testCtx,
					utils.MTLSTestNamespaceA,
					utils.MTLSClientPodName,
					bundleRouteB,
					utils.FederationRoutePort,
				)
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "mTLS attempt failed: stdout=%s stderr=%s err=%v\n",
						strings.TrimSpace(stdout), strings.TrimSpace(stderr), err)
					return fmt.Errorf("mTLS connection failed: %w", err)
				}
				if strings.Contains(stdout, "verify error") || strings.Contains(stdout, "EXIT_CODE=1") {
					return fmt.Errorf("TLS verification error in output: %s", stdout)
				}
				fmt.Fprintf(GinkgoWriter, "[PASS] Cross-cluster mTLS handshake succeeded\n")
				return nil
			}).WithTimeout(utils.FederationTimeout).WithPolling(30*time.Second).Should(Succeed(),
				"cross-cluster mTLS should succeed with federated trust bundles")
		})

		It("mTLS fails when federated trust is removed (negative control)", func() {
			By("Deleting ClusterFederatedTrustDomain on Cluster A")
			cftd := &spiffev1alpha1.ClusterFederatedTrustDomain{
				ObjectMeta: metav1.ObjectMeta{Name: "federation-cluster-b"},
			}
			Expect(k8sClient.Delete(testCtx, cftd)).To(Succeed(),
				"failed to delete ClusterFederatedTrustDomain on Cluster A")

			By("Waiting for trust bundle to expire/refresh")
			time.Sleep(utils.FederationBundlePropagation)

			By("Attempting mTLS connection (should fail without federated trust)")
			// After removing the trust domain, the client should no longer trust
			// the server's certificate from Cluster B
			Eventually(func() bool {
				stdout, _, err := utils.AttemptMTLSConnection(
					testCtx,
					utils.MTLSTestNamespaceA,
					utils.MTLSClientPodName,
					bundleRouteB,
					utils.FederationRoutePort,
				)
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "[EXPECTED] mTLS connection failed as expected: %v\n", err)
					return true
				}
				if strings.Contains(stdout, "verify error") || strings.Contains(stdout, "EXIT_CODE=1") {
					fmt.Fprintf(GinkgoWriter, "[EXPECTED] TLS verification error (federation removed): %s\n", stdout)
					return true
				}
				fmt.Fprintf(GinkgoWriter, "mTLS still succeeding unexpectedly, waiting for bundle expiry...\n")
				return false
			}).WithTimeout(utils.FederationTimeout).WithPolling(30*time.Second).Should(BeTrue(),
				"mTLS should fail after federated trust is removed")

			By("Re-creating ClusterFederatedTrustDomain to restore state")
			cftdRestore := &spiffev1alpha1.ClusterFederatedTrustDomain{
				ObjectMeta: metav1.ObjectMeta{Name: "federation-cluster-b"},
				Spec: spiffev1alpha1.ClusterFederatedTrustDomainSpec{
					TrustDomain:       trustDomainB,
					BundleEndpointURL: fmt.Sprintf("https://%s:%d", bundleRouteB, utils.FederationRoutePort),
					BundleEndpointProfile: spiffev1alpha1.BundleEndpointProfile{
						Type:             spiffev1alpha1.HTTPSSPIFFEProfileType,
						EndpointSPIFFEID: fmt.Sprintf("spiffe://%s/spire/server", trustDomainB),
					},
				},
			}
			Expect(k8sClient.Create(testCtx, cftdRestore)).To(Succeed())
		})
	})
})
