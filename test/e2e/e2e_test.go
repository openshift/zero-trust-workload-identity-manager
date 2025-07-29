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

	BeforeAll(func() {
		By("Getting cluster base domain")
		baseDomain, err := utils.GetClusterBaseDomain(context.Background(), configClient)
		Expect(err).NotTo(HaveOccurred(), "failed to get cluster base domain")

		// declare shared variables for tests
		appDomain = fmt.Sprintf("apps.%s", baseDomain)
		clusterName = "test01"
		bundleConfigMap = "spire-bundle"
	})

	BeforeEach(func() {
		var cancel context.CancelFunc
		testCtx, cancel = context.WithTimeout(context.Background(), utils.DefaultTimeout)
		DeferCleanup(cancel)
	})

	Context("when installing the operator", func() {
		It("should create a healthy operator Deployment", func() {
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

			By("Waiting for all resource generation conditions in ZeroTrustWorkloadIdentityManager object to be True")
			conditionTypes := []string{
				"RBACResourcesGeneration",
				"ServiceResourcesGeneration",
				"ServiceAccountResourcesGeneration",
				"SpiffeCSIResourcesGeneration",
				"ValidatingWebhookConfigurationResourcesGeneration",
			}
			cr := &operatorv1alpha1.ZeroTrustWorkloadIdentityManager{}
			utils.WaitForCRConditionsTrue(testCtx, k8sClient, cr, conditionTypes, utils.ShortTimeout)

			By("Waiting for operator Deployment to become Available")
			utils.WaitForDeploymentAvailable(testCtx, clientset, utils.OperatorDeploymentName, utils.OperatorNamespace, utils.ShortTimeout)
		})

		It("should recover from the Pod force deletion", func() {
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
	})

	Context("when creating a SpireServer object", func() {
		It("should create a healthy SPIRE Server StatefulSet", func() {
			By("Creating SpireServer object")
			spireServer := &operatorv1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: operatorv1alpha1.SpireServerSpec{
					TrustDomain:     appDomain,
					ClusterName:     clusterName,
					BundleConfigMap: bundleConfigMap,
					CASubject: &operatorv1alpha1.CASubject{
						CommonName:   appDomain,
						Country:      "US",
						Organization: "RH",
					},
					Persistence: &operatorv1alpha1.Persistence{
						Type:       "pvc",
						Size:       "1Gi",
						AccessMode: "ReadWriteOncePod",
					},
					Datastore: &operatorv1alpha1.DataStore{
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

			By("Waiting for all resource generation conditions in SpireServer object to be True")
			conditionTypes := []string{
				"SpireServerConfigMapGeneration",
				"SpireControllerManagerConfigMapGeneration",
				"SpireBundleConfigMapGeneration",
				"SpireServerStatefulSetGeneration",
			}
			cr := &operatorv1alpha1.SpireServer{}
			utils.WaitForCRConditionsTrue(testCtx, k8sClient, cr, conditionTypes, utils.ShortTimeout)

			By("Waiting for SPIRE Server StatefulSet to become Ready")
			utils.WaitForStatefulSetReady(testCtx, clientset, utils.SpireServerStatefulSetName, utils.OperatorNamespace, utils.DefaultTimeout)
		})

		It("custom resource limits and requests should apply to the SPIRE Server containers", func() {
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

			spireServer.Spec.Resources = expectedResources
			err = k8sClient.Update(testCtx, spireServer)
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireServer object with resources")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpireServer resources modification")
				server := &operatorv1alpha1.SpireServer{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, server); err == nil {
					server.Spec.Resources = nil
					k8sClient.Update(ctx, server)
				}
			})

			By("Restarting operator Pod") // TODO: remove this step once SPIRE-68 is fixed
			err = clientset.CoreV1().Pods(utils.OperatorNamespace).DeleteCollection(testCtx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: utils.OperatorLabelSelector,
			})
			Expect(err).NotTo(HaveOccurred())

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

		It("custom nodeSelector and tolerations should apply to the SPIRE Server Pods and trigger scheduling", func() {
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

			spireServer.Spec.NodeSelector = expectedNodeSelector
			spireServer.Spec.Tolerations = expectedToleration
			err = k8sClient.Update(testCtx, spireServer)
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

			By("Restarting operator Pod") // TODO: remove this step once SPIRE-68 is fixed
			err = clientset.CoreV1().Pods(utils.OperatorNamespace).DeleteCollection(testCtx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: utils.OperatorLabelSelector,
			})
			Expect(err).NotTo(HaveOccurred())

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

		It("custom affinity should apply to the SPIRE Server Pods and trigger scheduling", func() {
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

			spireServer.Spec.Affinity = expectedAffinity
			spireServer.Spec.Tolerations = expectedToleration
			err = k8sClient.Update(testCtx, spireServer)
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

			By("Restarting operator Pod") // TODO: remove this step once SPIRE-68 is fixed
			err = clientset.CoreV1().Pods(utils.OperatorNamespace).DeleteCollection(testCtx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: utils.OperatorLabelSelector,
			})
			Expect(err).NotTo(HaveOccurred())

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
	})

	Context("when creating a SpireAgent object", func() {
		It("should create a healthy SPIRE Agent DaemonSet", func() {
			By("Creating SpireAgent object")
			spireAgent := &operatorv1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: operatorv1alpha1.SpireAgentSpec{
					TrustDomain:     appDomain,
					ClusterName:     clusterName,
					BundleConfigMap: bundleConfigMap,
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

			By("Waiting for all resource generation conditions in SpireAgent object to be True")
			conditionTypes := []string{
				"SpireAgentSCCGeneration",
				"SpireAgentConfigMapGeneration",
				"SpireAgentDaemonSetGeneration",
			}
			cr := &operatorv1alpha1.SpireAgent{}
			utils.WaitForCRConditionsTrue(testCtx, k8sClient, cr, conditionTypes, utils.ShortTimeout)

			By("Waiting for SPIRE Agent DaemonSet to become Available")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)
		})

		It("custom resource limits and requests should apply to the SPIRE Agent containers", func() {
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

			spireAgent.Spec.Resources = expectedResources
			err = k8sClient.Update(testCtx, spireAgent)
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireAgent object with resources")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting SpireAgent resources modification")
				agent := &operatorv1alpha1.SpireAgent{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, agent); err == nil {
					agent.Spec.Resources = nil
					k8sClient.Update(ctx, agent)
				}
			})

			By("Restarting operator Pod") // TODO: remove this step once SPIRE-68 is fixed
			err = clientset.CoreV1().Pods(utils.OperatorNamespace).DeleteCollection(testCtx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: utils.OperatorLabelSelector,
			})
			Expect(err).NotTo(HaveOccurred())

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

		It("custom nodeSelector and tolerations should apply to the SPIRE Agent Pods and trigger scheduling", func() {
			By("Getting SpireAgent object")
			spireAgent := &operatorv1alpha1.SpireAgent{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireAgent)
			Expect(err).NotTo(HaveOccurred(), "failed to get SpireAgent object")

			// record initial generation of the DaemonSet before updating SpireAgent object
			daemonset, err := clientset.AppsV1().DaemonSets(utils.OperatorNamespace).Get(testCtx, utils.SpireAgentDaemonSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := daemonset.Generation

			By("Patching SpireAgent object with nodeSelector and tolerations to schedule pods on control-plane nodes")
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

			spireAgent.Spec.NodeSelector = expectedNodeSelector
			spireAgent.Spec.Tolerations = expectedToleration
			err = k8sClient.Update(testCtx, spireAgent)
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

			By("Restarting operator Pod") // TODO: remove this step once SPIRE-68 is fixed
			err = clientset.CoreV1().Pods(utils.OperatorNamespace).DeleteCollection(testCtx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: utils.OperatorLabelSelector,
			})
			Expect(err).NotTo(HaveOccurred())

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

		It("custom affinity should apply to the SPIRE Agent Pods and trigger scheduling", func() {
			By("Retrieving any SPIRE Agent Pod and its Node for affinity testing")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireAgentPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			spireAgentPod := pods.Items[0]
			targetNodeName := spireAgentPod.Spec.NodeName
			fmt.Fprintf(GinkgoWriter, "will use node '%s' as target\n", targetNodeName)

			By("Labeling the target Node with test label to simulate NodeAffinity")
			testLabelKey := "test.spire.agent/node-affinity"
			testLabelValue := "true"

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

			By("Patching SpireAgent object with NodeAffinity configuration")
			expectedAffinity := &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      testLabelKey,
										Operator: corev1.NodeSelectorOpIn,
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

			spireAgent.Spec.Affinity = expectedAffinity
			spireAgent.Spec.Tolerations = expectedToleration
			err = k8sClient.Update(testCtx, spireAgent)
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

			By("Restarting operator Pod") // TODO: remove this step once SPIRE-68 is fixed
			err = clientset.CoreV1().Pods(utils.OperatorNamespace).DeleteCollection(testCtx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: utils.OperatorLabelSelector,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for SPIRE Agent DaemonSet rolling update to start")
			utils.WaitForDaemonSetRollingUpdate(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for SPIRE Agent DaemonSet to become Available")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpireAgentDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if SPIRE Agent Pod is constrained to the labeled Node")
			newPods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpireAgentPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(newPods.Items).To(HaveLen(1), "nodeAffinity should constrain daemonset to exactly one pod")
			Expect(newPods.Items[0].Spec.NodeName).To(Equal(targetNodeName), "pod should be scheduled on the labeled node")
			fmt.Fprintf(GinkgoWriter, "pod has been constrained to labeled node '%s'\n", targetNodeName)
		})
	})
})
