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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	operatorv1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/test/e2e/utils"

	corev1 "k8s.io/api/core/v1"
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
		testCtx, cancel = context.WithTimeout(context.Background(), utils.DefaultTimeout)
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
					DefaultX509Validity: metav1.Duration{Duration: 1 * time.Hour},
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
			By("Getting SpireServer object")
			spireServer := &operatorv1alpha1.SpireServer{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireServer)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireServer object")

			// record initial generation of the StatefulSet before updating SpireServer object
			statefulset, err := clientset.AppsV1().StatefulSets(utils.OperatorNamespace).Get(testCtx, utils.SpireServerStatefulSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := statefulset.Generation

			By("Patching SpireServer object with nodeSelector and tolerations to schedule Pod on control-plane Nodes")
			expectedNodeSelector := map[string]string{
				"node-role.kubernetes.io/control-plane": "",
			}
			expectedToleration := []*corev1.Toleration{
				{
					Key:      "node-role.kubernetes.io/master",
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				},
				{
					Key:      "node-role.kubernetes.io/control-plane",
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
			By("Retrieving any SPIRE Server Pod and its Node for affinity testing")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireServerPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			spireServerPod := pods.Items[0]
			originalNodeName := spireServerPod.Spec.NodeName
			fmt.Fprintf(GinkgoWriter, "pod '%s' is currently on node '%s'\n", spireServerPod.Name, originalNodeName)

			By("Creating test Pod on the same Node as SPIRE Server Pod to simulate PodAntiAffinity")
			testPodName := fmt.Sprintf("test-spire-server-%d", time.Now().Unix())
			testPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testPodName,
					Namespace: utils.OperatorNamespace,
					Labels: map[string]string{
						"statefulset.kubernetes.io/pod-name": spireServerPod.Name,
					},
				},
				Spec: corev1.PodSpec{
					NodeName: originalNodeName,
					Containers: []corev1.Container{
						{
							Name:    "dummy",
							Image:   "docker.io/library/busybox:latest",
							Command: []string{"sleep", "600"},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: &[]bool{false}[0],
								RunAsNonRoot:             &[]bool{true}[0],
								RunAsUser:                &[]int64{1000}[0],
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeRuntimeDefault,
								},
							},
						},
					},
				},
			}
			_, err = clientset.CoreV1().Pods(utils.OperatorNamespace).Create(testCtx, testPod, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to create test Pod")
			DeferCleanup(func(ctx context.Context) {
				By("Deleting test Pod")
				clientset.CoreV1().Pods(utils.OperatorNamespace).Delete(ctx, testPodName, metav1.DeleteOptions{})
			})

			By("Waiting for test Pod to become Running")
			utils.WaitForPodRunning(testCtx, clientset, testPodName, utils.OperatorNamespace, utils.ShortTimeout)

			By("Getting SpireServer object")
			spireServer := &operatorv1alpha1.SpireServer{}
			err = k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireServer)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireServer object")

			// record initial generation of the StatefulSet before updating SpireServer object
			statefulset, err := clientset.AppsV1().StatefulSets(utils.OperatorNamespace).Get(testCtx, utils.SpireServerStatefulSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := statefulset.Generation

			By("Patching SpireServer object with PodAntiAffinity configuration")
			expectedAffinity := &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"statefulset.kubernetes.io/pod-name": spireServerPod.Name,
								},
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			}
			expectedToleration := []*corev1.Toleration{
				{
					Key:      "node-role.kubernetes.io/master",
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

			By("Verifying if SPIRE Server Pod has been rescheduled to a different Node")
			newPods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireServerPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(newPods.Items).NotTo(BeEmpty())
			Expect(newPods.Items[0].Spec.NodeName).NotTo(Equal(originalNodeName), "pod should be rescheduled to a different node")
			fmt.Fprintf(GinkgoWriter, "pod '%s' has been rescheduled to node '%s'\n", newPods.Items[0].Name, newPods.Items[0].Spec.NodeName)
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
			utils.WaitForStatefulSetReady(testCtx, clientset, utils.SpireServerStatefulSetName, utils.OperatorNamespace, utils.ShortTimeout)

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
					Key:      "node-role.kubernetes.io/master",
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
					Key:      "node-role.kubernetes.io/master",
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
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, utils.ShortTimeout)

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
					Key:      "node-role.kubernetes.io/master",
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
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpiffeCSIDriverDaemonSetName, utils.OperatorNamespace, utils.ShortTimeout)

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
			utils.VerifyContainerResources(pods.Items, expectedResources)
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
					Key:      "node-role.kubernetes.io/master",
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
			utils.VerifyPodScheduling(testCtx, clientset, newPods.Items, expectedNodeSelector)

			By("Verifying if SPIRE OIDC Discovery Provider Pods tolerate Node taints correctly")
			utils.VerifyPodTolerations(testCtx, clientset, newPods.Items, expectedToleration)
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
					Key:      "node-role.kubernetes.io/master",
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
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.SpireOIDCDiscoveryProviderDeploymentName, utils.OperatorNamespace, utils.ShortTimeout)

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

			// record initial generation of the Deployment before updating Subscription object
			deployment, err := clientset.AppsV1().Deployments(utils.OperatorNamespace).Get(testCtx, utils.OperatorDeploymentName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := deployment.Generation

			By("Patching Subscription object to enable CreateOnlyMode")
			err = utils.PatchSubscriptionEnv(testCtx, k8sClient, utils.OperatorNamespace, subscriptionName, utils.CreateOnlyModeEnvVar, "true")
			Expect(err).NotTo(HaveOccurred(), "failed to patch Subscription with env %s=true", utils.CreateOnlyModeEnvVar)

			By("Waiting for operator Deployment rolling update to start")
			utils.WaitForDeploymentRollingUpdate(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, initialGen, utils.DefaultTimeout)

			By("Waiting for operator Deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying CreateOnlyMode condition is True")
			utils.WaitForZeroTrustWorkloadIdentityManagerConditions(testCtx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
				"CreateOnlyMode": metav1.ConditionTrue,
			}, utils.ShortTimeout)

			// record new generation of the Deployment before updating Subscription object
			deployment, err = clientset.AppsV1().Deployments(utils.OperatorNamespace).Get(testCtx, utils.OperatorDeploymentName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			newGen := deployment.Generation

			By("Patching Subscription object to disable CreateOnlyMode")
			err = utils.PatchSubscriptionEnv(testCtx, k8sClient, utils.OperatorNamespace, subscriptionName, utils.CreateOnlyModeEnvVar, "false")
			Expect(err).NotTo(HaveOccurred(), "failed to patch Subscription with env %s=false", utils.CreateOnlyModeEnvVar)

			By("Waiting for operator Deployment rolling update to start")
			utils.WaitForDeploymentRollingUpdate(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, newGen, utils.DefaultTimeout)

			By("Waiting for operator Deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying CreateOnlyMode condition is False")
			utils.WaitForZeroTrustWorkloadIdentityManagerConditions(testCtx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
				"CreateOnlyMode": metav1.ConditionFalse,
			}, utils.ShortTimeout)
		})

		It("should pause ConfigMap reconciliation when CreateOnlyMode is True and resume when CreateOnlyMode is False", func() {
			By("Getting original ConfigMap content")
			originalCM, err := clientset.CoreV1().ConfigMaps(utils.OperatorNamespace).Get(testCtx, utils.SpireServerConfigMapName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to get ConfigMap")
			originalServerConf := originalCM.Data[utils.SpireServerConfigKey]
			Expect(originalServerConf).NotTo(BeEmpty(), "%s should exist in ConfigMap", utils.SpireServerConfigKey)
			fmt.Fprintf(GinkgoWriter, "original ConfigMap resourceVersion: %s\n", originalCM.ResourceVersion)

			// record initial generation of the Deployment before updating Subscription object
			deployment, err := clientset.AppsV1().Deployments(utils.OperatorNamespace).Get(testCtx, utils.OperatorDeploymentName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to get operator Deployment")
			initialGen := deployment.Generation

			By("Patching Subscription object to enable CreateOnlyMode")
			err = utils.PatchSubscriptionEnv(testCtx, k8sClient, utils.OperatorNamespace, subscriptionName, utils.CreateOnlyModeEnvVar, "true")
			Expect(err).NotTo(HaveOccurred(), "failed to patch Subscription with env %s=true", utils.CreateOnlyModeEnvVar)

			By("Waiting for operator Deployment rolling update to start")
			utils.WaitForDeploymentRollingUpdate(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, initialGen, utils.DefaultTimeout)

			By("Waiting for operator Deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Waiting for CreateOnlyMode condition to become True")
			utils.WaitForZeroTrustWorkloadIdentityManagerConditions(testCtx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
				"CreateOnlyMode": metav1.ConditionTrue,
			}, utils.ShortTimeout)

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

			// record new generation of the Deployment before updating Subscription object
			deployment, err = clientset.AppsV1().Deployments(utils.OperatorNamespace).Get(testCtx, utils.OperatorDeploymentName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to get operator Deployment")
			newGen := deployment.Generation

			By("Patching Subscription object to disable CreateOnlyMode")
			err = utils.PatchSubscriptionEnv(testCtx, k8sClient, utils.OperatorNamespace, subscriptionName, utils.CreateOnlyModeEnvVar, "false")
			Expect(err).NotTo(HaveOccurred(), "failed to patch Subscription with env %s=false", utils.CreateOnlyModeEnvVar)

			By("Waiting for operator Deployment rolling update to start")
			utils.WaitForDeploymentRollingUpdate(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, newGen, utils.DefaultTimeout)

			By("Waiting for operator Deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Waiting for CreateOnlyMode condition is False")
			utils.WaitForZeroTrustWorkloadIdentityManagerConditions(testCtx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
				"CreateOnlyMode": metav1.ConditionFalse,
			}, utils.ShortTimeout)

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
