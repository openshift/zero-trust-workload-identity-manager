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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	operatorv1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/test/e2e/utils"
	operatorv1 "github.com/operator-framework/api/pkg/operators/v1"

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

	BeforeAll(func() {
		By("Getting cluster base domain")
		baseDomain, err := utils.GetClusterBaseDomain(context.Background(), configClient)
		Expect(err).NotTo(HaveOccurred(), "failed to get cluster base domain")

		// declare shared variables for tests
		appDomain = fmt.Sprintf("apps.%s", baseDomain)
		jwtIssuer = fmt.Sprintf("https://oidc-discovery.%s", appDomain)
		clusterName = "test01"
		bundleConfigMap = "spire-bundle"
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
			cr := &operatorv1alpha1.SpireServer{}
			conditionTypes := []string{
				"ServiceAccountAvailable",
				"ServiceAvailable",
				"RBACAvailable",
				"ValidatingWebhookAvailable",
				"ServerConfigMapAvailable",
				"ControllerManagerConfigAvailable",
				"BundleConfigAvailable",
				"StatefulSetAvailable",
				"TTLConfigurationValid",
				"Ready",
			}
			utils.WaitForCRConditionsTrue(testCtx, k8sClient, cr, conditionTypes, utils.DefaultTimeout)

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
			cr := &operatorv1alpha1.SpireAgent{}
			conditionTypes := []string{
				"ServiceAccountAvailable",
				"ServiceAvailable",
				"RBACAvailable",
				"ConfigMapAvailable",
				"SecurityContextConstraintsAvailable",
				"DaemonSetAvailable",
				"Ready",
			}
			utils.WaitForCRConditionsTrue(testCtx, k8sClient, cr, conditionTypes, utils.DefaultTimeout)

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
			cr := &operatorv1alpha1.SpiffeCSIDriver{}
			conditionTypes := []string{
				"ServiceAccountAvailable",
				"CSIDriverAvailable",
				"SecurityContextConstraintsAvailable",
				"DaemonSetAvailable",
				"Ready",
			}
			utils.WaitForCRConditionsTrue(testCtx, k8sClient, cr, conditionTypes, utils.DefaultTimeout)

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
			conditionTypes := []string{
				"ServiceAccountAvailable",
				"ServiceAvailable",
				"ClusterSPIFFEIDAvailable",
				"ConfigMapAvailable",
				"DeploymentAvailable",
				"RouteAvailable",
				"Ready",
			}
			cr := &operatorv1alpha1.SpireOIDCDiscoveryProvider{}
			utils.WaitForCRConditionsTrue(testCtx, k8sClient, cr, conditionTypes, utils.DefaultTimeout)

			By("Waiting for SPIRE OIDC Discovery Provider Deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.SpireOIDCDiscoveryProviderDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)
		})

		It("ZeroTrustWorkloadIdentityManager should aggregate status from all operands", func() {
			By("Waiting for ZeroTrustWorkloadIdentityManager to show all operands available")
			cr := &operatorv1alpha1.ZeroTrustWorkloadIdentityManager{}
			conditionTypes := []string{
				"OperandsAvailable",
				"Ready",
			}
			utils.WaitForCRConditionsTrue(testCtx, k8sClient, cr, conditionTypes, utils.DefaultTimeout)

			By("Verifying ZeroTrustWorkloadIdentityManager operand status")
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
				Expect(operand.Message).To(Equal("Ready"), "%s message should be 'Ready'", kind)
				fmt.Fprintf(GinkgoWriter, "Operand %s is ready\n", kind)
			}
		})

		It("OperatorCondition should have Upgradeable condition set correctly", func() {
			By("Finding OperatorCondition resource dynamically")
			operatorConditionList := &operatorv1.OperatorConditionList{}
			err := k8sClient.List(testCtx, operatorConditionList, client.InNamespace(utils.OperatorNamespace))
			Expect(err).NotTo(HaveOccurred(), "failed to list OperatorConditions")
			Expect(operatorConditionList.Items).NotTo(BeEmpty(), "no OperatorCondition found in namespace")

			// Find the OperatorCondition for this operator (name contains operator name prefix)
			var operatorCondition *operatorv1.OperatorCondition
			for i := range operatorConditionList.Items {
				if operatorConditionList.Items[i].Name == "zero-trust-workload-identity-manager" ||
					strings.HasPrefix(operatorConditionList.Items[i].Name, "zero-trust-workload-identity-manager.v") {
					operatorCondition = &operatorConditionList.Items[i]
					break
				}
			}
			Expect(operatorCondition).NotTo(BeNil(), "OperatorCondition for zero-trust-workload-identity-manager not found")
			fmt.Fprintf(GinkgoWriter, "Found OperatorCondition: %s\n", operatorCondition.Name)

			By("Verifying OperatorCondition Upgradeable condition")
			// Find Upgradeable condition in OperatorCondition
			var operatorCondUpgradeable *metav1.Condition
			for i := range operatorCondition.Status.Conditions {
				if operatorCondition.Status.Conditions[i].Type == "Upgradeable" {
					operatorCondUpgradeable = &operatorCondition.Status.Conditions[i]
					break
				}
			}
			Expect(operatorCondUpgradeable).NotTo(BeNil(), "Upgradeable condition should exist in OperatorCondition")
			fmt.Fprintf(GinkgoWriter, "OperatorCondition Upgradeable: Status=%s, Reason=%s, Message=%s\n",
				operatorCondUpgradeable.Status, operatorCondUpgradeable.Reason, operatorCondUpgradeable.Message)

			// Verify expected values when all operands are ready
			Expect(operatorCondUpgradeable.Status).To(Equal(metav1.ConditionTrue),
				"Upgradeable should be True when all operands are ready")
			Expect(operatorCondUpgradeable.Reason).To(Equal("Ready"),
				"Upgradeable reason should be Ready")

			fmt.Fprintf(GinkgoWriter, "OperatorCondition Upgradeable set correctly\n")
		})
	})

	Context("Common configurations", func() {
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

		It("SPIRE Agent log level can be configured through CR", func() {
			By("Getting SpireAgent object")
			spireAgent := &operatorv1alpha1.SpireAgent{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireAgent)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireAgent object")

			initialLogLevel := spireAgent.Spec.LogLevel
			if initialLogLevel == "" {
				initialLogLevel = "info"
			}
			fmt.Fprintf(GinkgoWriter, "initial log level: %s\n", initialLogLevel)

			daemonset, err := clientset.AppsV1().DaemonSets(utils.OperatorNamespace).Get(testCtx, utils.SpireAgentDaemonSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := daemonset.Generation

			By("Patching SpireAgent object with debug log level")
			newLogLevel := "debug"
			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireAgent, func() {
				spireAgent.Spec.LogLevel = newLogLevel
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireAgent log level")
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

			By("Verifying the CR has the updated log level")
			updatedAgent := &operatorv1alpha1.SpireAgent{}
			err = k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, updatedAgent)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedAgent.Spec.LogLevel).To(Equal(newLogLevel), "log level should be updated to %s", newLogLevel)

			By("Verifying pod logs contain debug level messages")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{
				LabelSelector: utils.SpireAgentPodLabel,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())

			// Wait for pod to emit some logs, then verify log level
			Eventually(func() bool {
				podLogs, err := clientset.CoreV1().Pods(utils.OperatorNamespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{}).Do(testCtx).Raw()
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "failed to get pod logs: %v\n", err)
					return false
				}
				logsStr := string(podLogs)
				if strings.Contains(logsStr, "level=debug") {
					fmt.Fprintf(GinkgoWriter, "found debug level log entries in pod '%s' logs\n", pods.Items[0].Name)
					return true
				}
				fmt.Fprintf(GinkgoWriter, "waiting for debug level log entries in pod '%s'...\n", pods.Items[0].Name)
				return false
			}).WithTimeout(utils.ShortTimeout).WithPolling(utils.ShortInterval).Should(BeTrue(),
				"pod logs should contain debug level messages after log level change")

			fmt.Fprintf(GinkgoWriter, "successfully validated log level change from %s to %s for SpireAgent\n", initialLogLevel, newLogLevel)
		})

		It("SPIRE Server log level can be configured through CR", func() {
			By("Getting SpireServer object")
			spireServer := &operatorv1alpha1.SpireServer{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireServer)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireServer object")

			// Record the initial log level (default is "info")
			initialLogLevel := spireServer.Spec.LogLevel
			if initialLogLevel == "" {
				initialLogLevel = "info"
			}
			fmt.Fprintf(GinkgoWriter, "initial log level: %s\n", initialLogLevel)

			// Record the initial generation of the StatefulSet before updating the SpireServer object
			statefulset, err := clientset.AppsV1().StatefulSets(utils.OperatorNamespace).Get(testCtx, utils.SpireServerStatefulSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := statefulset.Generation

			By("Patching SpireServer object with debug log level")
			newLogLevel := "debug"
			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireServer, func() {
				spireServer.Spec.LogLevel = newLogLevel
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireServer log level")
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

			By("Verifying the CR has the updated log level")
			updatedServer := &operatorv1alpha1.SpireServer{}
			err = k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, updatedServer)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedServer.Spec.LogLevel).To(Equal(newLogLevel), "log level should be updated to %s", newLogLevel)

			By("Verifying pod logs contain debug level messages")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{
				LabelSelector: utils.SpireServerPodLabel,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())

			// Wait for the pod to emit some logs, then verify the log level
			// Note: spire-server pod has multiple containers, so we need to specify the container name
			Eventually(func() bool {
				podLogs, err := clientset.CoreV1().Pods(utils.OperatorNamespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{
					Container: "spire-server",
				}).Do(testCtx).Raw()
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "failed to get pod logs: %v\n", err)
					return false
				}
				logsStr := string(podLogs)
				if strings.Contains(logsStr, "level=debug") {
					fmt.Fprintf(GinkgoWriter, "found debug level log entries in pod '%s' container 'spire-server' logs\n", pods.Items[0].Name)
					return true
				}
				fmt.Fprintf(GinkgoWriter, "waiting for debug level log entries in pod '%s' container 'spire-server'...\n", pods.Items[0].Name)
				return false
			}).WithTimeout(utils.ShortTimeout).WithPolling(utils.ShortInterval).Should(BeTrue(),
				"pod logs should contain debug level messages after log level change")

			fmt.Fprintf(GinkgoWriter, "successfully validated log level change from %s to %s for SpireServer\n", initialLogLevel, newLogLevel)
		})

		It("SPIRE OIDC Discovery Provider log level can be configured through CR", func() {
			By("Getting SpireOIDCDiscoveryProvider object")
			spireOIDCDiscoveryProvider := &operatorv1alpha1.SpireOIDCDiscoveryProvider{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireOIDCDiscoveryProvider)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireOIDCDiscoveryProvider object")

			// Record the initial log level (default is "info")
			initialLogLevel := spireOIDCDiscoveryProvider.Spec.LogLevel
			if initialLogLevel == "" {
				initialLogLevel = "info"
			}
			fmt.Fprintf(GinkgoWriter, "initial log level: %s\n", initialLogLevel)

			// Record the initial generation of the Deployment before updating SpireOIDCDiscoveryProvider object
			deployment, err := clientset.AppsV1().Deployments(utils.OperatorNamespace).Get(testCtx, utils.SpireOIDCDiscoveryProviderDeploymentName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := deployment.Generation

			By("Patching SpireOIDCDiscoveryProvider object with debug log level")
			newLogLevel := "debug"
			err = utils.UpdateCRWithRetry(testCtx, k8sClient, spireOIDCDiscoveryProvider, func() {
				spireOIDCDiscoveryProvider.Spec.LogLevel = newLogLevel
			})
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireOIDCDiscoveryProvider log level")
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

			By("Verifying the CR has the updated log level")
			updatedProvider := &operatorv1alpha1.SpireOIDCDiscoveryProvider{}
			err = k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, updatedProvider)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedProvider.Spec.LogLevel).To(Equal(newLogLevel), "log level should be updated to %s", newLogLevel)

			By("Verifying pod logs contain debug level messages")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{
				LabelSelector: utils.SpireOIDCDiscoveryProviderPodLabel,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())

			// Wait for pod to emit some logs, then verify log level
			Eventually(func() bool {
				podLogs, err := clientset.CoreV1().Pods(utils.OperatorNamespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{}).Do(testCtx).Raw()
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "failed to get pod logs: %v\n", err)
					return false
				}
				logsStr := string(podLogs)
				if strings.Contains(logsStr, "level=debug") {
					fmt.Fprintf(GinkgoWriter, "found debug level log entries in pod '%s' logs\n", pods.Items[0].Name)
					return true
				}
				fmt.Fprintf(GinkgoWriter, "waiting for debug level log entries in pod '%s'...\n", pods.Items[0].Name)
				return false
			}).WithTimeout(utils.ShortTimeout).WithPolling(utils.ShortInterval).Should(BeTrue(),
				"pod logs should contain debug level messages after log level change")

			fmt.Fprintf(GinkgoWriter, "successfully validated log level change from %s to %s for SpireOIDCDiscoveryProvider\n", initialLogLevel, newLogLevel)
		})
	})

	Context("Operator configurations", func() {
		It("Operator log level can be configured through Subscription", func() {
			By("Finding Subscription for the operator dynamically")
			// List all subscriptions in the namespace and find the one for our operator
			listResult := clientset.CoreV1().RESTClient().
				Get().
				AbsPath("/apis/operators.coreos.com/v1alpha1").
				Namespace(utils.OperatorNamespace).
				Resource("subscriptions").
				Do(testCtx)

			Expect(listResult.Error()).NotTo(HaveOccurred(),
				"failed to list Subscriptions in namespace '%s' - the Subscription API may not be available", utils.OperatorNamespace)

			rawData, err := listResult.Raw()
			Expect(err).NotTo(HaveOccurred(), "failed to get raw subscription list")

			// Parse the subscription list to find the operator's subscription
			var subscriptionList map[string]interface{}
			err = json.Unmarshal(rawData, &subscriptionList)
			Expect(err).NotTo(HaveOccurred(), "failed to unmarshal subscription list")

			items, ok := subscriptionList["items"].([]interface{})
			Expect(ok && len(items) > 0).To(BeTrue(),
				"no Subscriptions found in namespace '%s' - operator must be installed via OLM", utils.OperatorNamespace)

			// Find subscription that matches our operator (by name containing "zero-trust-workload-identity-manager")
			var subscriptionName string
			var foundNames []string
			for _, item := range items {
				sub, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				metadata, ok := sub["metadata"].(map[string]interface{})
				if !ok {
					continue
				}
				name, ok := metadata["name"].(string)
				if !ok {
					continue
				}
				foundNames = append(foundNames, name)
				// Check if subscription name contains our operator name
				if strings.Contains(name, "zero-trust-workload-identity-manager") {
					subscriptionName = name
					break
				}
			}

			Expect(subscriptionName).NotTo(BeEmpty(),
				"no Subscription matching 'zero-trust-workload-identity-manager' found in namespace '%s'. Available subscriptions: %v",
				utils.OperatorNamespace, foundNames)

			fmt.Fprintf(GinkgoWriter, "found subscription '%s'\n", subscriptionName)

			By("Getting current operator log level from deployment")
			deployment, err := clientset.AppsV1().Deployments(utils.OperatorNamespace).Get(testCtx, utils.OperatorDeploymentName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to get operator deployment")
			Expect(deployment.Spec.Template.Spec.Containers).NotTo(BeEmpty(), "operator deployment has no containers")

			// Find current OPERATOR_LOG_LEVEL value
			var initialLogLevel string
			for _, env := range deployment.Spec.Template.Spec.Containers[0].Env {
				if env.Name == "OPERATOR_LOG_LEVEL" {
					initialLogLevel = env.Value
					break
				}
			}
			if initialLogLevel == "" {
				initialLogLevel = "0" // default if not set
			}
			fmt.Fprintf(GinkgoWriter, "initial operator log level: %s\n", initialLogLevel)

			// Record initial generation
			initialGen := deployment.Generation

			By("Patching Subscription to change operator log level")
			newLogLevel := "4"
			if initialLogLevel == "4" {
				newLogLevel = "2" // if already at 4, change to 2
			}

			// Create patch for subscription config.env
			patchData := map[string]interface{}{
				"spec": map[string]interface{}{
					"config": map[string]interface{}{
						"env": []map[string]interface{}{
							{
								"name":  "OPERATOR_LOG_LEVEL",
								"value": newLogLevel,
							},
						},
					},
				},
			}
			patchBytes, err := json.Marshal(patchData)
			Expect(err).NotTo(HaveOccurred(), "failed to marshal patch data")

			// Patch the subscription using the REST client
			result := clientset.CoreV1().RESTClient().
				Patch(types.MergePatchType).
				AbsPath("/apis/operators.coreos.com/v1alpha1").
				Namespace(utils.OperatorNamespace).
				Resource("subscriptions").
				Name(subscriptionName).
				Body(patchBytes).
				Do(testCtx)
			Expect(result.Error()).NotTo(HaveOccurred(), "failed to patch subscription")
			fmt.Fprintf(GinkgoWriter, "patched subscription to set OPERATOR_LOG_LEVEL=%s\n", newLogLevel)

			// Capture subscriptionName for DeferCleanup closure
			subName := subscriptionName

			DeferCleanup(func(ctx context.Context) {
				By("Resetting operator log level in Subscription")
				revertPatchData := map[string]interface{}{
					"spec": map[string]interface{}{
						"config": map[string]interface{}{
							"env": []map[string]interface{}{
								{
									"name":  "OPERATOR_LOG_LEVEL",
									"value": initialLogLevel,
								},
							},
						},
					},
				}
				revertPatchBytes, err := json.Marshal(revertPatchData)
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "warning: failed to marshal revert patch data: %v\n", err)
					return
				}
				clientset.CoreV1().RESTClient().
					Patch(types.MergePatchType).
					AbsPath("/apis/operators.coreos.com/v1alpha1").
					Namespace(utils.OperatorNamespace).
					Resource("subscriptions").
					Name(subName).
					Body(revertPatchBytes).
					Do(ctx)
			})

			By("Waiting for operator Deployment to be updated by OLM")
			Eventually(func() bool {
				updatedDeployment, err := clientset.AppsV1().Deployments(utils.OperatorNamespace).Get(testCtx, utils.OperatorDeploymentName, metav1.GetOptions{})
				if err != nil {
					fmt.Fprintf(GinkgoWriter, "failed to get deployment: %v\n", err)
					return false
				}

				// Check if generation has changed
				if updatedDeployment.Generation <= initialGen {
					fmt.Fprintf(GinkgoWriter, "deployment generation not updated yet (current=%d, initial=%d)\n", updatedDeployment.Generation, initialGen)
					return false
				}

				// Check if deployment has containers
				if len(updatedDeployment.Spec.Template.Spec.Containers) == 0 {
					fmt.Fprintf(GinkgoWriter, "deployment has no containers yet\n")
					return false
				}

				// Check if the new log level is set
				for _, env := range updatedDeployment.Spec.Template.Spec.Containers[0].Env {
					if env.Name == "OPERATOR_LOG_LEVEL" && env.Value == newLogLevel {
						fmt.Fprintf(GinkgoWriter, "deployment updated with OPERATOR_LOG_LEVEL=%s\n", newLogLevel)
						return true
					}
				}

				fmt.Fprintf(GinkgoWriter, "waiting for OPERATOR_LOG_LEVEL to be updated in deployment...\n")
				return false
			}).WithTimeout(utils.DefaultTimeout).WithPolling(utils.DefaultInterval).Should(BeTrue(),
				"operator deployment should be updated with new log level")

			By("Waiting for operator Deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying the deployment has the updated log level")
			finalDeployment, err := clientset.AppsV1().Deployments(utils.OperatorNamespace).Get(testCtx, utils.OperatorDeploymentName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(finalDeployment.Spec.Template.Spec.Containers).NotTo(BeEmpty(), "operator deployment has no containers")

			var actualLogLevel string
			for _, env := range finalDeployment.Spec.Template.Spec.Containers[0].Env {
				if env.Name == "OPERATOR_LOG_LEVEL" {
					actualLogLevel = env.Value
					break
				}
			}
			Expect(actualLogLevel).To(Equal(newLogLevel), "OPERATOR_LOG_LEVEL should be updated to %s", newLogLevel)

			By("Verifying operator pod is running with new configuration")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{
				LabelSelector: utils.OperatorLabelSelector,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())

			// Wait for pods to be running and ready
			Eventually(func() bool {
				pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{
					LabelSelector: utils.OperatorLabelSelector,
				})
				if err != nil || len(pods.Items) == 0 {
					return false
				}
				for _, pod := range pods.Items {
					if pod.Status.Phase != corev1.PodRunning {
						return false
					}
					for _, cond := range pod.Status.Conditions {
						if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
							fmt.Fprintf(GinkgoWriter, "operator pod '%s' is running and ready\n", pod.Name)
							return true
						}
					}
				}
				return false
			}).WithTimeout(utils.ShortTimeout).WithPolling(utils.ShortInterval).Should(BeTrue(),
				"operator pod should be running and ready")

			fmt.Fprintf(GinkgoWriter, "successfully validated operator log level change from %s to %s\n", initialLogLevel, newLogLevel)
		})
	})
})
