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
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	operatorv1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/test/e2e/utils"
	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Zero Trust Workload Identity Manager", Ordered, func() {
	var testCtx context.Context
	var appDomain string
	var clusterName string
	var bundleConfigMap string
	var jwtIssuer string
	var subscriptionName string
	var operatorConditionName string

	BeforeAll(func() {
		ctx := context.Background()

		By("Getting cluster base domain")
		baseDomain, err := utils.GetClusterBaseDomain(ctx, configClient)
		Expect(err).NotTo(HaveOccurred(), "failed to get cluster base domain")

		// declare shared variables for tests
		appDomain = fmt.Sprintf("apps.%s", baseDomain)
		jwtIssuer = fmt.Sprintf("https://oidc-discovery.%s", appDomain)
		clusterName = "test01"
		bundleConfigMap = "spire-bundle"

		By("Finding Subscription for the operator")
		var foundNames []string
		subscriptionName, foundNames, err = utils.FindOperatorSubscription(ctx, k8sClient, utils.OperatorNamespace, utils.OperatorSubscriptionNameFragment)
		Expect(err).NotTo(HaveOccurred(), "no Subscription matching '%s' found in namespace '%s'; found: %v",
			utils.OperatorSubscriptionNameFragment, utils.OperatorNamespace, foundNames)
		fmt.Fprintf(GinkgoWriter, "found Subscription '%s'\n", subscriptionName)

		By("Finding OperatorCondition for the operator")
		operatorConditionName, foundNames, err = utils.FindOperatorConditionName(ctx, k8sClient, utils.OperatorNamespace, utils.OperatorSubscriptionNameFragment)
		Expect(err).NotTo(HaveOccurred(), "no OperatorCondition matching '%s' found in namespace '%s'; found: %v",
			utils.OperatorSubscriptionNameFragment, utils.OperatorNamespace, foundNames)
		fmt.Fprintf(GinkgoWriter, "found OperatorCondition '%s'\n", operatorConditionName)
	})

	BeforeEach(func() {
		var cancel context.CancelFunc
		testCtx, cancel = context.WithTimeout(context.Background(), utils.TestContextTimeout)
		DeferCleanup(cancel)
	})

	Context("Installation", func() {
		It("Operator should be installed successfully", func() {
			By("Waiting for all managed CRDs to be Established")
			managedCRDs := []string{
				"zerotrustworkloadidentitymanagers.operator.openshift.io",
				"spireservers.operator.openshift.io",
				"spireagents.operator.openshift.io",
				"spiffecsidrivers.operator.openshift.io",
				"spireoidcdiscoveryproviders.operator.openshift.io",
				"clusterspiffeids.spire.spiffe.io",
				"clusterstaticentries.spire.spiffe.io",
				"clusterfederatedtrustdomains.spire.spiffe.io",
			}
			for _, crd := range managedCRDs {
				utils.WaitForCRDEstablished(testCtx, apiextClient, crd, utils.ShortTimeout)
			}

			By("Waiting for operator Deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, utils.ShortTimeout)
		})

		It("Global common configurations should be defined in ZeroTrustWorkloadIdentityManager object", func() {
			By("Creating ZeroTrustWorkloadIdentityManager object")
			ztwim := &operatorv1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: operatorv1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					BundleConfigMap: bundleConfigMap,
					TrustDomain:     appDomain,
					ClusterName:     clusterName,
				},
			}
			err := k8sClient.Create(testCtx, ztwim)
			Expect(err).NotTo(HaveOccurred(), "failed to create ZeroTrustWorkloadIdentityManager object")
		})

		It("Operator should recover from the force Pod deletion", func() {
			By("Getting operator Pod")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.OperatorLabelSelector})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())

			// record pod(s) name into a map
			oldPodNames := make(map[string]struct{})
			for _, pod := range pods.Items {
				oldPodNames[pod.Name] = struct{}{}
			}

			By("Deleting operator Pod manually")
			err = clientset.CoreV1().Pods(utils.OperatorNamespace).DeleteCollection(testCtx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: utils.OperatorLabelSelector,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for new Pod to be Running and old pod to be gone")
			Eventually(func() bool {
				newPods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.OperatorLabelSelector})
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "failed to list pods: %v\n", err)
					return false
				}

				if len(newPods.Items) == 0 {
					fmt.Fprintf(GinkgoWriter, "no pod found with label '%s' in namespace '%s'\n", utils.OperatorLabelSelector, utils.OperatorNamespace)
					return false
				}

				for _, pod := range newPods.Items {
					if _, existed := oldPodNames[pod.Name]; existed {
						fmt.Fprintf(GinkgoWriter, "old pod '%v' still exists\n", pod.Name)
						return false
					}
					if pod.Status.Phase != corev1.PodRunning {
						fmt.Fprintf(GinkgoWriter, "new pod '%v' is created but still in '%v' phase\n", pod.Name, pod.Status.Phase)
						return false
					}
				}

				return true
			}).WithTimeout(utils.ShortTimeout).WithPolling(utils.ShortInterval).Should(BeTrue(),
				"new pod should be running and old pod should be deleted successfully within %v", utils.ShortTimeout)

			By("Waiting for operator Deployment to become Available again")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, utils.ShortTimeout)
		})

		It("SPIRE Server should be installed successfully by creating a SpireServer object", func() {
			By("Creating SpireServer object")
			spireServer := &operatorv1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: operatorv1alpha1.SpireServerSpec{
					JwtIssuer:           jwtIssuer,
					CAValidity:          metav1.Duration{Duration: 24 * time.Hour},
					DefaultX509Validity: metav1.Duration{Duration: utils.DefaultX509SVIDTTL},
					DefaultJWTValidity:  metav1.Duration{Duration: 5 * time.Minute},
					CASubject: operatorv1alpha1.CASubject{
						CommonName:   appDomain,
						Country:      "US",
						Organization: "RH",
					},
					Persistence: operatorv1alpha1.Persistence{
						Size:       "1Gi",
						AccessMode: "ReadWriteOncePod",
					},
					Datastore: operatorv1alpha1.DataStore{
						DatabaseType:     "sqlite3",
						ConnectionString: "/run/spire/data/datastore.sqlite3",
						MaxOpenConns:     100,
						MaxIdleConns:     2,
						ConnMaxLifetime:  3600,
						DisableMigration: "false",
					},
				},
			}
			err := k8sClient.Create(testCtx, spireServer)
			Expect(err).NotTo(HaveOccurred(), "failed to create SpireServer object")

			By("Waiting for SpireServer conditions to be True")
			utils.WaitForSpireServerConditions(testCtx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
				"ServiceAccountAvailable":          metav1.ConditionTrue,
				"ServiceAvailable":                 metav1.ConditionTrue,
				"RBACAvailable":                    metav1.ConditionTrue,
				"ValidatingWebhookAvailable":       metav1.ConditionTrue,
				"ServerConfigMapAvailable":         metav1.ConditionTrue,
				"ControllerManagerConfigAvailable": metav1.ConditionTrue,
				"BundleConfigAvailable":            metav1.ConditionTrue,
				"StatefulSetAvailable":             metav1.ConditionTrue,
				"TTLConfigurationValid":            metav1.ConditionTrue,
				"Ready":                            metav1.ConditionTrue,
			}, utils.DefaultTimeout)

			By("Waiting for SPIRE Server StatefulSet to become Ready")
			utils.WaitForStatefulSetReady(testCtx, clientset, utils.SpireServerStatefulSetName, utils.OperatorNamespace, utils.DefaultTimeout)
		})

		It("SPIRE Agent should be installed successfully by creating a SpireAgent object", func() {
			By("Creating SpireAgent object")
			spireAgent := &operatorv1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: operatorv1alpha1.SpireAgentSpec{
					NodeAttestor: &operatorv1alpha1.NodeAttestor{
						K8sPSATEnabled: "true",
					},
					WorkloadAttestors: &operatorv1alpha1.WorkloadAttestors{
						K8sEnabled: "true",
						WorkloadAttestorsVerification: &operatorv1alpha1.WorkloadAttestorsVerification{
							Type: "auto",
						},
					},
				},
			}
			err := k8sClient.Create(testCtx, spireAgent)
			Expect(err).NotTo(HaveOccurred(), "failed to create SpireAgent object")

			By("Waiting for SpireAgent conditions to be True")
			utils.WaitForSpireAgentConditions(testCtx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
				"ServiceAccountAvailable":             metav1.ConditionTrue,
				"ServiceAvailable":                    metav1.ConditionTrue,
				"RBACAvailable":                       metav1.ConditionTrue,
				"ConfigMapAvailable":                  metav1.ConditionTrue,
				"SecurityContextConstraintsAvailable": metav1.ConditionTrue,
				"DaemonSetAvailable":                  metav1.ConditionTrue,
				"Ready":                               metav1.ConditionTrue,
			}, utils.DefaultTimeout)

			By("Waiting for SPIRE Agent DaemonSet to become Available")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)
		})

		It("SPIFFE CSI Driver should be installed successfully by creating a SpiffeCSIDriver object", func() {
			By("Creating SpiffeCSIDriver object")
			spiffeCSIDriver := &operatorv1alpha1.SpiffeCSIDriver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: operatorv1alpha1.SpiffeCSIDriverSpec{},
			}
			err := k8sClient.Create(testCtx, spiffeCSIDriver)
			Expect(err).NotTo(HaveOccurred(), "failed to create SpiffeCSIDriver object")

			By("Waiting for SpiffeCSIDriver conditions to be True")
			utils.WaitForSpiffeCSIDriverConditions(testCtx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
				"ServiceAccountAvailable":             metav1.ConditionTrue,
				"CSIDriverAvailable":                  metav1.ConditionTrue,
				"SecurityContextConstraintsAvailable": metav1.ConditionTrue,
				"DaemonSetAvailable":                  metav1.ConditionTrue,
				"Ready":                               metav1.ConditionTrue,
			}, utils.DefaultTimeout)

			By("Waiting for SPIFFE CSI Driver DaemonSet to become Available")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpiffeCSIDriverDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)
		})

		It("SPIRE OIDC Discovery Provider should be installed successfully by creating a SpireOIDCDiscoveryProvider object", func() {
			By("Creating SpireOIDCDiscoveryProvider object")
			spireOIDCDiscoveryProvider := &operatorv1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: operatorv1alpha1.SpireOIDCDiscoveryProviderSpec{
					JwtIssuer: jwtIssuer,
				},
			}
			err := k8sClient.Create(testCtx, spireOIDCDiscoveryProvider)
			Expect(err).NotTo(HaveOccurred(), "failed to create SpireOIDCDiscoveryProvider object")

			By("Waiting for SpireOIDCDiscoveryProvider conditions to be True")
			utils.WaitForSpireOIDCDiscoveryProviderConditions(testCtx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
				"ServiceAccountAvailable":  metav1.ConditionTrue,
				"ServiceAvailable":         metav1.ConditionTrue,
				"ClusterSPIFFEIDAvailable": metav1.ConditionTrue,
				"ConfigMapAvailable":       metav1.ConditionTrue,
				"DeploymentAvailable":      metav1.ConditionTrue,
				"RouteAvailable":           metav1.ConditionTrue,
				"Ready":                    metav1.ConditionTrue,
			}, utils.DefaultTimeout)

			By("Waiting for SPIRE OIDC Discovery Provider Deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.SpireOIDCDiscoveryProviderDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)
		})

		It("ZeroTrustWorkloadIdentityManager should aggregate status from all operands", func() {
			By("Waiting for ZeroTrustWorkloadIdentityManager to show all operands available")
			utils.WaitForZeroTrustWorkloadIdentityManagerConditions(testCtx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
				"OperandsAvailable": metav1.ConditionTrue,
				"Ready":             metav1.ConditionTrue,
			}, utils.DefaultTimeout)

			By("Verifying ZeroTrustWorkloadIdentityManager operand status")
			cr := &operatorv1alpha1.ZeroTrustWorkloadIdentityManager{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, cr)
			Expect(err).NotTo(HaveOccurred(), "failed to get ZeroTrustWorkloadIdentityManager")

			// Should have 4 operands
			Expect(cr.Status.Operands).To(HaveLen(4), "should have 4 operands")

			// Check each operand is ready
			operandMap := make(map[string]operatorv1alpha1.OperandStatus)
			for _, operand := range cr.Status.Operands {
				operandMap[operand.Kind] = operand
			}

			requiredOperands := []string{"SpireServer", "SpireAgent", "SpiffeCSIDriver", "SpireOIDCDiscoveryProvider"}
			for _, kind := range requiredOperands {
				operand, exists := operandMap[kind]
				Expect(exists).To(BeTrue(), "%s operand should exist in status", kind)
				Expect(operand.Ready).To(Equal("true"), "%s should be ready", kind)
				Expect(operand.Message).To(Equal(operatorv1alpha1.ReasonReady), "%s message should be 'Ready'", kind)
				fmt.Fprintf(GinkgoWriter, "Operand %s is ready\n", kind)
			}
		})

		It("SPIRE Agent process should run as root with supplemental GID within namespace range", func() {
			By("Getting a ready spire-agent pod")
			agentPod, err := utils.GetReadySpireAgentPod(testCtx, clientset)
			Expect(err).NotTo(HaveOccurred(), "should find a ready spire-agent pod")
			fmt.Fprintf(GinkgoWriter, "using spire-agent pod: %s\n", agentPod.Name)

			By("Executing grep on /proc/self/status in spire-agent container")
			effectiveUID, gids, err := utils.GetProcStatusFromPod(testCtx, utils.OperatorNamespace,
				agentPod.Name, utils.SpireAgentContainerName)
			Expect(err).NotTo(HaveOccurred(), "failed to get proc status from spire-agent pod")
			fmt.Fprintf(GinkgoWriter, "spire-agent process: effectiveUID=%d, Groups=%v\n", effectiveUID, gids)

			By("Verifying process runs as root (UID 0)")
			Expect(effectiveUID).To(Equal(0), "spire-agent process should run as root (UID 0)")

			By("Verifying process has root group (GID 0)")
			hasRootGroup := false
			for _, g := range gids {
				if g == 0 {
					hasRootGroup = true
					break
				}
			}
			Expect(hasRootGroup).To(BeTrue(), "spire-agent process should have root group (GID 0) in Groups")

			By("Fetching namespace supplemental-groups annotation")
			operatorNS := &corev1.Namespace{}
			Expect(k8sClient.Get(testCtx, types.NamespacedName{Name: utils.OperatorNamespace}, operatorNS)).To(Succeed(),
				"failed to fetch operator namespace %s", utils.OperatorNamespace)

			ann := operatorNS.Annotations["openshift.io/sa.scc.supplemental-groups"]
			Expect(strings.TrimSpace(ann)).NotTo(BeEmpty(),
				"namespace %s should have supplemental groups annotation", utils.OperatorNamespace)

			rangeStart, rangeSize, err := utils.ParseSupplementalRange(ann)
			Expect(err).NotTo(HaveOccurred(), "failed to parse supplemental groups annotation %q", ann)

			By("Verifying process has supplemental GID within namespace range")
			rangeEnd := rangeStart + rangeSize
			hasSupplementalGID := false
			for _, g := range gids {
				if g >= rangeStart && g < rangeEnd {
					hasSupplementalGID = true
					fmt.Fprintf(GinkgoWriter, "found supplemental GID %d within namespace range [%d, %d)\n", g, rangeStart, rangeEnd)
					break
				}
			}
			Expect(hasSupplementalGID).To(BeTrue(),
				"spire-agent process should have at least one supplemental GID within namespace range [%d, %d); actual Groups=%v",
				rangeStart, rangeEnd, gids)
		})

		It("Prometheus metrics endpoint should be serving", func() {
			By("Verifying metrics Service exists with correct port configuration")
			metricsSvc, err := clientset.CoreV1().Services(utils.OperatorNamespace).Get(testCtx,
				utils.OperatorMetricsServiceName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "metrics Service %q must exist", utils.OperatorMetricsServiceName)

			var foundMetricsPort bool
			for _, port := range metricsSvc.Spec.Ports {
				if port.Name == utils.OperatorMetricsPortName && port.Port == int32(utils.OperatorMetricsPort) {
					foundMetricsPort = true
					break
				}
			}
			Expect(foundMetricsPort).To(BeTrue(),
				"metrics Service must have port %q on %d", utils.OperatorMetricsPortName, utils.OperatorMetricsPort)
			fmt.Fprintf(GinkgoWriter, "metrics Service %q has correct port configuration\n", utils.OperatorMetricsServiceName)

			By("Verifying metrics endpoint has ready addresses")
			Eventually(func() bool {
				endpoints, err := clientset.CoreV1().Endpoints(utils.OperatorNamespace).Get(testCtx,
					utils.OperatorMetricsServiceName, metav1.GetOptions{})
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "failed to get metrics Endpoints: %v\n", err)
					return false
				}
				for _, subset := range endpoints.Subsets {
					if len(subset.Addresses) > 0 {
						fmt.Fprintf(GinkgoWriter, "metrics endpoint has %d ready address(es)\n", len(subset.Addresses))
						return true
					}
				}
				return false
			}).WithTimeout(utils.ShortTimeout).WithPolling(utils.ShortInterval).Should(BeTrue(),
				"metrics endpoint must have at least one ready address")

			By("Scraping /metrics from operator pod via bearer token")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{
				LabelSelector: utils.OperatorLabelSelector,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty(), "operator pod must exist")
			operatorPod := pods.Items[0].Name

			stdout, stderr, err := utils.ExecInPod(testCtx, utils.OperatorNamespace, operatorPod,
				pods.Items[0].Spec.Containers[0].Name,
				[]string{"sh", "-c",
					fmt.Sprintf(`TOKEN="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" && curl -sk -H "Authorization: Bearer ${TOKEN}" https://localhost:%d/metrics`, utils.OperatorMetricsPort)},
			)
			if err != nil {
				fmt.Fprintf(GinkgoWriter, "curl via exec failed (stderr: %s): %v, falling back to Service check only\n",
					strings.TrimSpace(stderr), err)
			} else {
				By("Verifying Prometheus metrics response contains expected controller-runtime metrics")
				Expect(stdout).To(ContainSubstring("controller_runtime_reconcile_total"),
					"metrics response must contain controller_runtime_reconcile_total")
				Expect(stdout).To(ContainSubstring("workqueue_depth"),
					"metrics response must contain workqueue_depth")
				fmt.Fprintf(GinkgoWriter, "metrics endpoint returned valid Prometheus metrics\n")
			}
		})
	})

	Context("OperatorCondition", func() {
		It("Upgradeable should be True when all operands are ready", func() {
			By("Verifying Upgradeable condition details")
			condition, err := utils.GetUpgradeableCondition(testCtx, k8sClient, utils.OperatorNamespace, operatorConditionName)
			Expect(err).NotTo(HaveOccurred())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue), "Upgradeable should be %s", metav1.ConditionTrue)
			Expect(condition.Reason).To(Equal(operatorv1alpha1.ReasonReady), "Upgradeable reason should be %s", operatorv1alpha1.ReasonReady)
			fmt.Fprintf(GinkgoWriter, "Upgradeable condition is correctly set: Status=%s, Reason=%s\n", condition.Status, condition.Reason)
		})

		It("Upgradeable should be False when SPIRE Server pod is deleted and recover to True after recovery", func() {
			By("Getting SPIRE Server pod")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireServerPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty(), "no SPIRE Server pods found")
			spireServerPod := pods.Items[0]
			fmt.Fprintf(GinkgoWriter, "will delete SPIRE Server pod '%s'\n", spireServerPod.Name)

			By("Deleting SPIRE Server pod")
			err = clientset.CoreV1().Pods(utils.OperatorNamespace).Delete(testCtx, spireServerPod.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to delete SPIRE Server pod")

			By("Waiting for Upgradeable condition to transition to False")
			utils.WaitForUpgradeableStatus(testCtx, k8sClient, utils.OperatorNamespace, operatorConditionName, metav1.ConditionFalse, utils.ShortTimeout)

			By("Waiting for SPIRE Server to recover")
			utils.WaitForStatefulSetReady(testCtx, clientset, utils.SpireServerStatefulSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying Upgradeable condition returns to True after recovery")
			utils.WaitForUpgradeableStatus(testCtx, k8sClient, utils.OperatorNamespace, operatorConditionName, metav1.ConditionTrue, utils.ShortTimeout)
		})

		It("Upgradeable should be False when multiple concurrent pod failures and recover to True after recovery", func() {
			By("Getting random SPIRE Agent and SPIFFE CSI Driver pods")
			agentPods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireAgentPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(agentPods.Items).NotTo(BeEmpty(), "no SPIRE Agent pods found")

			csiPods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpiffeCSIDriverPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(csiPods.Items).NotTo(BeEmpty(), "no SPIFFE CSI Driver pods found")

			By("Deleting selected SPIRE Agent and SPIFFE CSI Driver pods simultaneously")
			err = clientset.CoreV1().Pods(utils.OperatorNamespace).Delete(testCtx, agentPods.Items[0].Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
			err = clientset.CoreV1().Pods(utils.OperatorNamespace).Delete(testCtx, csiPods.Items[0].Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
			fmt.Fprintf(GinkgoWriter, "deleted pods: %s, %s\n", agentPods.Items[0].Name, csiPods.Items[0].Name)

			By("Waiting for Upgradeable condition to transition to False")
			utils.WaitForUpgradeableStatus(testCtx, k8sClient, utils.OperatorNamespace, operatorConditionName, metav1.ConditionFalse, utils.ShortTimeout)

			By("Waiting for SPIRE Agent and SPIFFE CSI Driver to recover")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpiffeCSIDriverDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying Upgradeable condition returns to True after recovery")
			utils.WaitForUpgradeableStatus(testCtx, k8sClient, utils.OperatorNamespace, operatorConditionName, metav1.ConditionTrue, utils.ShortTimeout)
		})
	})

	Context("SpireAgent attestation", func() {
		Context("SVID validation", func() {
			var f utils.AttestationFixture

			BeforeAll(func() {
				setupCtx, cancel := context.WithTimeout(context.Background(), utils.TestContextTimeout)
				DeferCleanup(cancel)
				f = utils.SetupAttestationTest(setupCtx, k8sClient, clientset, "svid-validation", nil)
			})

			It("svid.pem should be a valid X.509 certificate", Label("attestation"), func() {
				By("Reading and parsing svid.pem")
				_, block, rest, cert, err := utils.ReadAndParseSVIDCertificate(testCtx, f.Namespace, f.PodName, f.AppContainer)
				Expect(err).NotTo(HaveOccurred(), "failed to read and parse svid.pem from pod")

				By("Validating PEM encoding")
				Expect(block).NotTo(BeNil(), "svid.pem must contain a valid PEM block")
				Expect(block.Type).To(Equal("CERTIFICATE"), "PEM block type must be CERTIFICATE")
				Expect(rest).To(Satisfy(func(b []byte) bool {
					trimmed := strings.TrimSpace(string(b))
					if len(trimmed) == 0 {
						return true
					}
					_, _, _, parseErr := utils.ParsePEMCertificate(trimmed)
					return parseErr == nil
				}), "remaining data after first PEM block must be empty or another valid PEM block")

				By("Parsing X.509 certificate")
				Expect(cert.NotBefore.Before(time.Now())).To(BeTrue(),
					"certificate NotBefore (%s) must be in the past", cert.NotBefore)
				Expect(cert.NotAfter.After(time.Now())).To(BeTrue(),
					"certificate NotAfter (%s) must be in the future", cert.NotAfter)
			})

			It("SVID SPIFFE ID should match the expected value from ClusterSPIFFEID template", Label("attestation"), func() {
				By("Reading and parsing svid.pem")
				_, _, _, cert, err := utils.ReadAndParseSVIDCertificate(testCtx, f.Namespace, f.PodName, f.AppContainer)
				Expect(err).NotTo(HaveOccurred(), "failed to read and parse svid.pem from pod")

				By("Verifying SPIFFE ID in URI SAN")
				expectedSPIFFEID := fmt.Sprintf("spiffe://%s/ns/%s/sa/%s", appDomain, f.Namespace, f.SAName)
				Expect(cert.URIs).NotTo(BeEmpty(), "certificate must contain at least one URI SAN")
				Expect(cert.URIs[0].String()).To(Equal(expectedSPIFFEID),
					"SVID SPIFFE ID must match expected value")
			})

			It("SVID expiry time should be within valid TTL bounds", Label("attestation"), func() {
				By("Reading and parsing svid.pem")
				_, _, _, cert, err := utils.ReadAndParseSVIDCertificate(testCtx, f.Namespace, f.PodName, f.AppContainer)
				Expect(err).NotTo(HaveOccurred(), "failed to read and parse svid.pem from pod")

				By("Verifying TTL is within valid bounds")
				ttl := cert.NotAfter.Sub(cert.NotBefore)
				fmt.Fprintf(GinkgoWriter, "SVID TTL: %s (NotBefore=%s, NotAfter=%s)\n", ttl, cert.NotBefore, cert.NotAfter)
				Expect(ttl).To(BeNumerically(">", 0), "TTL must be positive")
				Expect(ttl).To(BeNumerically("<=", utils.DefaultX509SVIDTTL+utils.ClockSkewTolerance),
					"TTL must not exceed configured DefaultX509Validity (%s) plus clock skew tolerance (%s)",
					utils.DefaultX509SVIDTTL, utils.ClockSkewTolerance)
				Expect(cert.NotAfter.After(time.Now())).To(BeTrue(), "certificate must not be expired")
			})

			It("bundle.pem should be a valid CA and svid.pem should chain to it", Label("attestation"), func() {
				By("Reading and parsing bundle.pem")
				bundlePEM, err := utils.ReadBundlePEM(testCtx, f.Namespace, f.PodName, f.AppContainer)
				Expect(err).NotTo(HaveOccurred(), "failed to read bundle.pem from pod")

				bundleCerts, err := utils.ParseAllPEMCertificates(bundlePEM)
				Expect(err).NotTo(HaveOccurred(), "failed to parse bundle.pem certificates")
				Expect(bundleCerts).NotTo(BeEmpty(), "bundle.pem must contain at least one certificate")

				By("Verifying bundle certificates are CAs with CertSign KeyUsage")
				for i, ca := range bundleCerts {
					Expect(ca.IsCA).To(BeTrue(), "bundle certificate [%d] must be a CA", i)
					Expect(ca.KeyUsage & x509.KeyUsageCertSign).NotTo(BeZero(),
						"bundle certificate [%d] KeyUsage must include CertSign", i)
					fmt.Fprintf(GinkgoWriter, "bundle cert [%d]: Subject=%s, IsCA=%v, KeyUsage=%d\n",
						i, ca.Subject, ca.IsCA, ca.KeyUsage)
				}

				By("Building trust pool from bundle.pem")
				rootPool := x509.NewCertPool()
				for _, ca := range bundleCerts {
					rootPool.AddCert(ca)
				}

				By("Reading and parsing svid.pem")
				_, _, _, svidCert, err := utils.ReadAndParseSVIDCertificate(testCtx, f.Namespace, f.PodName, f.AppContainer)
				Expect(err).NotTo(HaveOccurred(), "failed to read and parse svid.pem from pod")

				By("Verifying svid.pem chains to bundle.pem")
				_, err = svidCert.Verify(x509.VerifyOptions{
					Roots:     rootPool,
					KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
				})
				Expect(err).NotTo(HaveOccurred(), "svid.pem must chain to bundle.pem")
			})
		})

		It("SVID should be rotated before expiry", Label("attestation"), func() {
			f := utils.SetupAttestationTest(testCtx, k8sClient, clientset, "svid-rotation", func(spec *spiffev1alpha1.ClusterSPIFFEIDSpec) {
				spec.TTL = metav1.Duration{Duration: 60 * time.Second}
			})

			By("Recording initial certificate serial number")
			_, _, _, initialCert, err := utils.ReadAndParseSVIDCertificate(testCtx, f.Namespace, f.PodName, f.AppContainer)
			Expect(err).NotTo(HaveOccurred(), "failed to read and parse svid.pem from pod")
			initialSerial := initialCert.SerialNumber.String()
			fmt.Fprintf(GinkgoWriter, "initial SVID serial: %s\n", initialSerial)

			By("Waiting for SVID rotation (serial number change)")
			var rotatedCert *x509.Certificate
			Eventually(func() string {
				_, _, _, c, parseErr := utils.ReadAndParseSVIDCertificate(testCtx, f.Namespace, f.PodName, f.AppContainer)
				if parseErr != nil {
					return initialSerial
				}
				rotatedCert = c
				return c.SerialNumber.String()
			}).WithTimeout(utils.DefaultTimeout).WithPolling(utils.DefaultInterval).ShouldNot(Equal(initialSerial),
				"SVID serial number must change after rotation")

			By("Verifying rotated certificate is still valid")
			Expect(rotatedCert).NotTo(BeNil(), "rotated certificate must have been captured")
			Expect(rotatedCert.NotBefore.Before(time.Now())).To(BeTrue(), "rotated cert NotBefore must be in the past")
			Expect(rotatedCert.NotAfter.After(time.Now())).To(BeTrue(), "rotated cert NotAfter must be in the future")

			By("Verifying rotated certificate is genuinely newer than the initial one")
			Expect(rotatedCert.NotBefore.After(initialCert.NotBefore)).To(BeTrue(),
				"rotated cert must have a later NotBefore than initial cert")
			fmt.Fprintf(GinkgoWriter, "rotated SVID serial: %s\n", rotatedCert.SerialNumber.String())
		})

		It("should issue a fresh SVID after pod deletion and re-creation", Label("attestation"), func() {
			By("Setting up attestation environment")
			f := utils.SetupAttestationTest(testCtx, k8sClient, clientset, "pod-recreate", nil)

			By("Recording initial SVID serial number")
			_, _, _, initialCert, err := utils.ReadAndParseSVIDCertificate(testCtx, f.Namespace, f.PodName, f.AppContainer)
			Expect(err).NotTo(HaveOccurred(), "failed to read initial svid.pem")
			initialSerial := initialCert.SerialNumber.String()
			fmt.Fprintf(GinkgoWriter, "initial SVID serial: %s\n", initialSerial)

			By("Deleting the attestation pod")
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: f.PodName, Namespace: f.Namespace}}
			Expect(k8sClient.Delete(testCtx, pod)).To(Succeed(), "failed to delete attestation pod")

			By("Waiting for pod to be fully removed")
			Eventually(func() bool {
				err := k8sClient.Get(testCtx, types.NamespacedName{Name: f.PodName, Namespace: f.Namespace}, &corev1.Pod{})
				if apierrors.IsNotFound(err) {
					return true
				}
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "transient error while waiting for pod %s/%s deletion: %v\n", f.Namespace, f.PodName, err)
				}
				return false
			}).WithTimeout(utils.ShortTimeout).WithPolling(utils.ShortInterval).Should(BeTrue(),
				"old pod %s/%s should be fully removed", f.Namespace, f.PodName)

			By("Creating a new pod with the same configuration")
			newPodName := f.PodName + "-new"
			newPod := utils.NewAttestationPod(newPodName, f.Namespace, f.SAName, f.AppContainer, "pod-recreate")
			Expect(k8sClient.Create(testCtx, newPod)).To(Succeed(), "failed to create new attestation pod")

			By("Waiting for new pod to become ready")
			utils.WaitForPodReady(testCtx, clientset, newPodName, f.Namespace, 3*utils.ShortTimeout)

			By("Waiting for SVID files to appear in new pod")
			Eventually(func() string {
				stdout, _, err := utils.ExecInPod(testCtx, f.Namespace, newPodName, f.AppContainer, []string{"ls", "/certs/"})
				if err != nil {
					return ""
				}
				return stdout
			}).WithTimeout(utils.DefaultTimeout).WithPolling(utils.DefaultInterval).Should(
				And(ContainSubstring("svid.pem"), ContainSubstring("svid_key.pem"), ContainSubstring("bundle.pem")))

			By("Verifying new pod received a fresh SVID with a different serial number")
			_, _, _, newCert, err := utils.ReadAndParseSVIDCertificate(testCtx, f.Namespace, newPodName, f.AppContainer)
			Expect(err).NotTo(HaveOccurred(), "failed to read svid.pem from new pod")
			newSerial := newCert.SerialNumber.String()
			fmt.Fprintf(GinkgoWriter, "new SVID serial: %s\n", newSerial)
			Expect(newSerial).NotTo(Equal(initialSerial),
				"re-created pod must receive a fresh SVID with a different serial number")

			By("Verifying new SVID is currently valid")
			Expect(newCert.NotBefore.Before(time.Now())).To(BeTrue(), "new cert NotBefore must be in the past")
			Expect(newCert.NotAfter.After(time.Now())).To(BeTrue(), "new cert NotAfter must be in the future")
		})

		It("should propagate ClusterSPIFFEID template update to workload SVID", Label("attestation"), func() {
			By("Setting up attestation environment")
			f := utils.SetupAttestationTest(testCtx, k8sClient, clientset, "spiffeid-update", nil)

			By("Recording initial SPIFFE ID from svid.pem")
			_, _, _, initialCert, err := utils.ReadAndParseSVIDCertificate(testCtx, f.Namespace, f.PodName, f.AppContainer)
			Expect(err).NotTo(HaveOccurred(), "failed to read and parse initial svid.pem")
			Expect(initialCert.URIs).NotTo(BeEmpty(), "initial certificate must contain a URI SAN")
			initialSPIFFEID := initialCert.URIs[0].String()
			fmt.Fprintf(GinkgoWriter, "initial SPIFFE ID: %s\n", initialSPIFFEID)

			By("Updating ClusterSPIFFEID template to include /updated/ path segment")
			updatedTemplate := "spiffe://{{ .TrustDomain }}/updated/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}"
			cspiffeID := &spiffev1alpha1.ClusterSPIFFEID{}
			cspiffeID.Name = f.ClusterSPIFFEIDName
			err = utils.UpdateCRWithRetry(testCtx, k8sClient, cspiffeID, func() {
				cspiffeID.Spec.SPIFFEIDTemplate = updatedTemplate
			})
			Expect(err).NotTo(HaveOccurred(), "failed to update ClusterSPIFFEID template")

			By("Waiting for SVID to reflect the updated SPIFFE ID")
			expectedNewSPIFFEID := fmt.Sprintf("spiffe://%s/updated/ns/%s/sa/%s", appDomain, f.Namespace, f.SAName)
			Eventually(func() string {
				_, _, _, cert, parseErr := utils.ReadAndParseSVIDCertificate(testCtx, f.Namespace, f.PodName, f.AppContainer)
				if parseErr != nil || len(cert.URIs) == 0 {
					return initialSPIFFEID
				}
				return cert.URIs[0].String()
			}).WithTimeout(utils.DefaultTimeout).WithPolling(utils.DefaultInterval).Should(
				Equal(expectedNewSPIFFEID),
				"SVID SPIFFE ID must update after ClusterSPIFFEID template change",
			)
			fmt.Fprintf(GinkgoWriter, "updated SPIFFE ID: %s\n", expectedNewSPIFFEID)
		})

	})

	Context("Common configurations", func() {
		It("Operator log level can be configured through Subscription", func() {
			By("Retrieving initial log level from operator Deployment")
			initialLogLevel, err := utils.GetDeploymentEnvVar(testCtx, clientset, utils.OperatorNamespace, utils.OperatorDeploymentName, utils.OperatorLogLevelEnvVar)
			Expect(err).NotTo(HaveOccurred(), "failed to get operator Deployment env var")
			fmt.Fprintf(GinkgoWriter, "initial log level from Deployment: %s\n", initialLogLevel)

			// record initial generation of the Deployment before patching Subscription
			deployment, err := clientset.AppsV1().Deployments(utils.OperatorNamespace).Get(testCtx, utils.OperatorDeploymentName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to get operator Deployment")
			initialGen := deployment.Generation

			By("Patching Subscription object with verbose log level")
			newLogLevel := "4"
			err = utils.PatchSubscriptionEnv(testCtx, k8sClient, utils.OperatorNamespace, subscriptionName, utils.OperatorLogLevelEnvVar, newLogLevel)
			Expect(err).NotTo(HaveOccurred(), "failed to patch Subscription with env %s=%s", utils.OperatorLogLevelEnvVar, newLogLevel)
			DeferCleanup(func(ctx context.Context) {
				By("Resetting operator log level")
				utils.PatchSubscriptionEnv(ctx, k8sClient, utils.OperatorNamespace, subscriptionName, utils.OperatorLogLevelEnvVar, initialLogLevel)
			})

			By("Waiting for operator Deployment rolling update to start")
			utils.WaitForDeploymentRollingUpdate(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, initialGen, utils.DefaultTimeout)

			By("Waiting for operator Deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if operator Deployment has the expected log level")
			logLevel, err := utils.GetDeploymentEnvVar(testCtx, clientset, utils.OperatorNamespace, utils.OperatorDeploymentName, utils.OperatorLogLevelEnvVar)
			Expect(err).NotTo(HaveOccurred(), "failed to get env %s from Deployment", utils.OperatorLogLevelEnvVar)
			Expect(logLevel).To(Equal(newLogLevel), "%s should be updated to %s", utils.OperatorLogLevelEnvVar, newLogLevel)
		})

		It("SPIRE Server containers resource limits and requests can be configured through CR", func() {
			By("Getting SpireServer object")
			spireServer := &operatorv1alpha1.SpireServer{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireServer)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireServer object")

			// record initial generation of the StatefulSet before updating SpireServer object
			statefulset, err := clientset.AppsV1().StatefulSets(utils.OperatorNamespace).Get(testCtx, utils.SpireServerStatefulSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := statefulset.Generation

			By("Patching SpireServer object with resource specifications")
			expectedResources := &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			}

			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireServer, func() {
				spireServer.Spec.Resources = expectedResources
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireServer object with resources")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpireServer resources modification")
				server := &operatorv1alpha1.SpireServer{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, server); err == nil {
					server.Spec.Resources = nil
					k8sClient.Update(ctx, server)
				}
			})

			By("Waiting for SPIRE Server StatefulSet rolling update to start")
			utils.WaitForStatefulSetRollingUpdate(testCtx, clientset, utils.SpireServerStatefulSetName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for SPIRE Server StatefulSet to become Ready")
			utils.WaitForStatefulSetReady(testCtx, clientset, utils.SpireServerStatefulSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIRE Server Pods have the expected resource limits and requests")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireServerPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			utils.VerifyContainerResources(pods.Items, expectedResources)
		})

		It("SPIRE Server nodeSelector and tolerations can be configured through CR", func() {
			By("Getting current SPIRE Server Pod and its Node")
			currentPods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireServerPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(currentPods.Items).NotTo(BeEmpty(), "no SPIRE Server pods found")
			currentNodeName := currentPods.Items[0].Spec.NodeName
			Expect(currentNodeName).NotTo(BeEmpty(), "SPIRE Server pod should be scheduled to a node")
			fmt.Fprintf(GinkgoWriter, "SPIRE Server pod '%s' is on node '%s'\n", currentPods.Items[0].Name, currentNodeName)

			By("Getting SpireServer object")
			spireServer := &operatorv1alpha1.SpireServer{}
			err = k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireServer)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireServer object")

			// record initial generation of the StatefulSet before updating SpireServer object
			statefulset, err := clientset.AppsV1().StatefulSets(utils.OperatorNamespace).Get(testCtx, utils.SpireServerStatefulSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := statefulset.Generation

			By("Patching SpireServer object with nodeSelector and tolerations targeting the current Node")
			// Target the current node by hostname to avoid cross-AZ PVC re-attachment issues
			// (SPIRE Server uses ReadWriteOncePod PVC that is bound to a specific AZ).
			controlPlaneRoleKey := utils.InferControlPlaneRoleKey(testCtx, clientset)
			expectedNodeSelector := map[string]string{
				"kubernetes.io/hostname": currentNodeName,
			}
			expectedToleration := []*corev1.Toleration{
				{
					Key:      controlPlaneRoleKey,
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				},
			}

			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireServer, func() {
				spireServer.Spec.NodeSelector = expectedNodeSelector
				spireServer.Spec.Tolerations = expectedToleration
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireServer object with nodeSelector and tolerations")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpireServer nodeSelector and tolerations modification")
				server := &operatorv1alpha1.SpireServer{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, server); err == nil {
					server.Spec.NodeSelector = nil
					server.Spec.Tolerations = nil
					k8sClient.Update(ctx, server)
				}
			})

			By("Waiting for SPIRE Server StatefulSet rolling update to start")
			utils.WaitForStatefulSetRollingUpdate(testCtx, clientset, utils.SpireServerStatefulSetName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for SPIRE Server StatefulSet to become Ready")
			utils.WaitForStatefulSetReady(testCtx, clientset, utils.SpireServerStatefulSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIRE Server Pods have been scheduled to Nodes with required labels")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireServerPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			utils.VerifyPodScheduling(testCtx, clientset, pods.Items, expectedNodeSelector)

			By("Verifying if SPIRE Server Pods tolerate Node taints correctly")
			utils.VerifyPodTolerations(testCtx, clientset, pods.Items, expectedToleration)
		})

		It("SPIRE Server affinity can be configured through CR", func() {
			By("Getting current SPIRE Server Pod and its Node")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireServerPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			currentNodeName := pods.Items[0].Spec.NodeName
			Expect(currentNodeName).NotTo(BeEmpty(), "SPIRE Server pod should be scheduled to a node")
			fmt.Fprintf(GinkgoWriter, "pod '%s' is currently on node '%s'\n", pods.Items[0].Name, currentNodeName)

			By("Getting SpireServer object")
			spireServer := &operatorv1alpha1.SpireServer{}
			err = k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireServer)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireServer object")

			statefulset, err := clientset.AppsV1().StatefulSets(utils.OperatorNamespace).Get(testCtx, utils.SpireServerStatefulSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := statefulset.Generation

			By("Patching SpireServer object with NodeAffinity targeting the current Node")
			// Target the current node to avoid EBS PVC detach/re-attach delays.
			// SPIRE Server uses ReadWriteOncePod PVC; moving to any other node triggers
			// an EBS volume detach/attach cycle that can take unpredictable time.
			expectedAffinity := &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "kubernetes.io/hostname",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{currentNodeName},
									},
								},
							},
						},
					},
				},
			}
			expectedToleration := []*corev1.Toleration{
				{
					Key:      utils.InferControlPlaneRoleKey(testCtx, clientset),
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				},
			}

			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireServer, func() {
				spireServer.Spec.Affinity = expectedAffinity
				spireServer.Spec.Tolerations = expectedToleration
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireServer object with affinity")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpireServer affinity modification")
				server := &operatorv1alpha1.SpireServer{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, server); err == nil {
					server.Spec.Affinity = nil
					server.Spec.Tolerations = nil
					k8sClient.Update(ctx, server)
				}
			})

			By("Waiting for SPIRE Server StatefulSet rolling update to start")
			utils.WaitForStatefulSetRollingUpdate(testCtx, clientset, utils.SpireServerStatefulSetName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for SPIRE Server StatefulSet to become Ready")
			utils.WaitForStatefulSetReady(testCtx, clientset, utils.SpireServerStatefulSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying the StatefulSet pod template has the expected affinity")
			updatedSts, err := clientset.AppsV1().StatefulSets(utils.OperatorNamespace).Get(testCtx, utils.SpireServerStatefulSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedSts.Spec.Template.Spec.Affinity).NotTo(BeNil(), "StatefulSet pod template should have affinity set")
			Expect(updatedSts.Spec.Template.Spec.Affinity.NodeAffinity).NotTo(BeNil(), "StatefulSet pod template should have NodeAffinity set")
			terms := updatedSts.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
			Expect(terms).NotTo(BeNil(), "NodeAffinity should have RequiredDuringSchedulingIgnoredDuringExecution")
			Expect(terms.NodeSelectorTerms).NotTo(BeEmpty())
			Expect(terms.NodeSelectorTerms[0].MatchExpressions).NotTo(BeEmpty())
			Expect(terms.NodeSelectorTerms[0].MatchExpressions[0].Key).To(Equal("kubernetes.io/hostname"))
			Expect(terms.NodeSelectorTerms[0].MatchExpressions[0].Values).To(ContainElement(currentNodeName))
			fmt.Fprintf(GinkgoWriter, "StatefulSet pod template has expected NodeAffinity targeting node '%s'\n", currentNodeName)

			By("Verifying the StatefulSet pod template has the expected tolerations")
			Expect(updatedSts.Spec.Template.Spec.Tolerations).NotTo(BeEmpty(), "StatefulSet pod template should have tolerations set")

			By("Verifying if SPIRE Server Pod is on the expected Node")
			newPods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireServerPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(newPods.Items).NotTo(BeEmpty())
			Expect(newPods.Items[0].Spec.NodeName).To(Equal(currentNodeName), "pod should remain on the node matching the affinity rule")
			fmt.Fprintf(GinkgoWriter, "pod '%s' is on node '%s' matching the affinity rule\n", newPods.Items[0].Name, newPods.Items[0].Spec.NodeName)
		})

		It("SPIRE Server log level can be configured through CR", func() {
			By("Retrieving initial log level from SPIRE Server ConfigMap")
			initialLogLevel, found, err := utils.GetNestedStringFromConfigMapJSON(testCtx, clientset, utils.OperatorNamespace, utils.SpireServerConfigMapName, utils.SpireServerConfigKey, "server", "log_level")
			Expect(err).NotTo(HaveOccurred(), "failed to get initial server.log_level from ConfigMap")
			Expect(found).To(BeTrue(), "server.log_level should exist in ConfigMap")
			fmt.Fprintf(GinkgoWriter, "initial log level from ConfigMap: %s\n", initialLogLevel)

			By("Getting SpireServer object")
			spireServer := &operatorv1alpha1.SpireServer{}
			err = k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireServer)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireServer object")

			// record initial generation of the StatefulSet before updating SpireServer object
			statefulset, err := clientset.AppsV1().StatefulSets(utils.OperatorNamespace).Get(testCtx, utils.SpireServerStatefulSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireServer StatefulSet")
			initialGen := statefulset.Generation

			By("Patching SpireServer object with verbose log level")
			newLogLevel := "debug"
			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireServer, func() {
				spireServer.Spec.LogLevel = newLogLevel
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireServer with log level")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpireServer log level")
				server := &operatorv1alpha1.SpireServer{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, server); err == nil {
					server.Spec.LogLevel = initialLogLevel
					k8sClient.Update(ctx, server)
				}
			})

			By("Waiting for SPIRE Server StatefulSet rolling update to start")
			utils.WaitForStatefulSetRollingUpdate(testCtx, clientset, utils.SpireServerStatefulSetName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for SPIRE Server StatefulSet to become Ready")
			utils.WaitForStatefulSetReady(testCtx, clientset, utils.SpireServerStatefulSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIRE Server ConfigMap has the expected log level")
			logLevel, found, err := utils.GetNestedStringFromConfigMapJSON(testCtx, clientset, utils.OperatorNamespace, utils.SpireServerConfigMapName, utils.SpireServerConfigKey, "server", "log_level")
			Expect(err).NotTo(HaveOccurred(), "failed to get server.log_level from ConfigMap")
			Expect(found).To(BeTrue(), "server.log_level should exist in ConfigMap")
			Expect(logLevel).To(Equal(newLogLevel), "log_level should be updated to %s", newLogLevel)
		})

		It("SPIRE Server custom labels can be configured through CR and propagated to pod", func() {
			By("Getting SpireServer object")
			spireServer := &operatorv1alpha1.SpireServer{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireServer)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireServer object")

			// Record initial generation of the StatefulSet before updating SpireServer
			statefulset, err := clientset.AppsV1().StatefulSets(utils.OperatorNamespace).Get(testCtx, utils.SpireServerStatefulSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to get StatefulSet")
			initialGen := statefulset.Generation

			By("Patching SpireServer object with test labels")
			testLabels := map[string]string{
				"e2e-test-label": "test-value",
				"component":      "server",
			}
			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireServer, func() {
				spireServer.Spec.Labels = testLabels
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireServer with labels")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpireServer labels modification")
				server := &operatorv1alpha1.SpireServer{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, server); err == nil {
					server.Spec.Labels = nil
					k8sClient.Update(ctx, server)
				}
			})

			By("Waiting for SPIRE Server StatefulSet rolling update to start")
			utils.WaitForStatefulSetRollingUpdate(testCtx, clientset, utils.SpireServerStatefulSetName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for SPIRE Server StatefulSet to become Ready")
			utils.WaitForStatefulSetReady(testCtx, clientset, utils.SpireServerStatefulSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIRE Server Pods have the expected labels")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireServerPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			utils.VerifyPodLabels(pods.Items, testLabels)
		})

		It("SPIRE Agent containers resource limits and requests can be configured through CR", func() {
			By("Getting SpireAgent object")
			spireAgent := &operatorv1alpha1.SpireAgent{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireAgent)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireAgent object")

			// record initial generation of the DaemonSet before updating SpireAgent object
			daemonset, err := clientset.AppsV1().DaemonSets(utils.OperatorNamespace).Get(testCtx, utils.SpireAgentDaemonSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := daemonset.Generation

			By("Patching SpireAgent object with resource specifications")
			expectedResources := &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			}

			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireAgent, func() {
				spireAgent.Spec.Resources = expectedResources
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireAgent object with resources")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpireAgent resources modification")
				agent := &operatorv1alpha1.SpireAgent{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, agent); err == nil {
					agent.Spec.Resources = nil
					k8sClient.Update(ctx, agent)
				}
			})

			By("Waiting for SPIRE Agent DaemonSet rolling update to start")
			utils.WaitForDaemonSetRollingUpdate(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, initialGen, utils.DefaultTimeout)

			By("Waiting for SPIRE Agent DaemonSet to become Available")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIRE Agent Pods have the expected resource limits and requests")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireAgentPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			utils.VerifyContainerResources(pods.Items, expectedResources)
		})

		It("SPIRE Agent nodeSelector and tolerations can be configured through CR", func() {
			By("Getting SpireAgent object")
			spireAgent := &operatorv1alpha1.SpireAgent{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireAgent)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireAgent object")

			// record initial generation of the DaemonSet before updating SpireAgent object
			daemonset, err := clientset.AppsV1().DaemonSets(utils.OperatorNamespace).Get(testCtx, utils.SpireAgentDaemonSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := daemonset.Generation

			By("Patching SpireAgent object with nodeSelector and tolerations to schedule pods on all Linux nodes")
			expectedNodeSelector := map[string]string{
				"kubernetes.io/os": "linux",
			}
			expectedToleration := []*corev1.Toleration{
				{
					Key:      utils.InferControlPlaneRoleKey(testCtx, clientset),
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				},
			}

			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireAgent, func() {
				spireAgent.Spec.NodeSelector = expectedNodeSelector
				spireAgent.Spec.Tolerations = expectedToleration
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireAgent object with nodeSelector and tolerations")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpireAgent nodeSelector and tolerations modification")
				agent := &operatorv1alpha1.SpireAgent{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, agent); err == nil {
					agent.Spec.NodeSelector = nil
					agent.Spec.Tolerations = nil
					k8sClient.Update(ctx, agent)
				}
			})

			By("Waiting for SPIRE Agent DaemonSet rolling update to start")
			utils.WaitForDaemonSetRollingUpdate(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for SPIRE Agent DaemonSet to become Available")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIRE Agent Pods have been scheduled to Nodes with required labels")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireAgentPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			utils.VerifyPodScheduling(testCtx, clientset, pods.Items, expectedNodeSelector)

			By("Verifying if SPIRE Agent Pods tolerate Node taints correctly")
			utils.VerifyPodTolerations(testCtx, clientset, pods.Items, expectedToleration)
		})

		It("SPIRE Agent affinity can be configured through CR", func() {
			By("Retrieving any SPIRE Agent Pod and its Node for affinity testing")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireAgentPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			spireAgentPod := pods.Items[0]
			targetNodeName := spireAgentPod.Spec.NodeName
			fmt.Fprintf(GinkgoWriter, "will use node '%s' as target to exclude\n", targetNodeName)

			By("Labeling the target Node with test label to simulate NodeAffinity exclusion")
			testLabelKey := "test.spire.agent/node-affinity"
			testLabelValue := "exclude"

			patchData := fmt.Sprintf(`{"metadata":{"labels":{"%s":"%s"}}}`, testLabelKey, testLabelValue)
			_, err = clientset.CoreV1().Nodes().Patch(testCtx, targetNodeName, types.StrategicMergePatchType, []byte(patchData), metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to label node '%s'", targetNodeName)
			DeferCleanup(func(ctx context.Context) {
				By("Removing test label from Node")
				patchData := fmt.Sprintf(`{"metadata":{"labels":{"%s":null}}}`, testLabelKey)
				clientset.CoreV1().Nodes().Patch(ctx, targetNodeName, types.StrategicMergePatchType, []byte(patchData), metav1.PatchOptions{})
			})

			By("Getting SpireAgent object")
			spireAgent := &operatorv1alpha1.SpireAgent{}
			err = k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireAgent)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireAgent object")

			// record initial generation of the DaemonSet before updating SpireAgent object
			daemonset, err := clientset.AppsV1().DaemonSets(utils.OperatorNamespace).Get(testCtx, utils.SpireAgentDaemonSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := daemonset.Generation

			By("Patching SpireAgent object with NodeAffinity configuration to exclude labeled nodes")
			expectedAffinity := &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      testLabelKey,
										Operator: corev1.NodeSelectorOpNotIn,
										Values:   []string{testLabelValue},
									},
								},
							},
						},
					},
				},
			}
			expectedToleration := []*corev1.Toleration{
				{
					Key:      utils.InferControlPlaneRoleKey(testCtx, clientset),
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				},
			}

			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireAgent, func() {
				spireAgent.Spec.Affinity = expectedAffinity
				spireAgent.Spec.Tolerations = expectedToleration
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireAgent object with affinity")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpireAgent affinity modification")
				agent := &operatorv1alpha1.SpireAgent{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, agent); err == nil {
					agent.Spec.Affinity = nil
					agent.Spec.Tolerations = nil
					k8sClient.Update(ctx, agent)
				}
			})

			By("Waiting for SPIRE Agent DaemonSet rolling update to start")
			utils.WaitForDaemonSetRollingUpdate(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for SPIRE Agent DaemonSet to become Available")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIRE Agent Pods are excluded from the labeled Node")
			newPods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireAgentPodLabel})
			Expect(err).NotTo(HaveOccurred())
			for _, pod := range newPods.Items {
				Expect(pod.Spec.NodeName).NotTo(Equal(targetNodeName), "pod should not be scheduled on the labeled node '%s'", targetNodeName)
				fmt.Fprintf(GinkgoWriter, "pod '%s' correctly excluded from labeled node '%s', scheduled on '%s'\n", pod.Name, targetNodeName, pod.Spec.NodeName)
			}
		})

		It("SPIRE Agent log level can be configured through CR", func() {
			By("Retrieving initial log level from SPIRE Agent ConfigMap")
			initialLogLevel, found, err := utils.GetNestedStringFromConfigMapJSON(testCtx, clientset, utils.OperatorNamespace, utils.SpireAgentConfigMapName, utils.SpireAgentConfigKey, "agent", "log_level")
			Expect(err).NotTo(HaveOccurred(), "failed to get initial agent.log_level from ConfigMap")
			Expect(found).To(BeTrue(), "agent.log_level should exist in ConfigMap")
			fmt.Fprintf(GinkgoWriter, "initial log level from ConfigMap: %s\n", initialLogLevel)

			By("Getting SpireAgent object")
			spireAgent := &operatorv1alpha1.SpireAgent{}
			err = k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireAgent)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireAgent object")

			// record initial generation of the DaemonSet before updating SpireAgent object
			daemonset, err := clientset.AppsV1().DaemonSets(utils.OperatorNamespace).Get(testCtx, utils.SpireAgentDaemonSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireAgent DaemonSet")
			initialGen := daemonset.Generation

			By("Patching SpireAgent object with verbose log level")
			newLogLevel := "debug"
			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireAgent, func() {
				spireAgent.Spec.LogLevel = newLogLevel
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireAgent with log level")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpireAgent log level")
				agent := &operatorv1alpha1.SpireAgent{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, agent); err == nil {
					agent.Spec.LogLevel = initialLogLevel
					k8sClient.Update(ctx, agent)
				}
			})

			By("Waiting for SPIRE Agent DaemonSet rolling update to start")
			utils.WaitForDaemonSetRollingUpdate(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for SPIRE Agent DaemonSet to become Available")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIRE Agent ConfigMap has the expected log level")
			logLevel, found, err := utils.GetNestedStringFromConfigMapJSON(testCtx, clientset, utils.OperatorNamespace, utils.SpireAgentConfigMapName, utils.SpireAgentConfigKey, "agent", "log_level")
			Expect(err).NotTo(HaveOccurred(), "failed to get agent.log_level from ConfigMap")
			Expect(found).To(BeTrue(), "agent.log_level should exist in ConfigMap")
			Expect(logLevel).To(Equal(newLogLevel), "log_level should be updated to %s", newLogLevel)
		})

		It("SPIRE Agent custom labels can be configured through CR and propagated to pod", func() {
			By("Getting SpireAgent object")
			spireAgent := &operatorv1alpha1.SpireAgent{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireAgent)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireAgent object")

			// Record initial generation of the DaemonSet before updating SpireAgent
			daemonset, err := clientset.AppsV1().DaemonSets(utils.OperatorNamespace).Get(testCtx, utils.SpireAgentDaemonSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to get DaemonSet")
			initialGen := daemonset.Generation

			By("Patching SpireAgent object with test labels")
			testLabels := map[string]string{
				"e2e-test-label": "test-value",
				"component":      "agent",
			}
			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireAgent, func() {
				spireAgent.Spec.Labels = testLabels
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireAgent with labels")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpireAgent labels modification")
				agent := &operatorv1alpha1.SpireAgent{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, agent); err == nil {
					agent.Spec.Labels = nil
					k8sClient.Update(ctx, agent)
				}
			})

			By("Waiting for SPIRE Agent DaemonSet rolling update to start")
			utils.WaitForDaemonSetRollingUpdate(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for SPIRE Agent DaemonSet to become Available")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIRE Agent Pods have the expected labels")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireAgentPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			utils.VerifyPodLabels(pods.Items, testLabels)
		})

		It("SPIFFE CSI Driver containers resource limits and requests can be configured through CR", func() {
			By("Getting SpiffeCSIDriver object")
			spiffeCSIDriver := &operatorv1alpha1.SpiffeCSIDriver{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spiffeCSIDriver)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpiffeCSIDriver object")

			// record initial generation of the DaemonSet before updating SpiffeCSIDriver object
			daemonset, err := clientset.AppsV1().DaemonSets(utils.OperatorNamespace).Get(testCtx, utils.SpiffeCSIDriverDaemonSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := daemonset.Generation

			By("Patching SpiffeCSIDriver object with resource specifications")
			expectedResources := &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			}

			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spiffeCSIDriver, func() {
				spiffeCSIDriver.Spec.Resources = expectedResources
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpiffeCSIDriver object with resources")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpiffeCSIDriver resources modification")
				driver := &operatorv1alpha1.SpiffeCSIDriver{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, driver); err == nil {
					driver.Spec.Resources = nil
					k8sClient.Update(ctx, driver)
				}
			})

			By("Waiting for SPIFFE CSI Driver DaemonSet rolling update to start")
			utils.WaitForDaemonSetRollingUpdate(testCtx, clientset, utils.SpiffeCSIDriverDaemonSetName, utils.OperatorNamespace, initialGen, utils.DefaultTimeout)

			By("Waiting for SPIFFE CSI Driver DaemonSet to become Available")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpiffeCSIDriverDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIFFE CSI Driver Pods have the expected resource limits and requests")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpiffeCSIDriverPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			utils.VerifyContainerResources(pods.Items, expectedResources)
		})

		It("SPIFFE CSI Driver nodeSelector and tolerations can be configured through CR", func() {
			By("Getting SpiffeCSIDriver object")
			spiffeCSIDriver := &operatorv1alpha1.SpiffeCSIDriver{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spiffeCSIDriver)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpiffeCSIDriver object")

			// record initial generation of the DaemonSet before updating SpiffeCSIDriver object
			daemonset, err := clientset.AppsV1().DaemonSets(utils.OperatorNamespace).Get(testCtx, utils.SpiffeCSIDriverDaemonSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := daemonset.Generation

			By("Patching SpiffeCSIDriver object with nodeSelector and tolerations to schedule pods on all Linux nodes")
			expectedNodeSelector := map[string]string{
				"kubernetes.io/os": "linux",
			}
			expectedToleration := []*corev1.Toleration{
				{
					Key:      utils.InferControlPlaneRoleKey(testCtx, clientset),
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				},
			}

			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spiffeCSIDriver, func() {
				spiffeCSIDriver.Spec.NodeSelector = expectedNodeSelector
				spiffeCSIDriver.Spec.Tolerations = expectedToleration
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpiffeCSIDriver object with nodeSelector and tolerations")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpiffeCSIDriver nodeSelector and tolerations modification")
				driver := &operatorv1alpha1.SpiffeCSIDriver{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, driver); err == nil {
					driver.Spec.NodeSelector = nil
					driver.Spec.Tolerations = nil
					k8sClient.Update(ctx, driver)
				}
			})

			By("Waiting for SPIFFE CSI Driver DaemonSet rolling update to start")
			utils.WaitForDaemonSetRollingUpdate(testCtx, clientset, utils.SpiffeCSIDriverDaemonSetName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for SPIFFE CSI Driver DaemonSet to become Available")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpiffeCSIDriverDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIFFE CSI Driver Pods have been scheduled to Nodes with required labels")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpiffeCSIDriverPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			utils.VerifyPodScheduling(testCtx, clientset, pods.Items, expectedNodeSelector)

			By("Verifying if SPIFFE CSI Driver Pods tolerate Node taints correctly")
			utils.VerifyPodTolerations(testCtx, clientset, pods.Items, expectedToleration)
		})

		It("SPIFFE CSI Driver affinity can be configured through CR", func() {
			By("Retrieving any SPIFFE CSI Driver Pod and its Node for affinity testing")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpiffeCSIDriverPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			spiffeCSIDriverPod := pods.Items[0]
			targetNodeName := spiffeCSIDriverPod.Spec.NodeName
			fmt.Fprintf(GinkgoWriter, "will use node '%s' as target to exclude\n", targetNodeName)

			By("Labeling the target Node with test label to simulate NodeAffinity exclusion")
			testLabelKey := "test.spiffe-csi-driver/node-affinity"
			testLabelValue := "exclude"

			patchData := fmt.Sprintf(`{"metadata":{"labels":{"%s":"%s"}}}`, testLabelKey, testLabelValue)
			_, err = clientset.CoreV1().Nodes().Patch(testCtx, targetNodeName, types.StrategicMergePatchType, []byte(patchData), metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to label node '%s'", targetNodeName)
			DeferCleanup(func(ctx context.Context) {
				By("Removing test label from Node")
				patchData := fmt.Sprintf(`{"metadata":{"labels":{"%s":null}}}`, testLabelKey)
				clientset.CoreV1().Nodes().Patch(ctx, targetNodeName, types.StrategicMergePatchType, []byte(patchData), metav1.PatchOptions{})
			})

			By("Getting SpiffeCSIDriver object")
			spiffeCSIDriver := &operatorv1alpha1.SpiffeCSIDriver{}
			err = k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spiffeCSIDriver)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpiffeCSIDriver object")

			// record initial generation of the DaemonSet before updating SpiffeCSIDriver object
			daemonset, err := clientset.AppsV1().DaemonSets(utils.OperatorNamespace).Get(testCtx, utils.SpiffeCSIDriverDaemonSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := daemonset.Generation

			By("Patching SpiffeCSIDriver object with NodeAffinity configuration to exclude labeled nodes")
			expectedAffinity := &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      testLabelKey,
										Operator: corev1.NodeSelectorOpNotIn,
										Values:   []string{testLabelValue},
									},
								},
							},
						},
					},
				},
			}

			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spiffeCSIDriver, func() {
				spiffeCSIDriver.Spec.Affinity = expectedAffinity
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpiffeCSIDriver object with affinity")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpiffeCSIDriver affinity modification")
				driver := &operatorv1alpha1.SpiffeCSIDriver{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, driver); err == nil {
					driver.Spec.Affinity = nil
					k8sClient.Update(ctx, driver)
				}
			})

			By("Waiting for SPIFFE CSI Driver DaemonSet rolling update to start")
			utils.WaitForDaemonSetRollingUpdate(testCtx, clientset, utils.SpiffeCSIDriverDaemonSetName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for SPIFFE CSI Driver DaemonSet to become Available")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpiffeCSIDriverDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIFFE CSI Driver Pods are excluded from the labeled Node")
			newPods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpiffeCSIDriverPodLabel})
			Expect(err).NotTo(HaveOccurred())
			for _, pod := range newPods.Items {
				Expect(pod.Spec.NodeName).NotTo(Equal(targetNodeName), "pod should not be scheduled on the labeled node '%s'", targetNodeName)
				fmt.Fprintf(GinkgoWriter, "pod '%s' correctly excluded from labeled node '%s', scheduled on '%s'\n", pod.Name, targetNodeName, pod.Spec.NodeName)
			}
		})

		It("SPIFFE CSI Driver custom labels can be configured through CR and propagated to pod", func() {
			By("Getting SpiffeCSIDriver object")
			spiffeCSIDriver := &operatorv1alpha1.SpiffeCSIDriver{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spiffeCSIDriver)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpiffeCSIDriver object")

			// Record initial generation of the DaemonSet before updating SpiffeCSIDriver
			daemonset, err := clientset.AppsV1().DaemonSets(utils.OperatorNamespace).Get(testCtx, utils.SpiffeCSIDriverDaemonSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to get DaemonSet")
			initialGen := daemonset.Generation

			By("Patching SpiffeCSIDriver object with test labels")
			testLabels := map[string]string{
				"e2e-test-label": "test-value",
				"component":      "csi",
			}
			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spiffeCSIDriver, func() {
				spiffeCSIDriver.Spec.Labels = testLabels
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpiffeCSIDriver with labels")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpiffeCSIDriver labels modification")
				driver := &operatorv1alpha1.SpiffeCSIDriver{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, driver); err == nil {
					driver.Spec.Labels = nil
					k8sClient.Update(ctx, driver)
				}
			})

			By("Waiting for SPIFFE CSI Driver DaemonSet rolling update to start")
			utils.WaitForDaemonSetRollingUpdate(testCtx, clientset, utils.SpiffeCSIDriverDaemonSetName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for SPIFFE CSI Driver DaemonSet to become Available")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpiffeCSIDriverDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIFFE CSI Driver Pods have the expected labels")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpiffeCSIDriverPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			utils.VerifyPodLabels(pods.Items, testLabels)
		})

		It("SPIRE OIDC Discovery Provider containers resource limits and requests can be configured through CR", func() {
			By("Getting SpireOIDCDiscoveryProvider object")
			spireOIDCDiscoveryProvider := &operatorv1alpha1.SpireOIDCDiscoveryProvider{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireOIDCDiscoveryProvider)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireOIDCDiscoveryProvider object")

			// record initial generation of the Deployment before updating SpireOIDCDiscoveryProvider object
			deployment, err := clientset.AppsV1().Deployments(utils.OperatorNamespace).Get(testCtx, utils.SpireOIDCDiscoveryProviderDeploymentName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := deployment.Generation

			By("Patching SpireOIDCDiscoveryProvider object with resource specifications")
			expectedResources := &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("32Mi"),
				},
			}

			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireOIDCDiscoveryProvider, func() {
				spireOIDCDiscoveryProvider.Spec.Resources = expectedResources
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireOIDCDiscoveryProvider object with resources")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpireOIDCDiscoveryProvider resources modification")
				provider := &operatorv1alpha1.SpireOIDCDiscoveryProvider{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, provider); err == nil {
					provider.Spec.Resources = nil
					k8sClient.Update(ctx, provider)
				}
			})

			By("Waiting for SPIRE OIDC Discovery Provider Deployment rolling update to start")
			utils.WaitForDeploymentRollingUpdate(testCtx, clientset, utils.SpireOIDCDiscoveryProviderDeploymentName, utils.OperatorNamespace, initialGen, utils.DefaultTimeout)

			By("Waiting for SPIRE OIDC Discovery Provider Deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.SpireOIDCDiscoveryProviderDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIRE OIDC Discovery Provider Pods have the expected resource limits and requests")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireOIDCDiscoveryProviderPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			activePods := utils.FilterActivePods(pods.Items)
			Expect(activePods).NotTo(BeEmpty(), "no Running OIDC Discovery Provider pods found")
			utils.VerifyContainerResources(activePods, expectedResources)
		})

		It("SPIRE OIDC Discovery Provider nodeSelector and tolerations can be configured through CR", func() {
			By("Finding a different Node with SPIFFE CSI Driver Pod placed to schedule OIDC Discovery Provider Pod")
			oidcPods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireOIDCDiscoveryProviderPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(oidcPods.Items).NotTo(BeEmpty())
			currentNodeName := oidcPods.Items[0].Spec.NodeName

			driverPods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpiffeCSIDriverPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(driverPods.Items).NotTo(BeEmpty())

			var targetNodeName string
			for _, pod := range driverPods.Items {
				if pod.Spec.NodeName != "" && pod.Spec.NodeName != currentNodeName {
					targetNodeName = pod.Spec.NodeName
					break
				}
			}
			Expect(targetNodeName).NotTo(BeEmpty(), "failed to find a different node with SPIFFE CSI Driver pod placed")
			fmt.Fprintf(GinkgoWriter, "will move SPIRE OIDC Discovery Provider pod from '%s' to '%s'\n", currentNodeName, targetNodeName)

			By("Getting SpireOIDCDiscoveryProvider object")
			spireOIDCDiscoveryProvider := &operatorv1alpha1.SpireOIDCDiscoveryProvider{}
			err = k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireOIDCDiscoveryProvider)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireOIDCDiscoveryProvider object")

			// record initial generation of the Deployment before updating SpireOIDCDiscoveryProvider object
			deployment, err := clientset.AppsV1().Deployments(utils.OperatorNamespace).Get(testCtx, utils.SpireOIDCDiscoveryProviderDeploymentName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := deployment.Generation

			By("Patching SpireOIDCDiscoveryProvider object with nodeSelector and tolerations to schedule Pod on node with SPIFFE CSI Driver")
			expectedNodeSelector := map[string]string{
				"kubernetes.io/hostname": targetNodeName,
			}
			expectedToleration := []*corev1.Toleration{
				{
					Key:      utils.InferControlPlaneRoleKey(testCtx, clientset),
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				},
			}

			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireOIDCDiscoveryProvider, func() {
				spireOIDCDiscoveryProvider.Spec.NodeSelector = expectedNodeSelector
				spireOIDCDiscoveryProvider.Spec.Tolerations = expectedToleration
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireOIDCDiscoveryProvider object with nodeSelector and tolerations")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpireOIDCDiscoveryProvider nodeSelector and tolerations modification")
				provider := &operatorv1alpha1.SpireOIDCDiscoveryProvider{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, provider); err == nil {
					provider.Spec.NodeSelector = nil
					provider.Spec.Tolerations = nil
					k8sClient.Update(ctx, provider)
				}
			})

			By("Waiting for SPIRE OIDC Discovery Provider Deployment rolling update to start")
			utils.WaitForDeploymentRollingUpdate(testCtx, clientset, utils.SpireOIDCDiscoveryProviderDeploymentName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for SPIRE OIDC Discovery Provider Deployment to become Ready")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.SpireOIDCDiscoveryProviderDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIRE OIDC Discovery Provider Pods has been scheduled to the target Node with SPIFFE CSI Driver Pod")
			newPods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireOIDCDiscoveryProviderPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(newPods.Items).NotTo(BeEmpty())
			runningPods := utils.FilterActivePods(newPods.Items)
			Expect(runningPods).NotTo(BeEmpty(), "no Running OIDC Discovery Provider pods found")
			utils.VerifyPodScheduling(testCtx, clientset, runningPods, expectedNodeSelector)

			By("Verifying if SPIRE OIDC Discovery Provider Pods tolerate Node taints correctly")
			utils.VerifyPodTolerations(testCtx, clientset, runningPods, expectedToleration)
		})

		It("SPIRE OIDC Discovery Provider affinity can be configured through CR", func() {
			By("Retrieving any SPIRE OIDC Discovery Provider Pod and its Node for affinity testing")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireOIDCDiscoveryProviderPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			spireOIDCDiscoveryProviderPod := pods.Items[0]
			currentNodeName := spireOIDCDiscoveryProviderPod.Spec.NodeName
			fmt.Fprintf(GinkgoWriter, "pod '%s' is currently on node '%s'\n", spireOIDCDiscoveryProviderPod.Name, currentNodeName)

			By("Finding SPIFFE CSI Driver Pod on a different Node to simulate NodeAffinity")
			csiDriverPods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpiffeCSIDriverPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(csiDriverPods.Items).NotTo(BeEmpty())

			var targetCSIDriverPod corev1.Pod
			var targetNodeName string
			for _, pod := range csiDriverPods.Items {
				if pod.Spec.NodeName != "" && pod.Spec.NodeName != currentNodeName {
					targetCSIDriverPod = pod
					targetNodeName = pod.Spec.NodeName
					break
				}
			}
			Expect(targetNodeName).NotTo(BeEmpty(), "failed to find a different node with SPIFFE CSI Driver pod placed")
			fmt.Fprintf(GinkgoWriter, "will use SPIFFE CSI Driver pod '%s' on node '%s' as affinity target\n", targetCSIDriverPod.Name, targetNodeName)

			By("Getting SpireOIDCDiscoveryProvider object")
			spireOIDCDiscoveryProvider := &operatorv1alpha1.SpireOIDCDiscoveryProvider{}
			err = k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireOIDCDiscoveryProvider)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireOIDCDiscoveryProvider object")

			// record initial generation of the Deployment before updating SpireOIDCDiscoveryProvider object
			deployment, err := clientset.AppsV1().Deployments(utils.OperatorNamespace).Get(testCtx, utils.SpireOIDCDiscoveryProviderDeploymentName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := deployment.Generation

			By("Patching SpireOIDCDiscoveryProvider object with NodeAffinity configuration")
			expectedAffinity := &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "kubernetes.io/hostname",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{targetNodeName},
									},
								},
							},
						},
					},
				},
			}
			expectedToleration := []*corev1.Toleration{
				{
					Key:      utils.InferControlPlaneRoleKey(testCtx, clientset),
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				},
			}

			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireOIDCDiscoveryProvider, func() {
				spireOIDCDiscoveryProvider.Spec.Affinity = expectedAffinity
				spireOIDCDiscoveryProvider.Spec.Tolerations = expectedToleration
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireOIDCDiscoveryProvider object with affinity")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpireOIDCDiscoveryProvider affinity modification")
				provider := &operatorv1alpha1.SpireOIDCDiscoveryProvider{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, provider); err == nil {
					provider.Spec.Affinity = nil
					provider.Spec.Tolerations = nil
					k8sClient.Update(ctx, provider)
				}
			})

			By("Waiting for SPIRE OIDC Discovery Provider Deployment rolling update to start")
			utils.WaitForDeploymentRollingUpdate(testCtx, clientset, utils.SpireOIDCDiscoveryProviderDeploymentName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for SPIRE OIDC Discovery Provider Deployment to become Ready")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.SpireOIDCDiscoveryProviderDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIRE OIDC Discovery Provider Pod has been rescheduled to the target Node")
			newPods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireOIDCDiscoveryProviderPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(newPods.Items).NotTo(BeEmpty())
			Expect(newPods.Items[0].Spec.NodeName).To(Equal(targetNodeName), "pod should be rescheduled to the target node")
			fmt.Fprintf(GinkgoWriter, "pod '%s' has been rescheduled to node '%s'\n", newPods.Items[0].Name, targetNodeName)
		})

		It("SPIRE OIDC Discovery Provider log level can be configured through CR", func() {
			By("Retrieving initial log level from SPIRE OIDC Discovery Provider ConfigMap")
			initialLogLevel, found, err := utils.GetNestedStringFromConfigMapJSON(testCtx, clientset, utils.OperatorNamespace, utils.SpireOIDCDiscoveryProviderConfigMapName, utils.SpireOIDCDiscoveryProviderConfigKey, "log_level")
			Expect(err).NotTo(HaveOccurred(), "failed to get initial log_level from ConfigMap")
			Expect(found).To(BeTrue(), "log_level should exist in ConfigMap")
			fmt.Fprintf(GinkgoWriter, "initial log level from ConfigMap: %s\n", initialLogLevel)

			By("Getting SpireOIDCDiscoveryProvider object")
			spireOIDCDiscoveryProvider := &operatorv1alpha1.SpireOIDCDiscoveryProvider{}
			err = k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireOIDCDiscoveryProvider)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireOIDCDiscoveryProvider object")

			// record initial generation of the Deployment before updating SpireOIDCDiscoveryProvider object
			deployment, err := clientset.AppsV1().Deployments(utils.OperatorNamespace).Get(testCtx, utils.SpireOIDCDiscoveryProviderDeploymentName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireOIDCDiscoveryProvider Deployment")
			initialGen := deployment.Generation

			By("Patching SpireOIDCDiscoveryProvider object with verbose log level")
			newLogLevel := "debug"
			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireOIDCDiscoveryProvider, func() {
				spireOIDCDiscoveryProvider.Spec.LogLevel = newLogLevel
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireOIDCDiscoveryProvider with log level")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpireOIDCDiscoveryProvider log level")
				provider := &operatorv1alpha1.SpireOIDCDiscoveryProvider{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, provider); err == nil {
					provider.Spec.LogLevel = initialLogLevel
					k8sClient.Update(ctx, provider)
				}
			})

			By("Waiting for SPIRE OIDC Discovery Provider Deployment rolling update to start")
			utils.WaitForDeploymentRollingUpdate(testCtx, clientset, utils.SpireOIDCDiscoveryProviderDeploymentName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for SPIRE OIDC Discovery Provider Deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.SpireOIDCDiscoveryProviderDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIRE OIDC Discovery Provider ConfigMap has the expected log level")
			logLevel, found, err := utils.GetNestedStringFromConfigMapJSON(testCtx, clientset, utils.OperatorNamespace, utils.SpireOIDCDiscoveryProviderConfigMapName, utils.SpireOIDCDiscoveryProviderConfigKey, "log_level")
			Expect(err).NotTo(HaveOccurred(), "failed to get log_level from ConfigMap")
			Expect(found).To(BeTrue(), "log_level should exist in ConfigMap")
			Expect(logLevel).To(Equal(newLogLevel), "log_level should be updated to %s", newLogLevel)
		})

		It("SPIRE OIDC Discovery Provider custom labels can be configured through CR and propagated to pod", func() {
			By("Getting SpireOIDCDiscoveryProvider object")
			spireOIDCDiscoveryProvider := &operatorv1alpha1.SpireOIDCDiscoveryProvider{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireOIDCDiscoveryProvider)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireOIDCDiscoveryProvider object")

			// Record initial generation of the Deployment before updating SpireOIDCDiscoveryProvider
			deployment, err := clientset.AppsV1().Deployments(utils.OperatorNamespace).Get(testCtx, utils.SpireOIDCDiscoveryProviderDeploymentName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to get Deployment")
			initialGen := deployment.Generation

			By("Patching SpireOIDCDiscoveryProvider object with test labels")
			testLabels := map[string]string{
				"e2e-test-label": "test-value",
				"component":      "oidc",
			}
			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireOIDCDiscoveryProvider, func() {
				spireOIDCDiscoveryProvider.Spec.Labels = testLabels
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireOIDCDiscoveryProvider with labels")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpireOIDCDiscoveryProvider labels modification")
				provider := &operatorv1alpha1.SpireOIDCDiscoveryProvider{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, provider); err == nil {
					provider.Spec.Labels = nil
					k8sClient.Update(ctx, provider)
				}
			})

			By("Waiting for SPIRE OIDC Discovery Provider Deployment rolling update to start")
			utils.WaitForDeploymentRollingUpdate(testCtx, clientset, utils.SpireOIDCDiscoveryProviderDeploymentName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for SPIRE OIDC Discovery Provider Deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.SpireOIDCDiscoveryProviderDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIRE OIDC Discovery Provider Pods have the expected labels")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireOIDCDiscoveryProviderPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			utils.VerifyPodLabels(pods.Items, testLabels)
		})
	})

	Context("CreateOnlyMode", func() {
		It("should transition based on CREATE_ONLY_MODE env var value", func() {
			By("Verifying CreateOnlyMode condition is not set by default")
			cr := &operatorv1alpha1.ZeroTrustWorkloadIdentityManager{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, cr)
			Expect(err).NotTo(HaveOccurred(), "failed to get ZeroTrustWorkloadIdentityManager")
			for _, cond := range cr.Status.Conditions {
				Expect(cond.Type).NotTo(Equal("CreateOnlyMode"), "CreateOnlyMode condition should not exist by default")
			}

			By("Patching Subscription object to enable CreateOnlyMode")
			err = utils.PatchSubscriptionEnv(testCtx, k8sClient, utils.OperatorNamespace, subscriptionName, utils.CreateOnlyModeEnvVar, "true")
			Expect(err).NotTo(HaveOccurred(), "failed to patch Subscription with env %s=true", utils.CreateOnlyModeEnvVar)

			By("Waiting for OLM to propagate CREATE_ONLY_MODE=true to the operator Deployment")
			utils.WaitForDeploymentEnvVar(testCtx, clientset, utils.OperatorNamespace, utils.OperatorDeploymentName, utils.CreateOnlyModeEnvVar, "true", utils.DefaultTimeout)

			By("Waiting for operator Deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying CreateOnlyMode condition is True")
			utils.WaitForZeroTrustWorkloadIdentityManagerConditions(testCtx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
				"CreateOnlyMode": metav1.ConditionTrue,
			}, utils.DefaultTimeout)

			By("Patching Subscription object to disable CreateOnlyMode")
			err = utils.PatchSubscriptionEnv(testCtx, k8sClient, utils.OperatorNamespace, subscriptionName, utils.CreateOnlyModeEnvVar, "false")
			Expect(err).NotTo(HaveOccurred(), "failed to patch Subscription with env %s=false", utils.CreateOnlyModeEnvVar)

			By("Waiting for OLM to propagate CREATE_ONLY_MODE=false to the operator Deployment")
			utils.WaitForDeploymentEnvVar(testCtx, clientset, utils.OperatorNamespace, utils.OperatorDeploymentName, utils.CreateOnlyModeEnvVar, "false", utils.DefaultTimeout)

			By("Waiting for operator Deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying CreateOnlyMode condition is False")
			utils.WaitForZeroTrustWorkloadIdentityManagerConditions(testCtx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
				"CreateOnlyMode": metav1.ConditionFalse,
			}, utils.DefaultTimeout)
		})

		It("should pause ConfigMap reconciliation when CreateOnlyMode is True and resume when CreateOnlyMode is False", func() {
			By("Getting original ConfigMap content")
			originalCM, err := clientset.CoreV1().ConfigMaps(utils.OperatorNamespace).Get(testCtx, utils.SpireServerConfigMapName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to get ConfigMap")
			originalServerConf := originalCM.Data[utils.SpireServerConfigKey]
			Expect(originalServerConf).NotTo(BeEmpty(), "%s should exist in ConfigMap", utils.SpireServerConfigKey)
			fmt.Fprintf(GinkgoWriter, "original ConfigMap resourceVersion: %s\n", originalCM.ResourceVersion)

			By("Patching Subscription object to enable CreateOnlyMode")
			err = utils.PatchSubscriptionEnv(testCtx, k8sClient, utils.OperatorNamespace, subscriptionName, utils.CreateOnlyModeEnvVar, "true")
			Expect(err).NotTo(HaveOccurred(), "failed to patch Subscription with env %s=true", utils.CreateOnlyModeEnvVar)

			By("Waiting for OLM to propagate CREATE_ONLY_MODE=true to the operator Deployment")
			utils.WaitForDeploymentEnvVar(testCtx, clientset, utils.OperatorNamespace, utils.OperatorDeploymentName, utils.CreateOnlyModeEnvVar, "true", utils.DefaultTimeout)

			By("Waiting for operator Deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Waiting for CreateOnlyMode condition to become True")
			utils.WaitForZeroTrustWorkloadIdentityManagerConditions(testCtx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
				"CreateOnlyMode": metav1.ConditionTrue,
			}, utils.DefaultTimeout)

			By("Patching ConfigMap to introduce drift")
			driftMarker := "# e2e-test-marker: drift-detection"
			modifiedConf := originalServerConf + "\n" + driftMarker
			cm, err := clientset.CoreV1().ConfigMaps(utils.OperatorNamespace).Get(testCtx, utils.SpireServerConfigMapName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to get ConfigMap")
			cm.Data[utils.SpireServerConfigKey] = modifiedConf
			_, err = clientset.CoreV1().ConfigMaps(utils.OperatorNamespace).Update(testCtx, cm, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to update ConfigMap with drift")

			By("Verifying ConfigMap drift is NOT corrected with CreateOnlyMode is True")
			Consistently(func() bool {
				cm, err := clientset.CoreV1().ConfigMaps(utils.OperatorNamespace).Get(testCtx, utils.SpireServerConfigMapName, metav1.GetOptions{})
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "failed to get ConfigMap: %v\n", err)
					return false
				}
				return strings.Contains(cm.Data[utils.SpireServerConfigKey], driftMarker)
			}).WithPolling(utils.ShortInterval).WithTimeout(30*time.Second).Should(BeTrue(),
				"ConfigMap drift should NOT be corrected when CreateOnlyMode is True")

			By("Patching Subscription object to disable CreateOnlyMode")
			err = utils.PatchSubscriptionEnv(testCtx, k8sClient, utils.OperatorNamespace, subscriptionName, utils.CreateOnlyModeEnvVar, "false")
			Expect(err).NotTo(HaveOccurred(), "failed to patch Subscription with env %s=false", utils.CreateOnlyModeEnvVar)

			By("Waiting for OLM to propagate CREATE_ONLY_MODE=false to the operator Deployment")
			utils.WaitForDeploymentEnvVar(testCtx, clientset, utils.OperatorNamespace, utils.OperatorDeploymentName, utils.CreateOnlyModeEnvVar, "false", utils.DefaultTimeout)

			By("Waiting for operator Deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Waiting for CreateOnlyMode condition to become False")
			utils.WaitForZeroTrustWorkloadIdentityManagerConditions(testCtx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
				"CreateOnlyMode": metav1.ConditionFalse,
			}, utils.DefaultTimeout)

			By("Verifying ConfigMap drift is corrected with CreateOnlyMode is False")
			Eventually(func() bool {
				cm, err := clientset.CoreV1().ConfigMaps(utils.OperatorNamespace).Get(testCtx, utils.SpireServerConfigMapName, metav1.GetOptions{})
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "failed to get ConfigMap: %v\n", err)
					return false
				}
				return !strings.Contains(cm.Data[utils.SpireServerConfigKey], driftMarker)
			}).WithPolling(utils.ShortInterval).WithTimeout(utils.ShortTimeout).Should(BeTrue(),
				"ConfigMap drift should be corrected when CreateOnlyMode is False")
		})
	})
})
