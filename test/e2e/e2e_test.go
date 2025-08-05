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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	operatorv1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/test/e2e/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
						Size:       "2Gi",
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
	})

	Context("when creating a spiffe csi object", func() {
		It("should create a healthy spiffe csi driver daemonset", func() {
			By("Creating spiffe csi driver object")
			spiffeCsi := &operatorv1alpha1.SpiffeCSIDriver{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: operatorv1alpha1.SpiffeCSIDriverSpec{},
			}
			err := k8sClient.Create(testCtx, spiffeCsi)
			Expect(err).NotTo(HaveOccurred(), "failed to create spiffe csi object")

			By("Waiting for all resource generation conditions in spiffe csi object to be True")
			conditionTypes := []string{
				"SpiffeCSISCCGeneration",
				"SpiffeCSIDaemonSetGeneration",
			}
			cr := &operatorv1alpha1.SpiffeCSIDriver{}
			utils.WaitForCRConditionsTrue(testCtx, k8sClient, cr, conditionTypes, utils.ShortTimeout)

			By("Waiting for SPIRE CSI DaemonSet to become Available")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpireCSIDriverDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)
		})

		It("SPIFFE CSI Driver containers resources can be configured through SpiffeCSIDriver CR", func() {
			By("Getting spiffe csi object")
			spiffeCsi := &operatorv1alpha1.SpiffeCSIDriver{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spiffeCsi)
			Expect(err).NotTo(HaveOccurred(), "failed to get spiffe csi object")

			// record initial generation of the DaemonSet before updating SpireAgent object
			daemonset, err := clientset.AppsV1().DaemonSets(utils.OperatorNamespace).Get(testCtx, utils.SpireCSIDriverDaemonSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := daemonset.Generation

			By("Patching spiffe csi object with resource specifications")
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

			spiffeCsi.Spec.Resources = expectedResources
			err = k8sClient.Update(testCtx, spiffeCsi)
			Expect(err).NotTo(HaveOccurred(), "failed to patch SpireAgent object with resources")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting spiffe csi resources modification")
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

			By("Waiting for spiife csi DaemonSet rolling update to start")
			utils.WaitForDaemonSetRollingUpdate(testCtx, clientset, utils.SpireCSIDriverDaemonSetName, utils.OperatorNamespace, initialGen, utils.DefaultTimeout)

			By("Waiting for spiffe CSI DaemonSet to become Available")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpireCSIDriverDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if spiffe CSI Pods have the expected resource limits and requests")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpiffeCsiDriverPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			utils.VerifyContainerResources(pods.Items, expectedResources)
		})

		It("Spiffe csi nodeSelector and tolerations can be configured through CR", func() {
			By("Getting SpireAgent object")
			spiffeCsi := &operatorv1alpha1.SpiffeCSIDriver{}
			err := k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spiffeCsi)
			Expect(err).NotTo(HaveOccurred(), "failed to get spiffe csi object")

			// record initial generation of the DaemonSet before updating SpireAgent object
			daemonset, err := clientset.AppsV1().DaemonSets(utils.OperatorNamespace).Get(testCtx, utils.SpireCSIDriverDaemonSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := daemonset.Generation

			By("Patching spiffe csi object with nodeSelector and tolerations to schedule pods on control-plane nodes")
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

			spiffeCsi.Spec.NodeSelector = expectedNodeSelector
			spiffeCsi.Spec.Tolerations = expectedToleration
			err = k8sClient.Update(testCtx, spiffeCsi)
			Expect(err).NotTo(HaveOccurred(), "failed to patch spiffe csi object with nodeSelector and tolerations")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting spiffe csi nodeSelector and tolerations modification")
				agent := &operatorv1alpha1.SpireAgent{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, agent); err == nil {
					agent.Spec.NodeSelector = nil
					agent.Spec.Tolerations = nil
					k8sClient.Update(ctx, agent)
				}
			})

			By("Restarting operator Pod")
			err = clientset.CoreV1().Pods(utils.OperatorNamespace).DeleteCollection(testCtx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: utils.OperatorLabelSelector,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for spiffi csi DaemonSet rolling update to start")
			utils.WaitForDaemonSetRollingUpdate(testCtx, clientset, utils.SpireCSIDriverDaemonSetName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for spiffi csi DaemonSet to become Available")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpireCSIDriverDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if spiffi csi Pods have been scheduled to Nodes with required labels")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpiffeCsiDriverPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).NotTo(BeEmpty())
			utils.VerifyPodScheduling(testCtx, clientset, pods.Items, expectedNodeSelector)

			By("Verifying if spiffe csi Pods tolerate Node taints correctly")
			utils.VerifyPodTolerations(testCtx, clientset, pods.Items, expectedToleration)
		})

		It("Spiffe csi affinity can be configured through CR", func() {
			By("Retrieving any Spiffe csi Pod and its Node for affinity testing")
			pods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpiffeCsiDriverPodLabel})
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

			By("Getting spiffe csi object")
			spireAgent := &operatorv1alpha1.SpiffeCSIDriver{}
			err = k8sClient.Get(testCtx, client.ObjectKey{Name: "cluster"}, spireAgent)
			Expect(err).NotTo(HaveOccurred(), "failed to get spiffe csi object")

			// record initial generation of the DaemonSet before updating SpireAgent object
			daemonset, err := clientset.AppsV1().DaemonSets(utils.OperatorNamespace).Get(testCtx, utils.SpireCSIDriverDaemonSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			initialGen := daemonset.Generation

			By("Patching spiffe csi object with NodeAffinity configuration")
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
			Expect(err).NotTo(HaveOccurred(), "failed to patch spiffe csi object with affinity")
			DeferCleanup(func(ctx context.Context) {
				By("Resetting spiffe csi affinity modification")
				agent := &operatorv1alpha1.SpireAgent{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, agent); err == nil {
					agent.Spec.Affinity = nil
					agent.Spec.Tolerations = nil
					k8sClient.Update(ctx, agent)
				}
			})

			By("Restarting operator Pod")
			err = clientset.CoreV1().Pods(utils.OperatorNamespace).DeleteCollection(testCtx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: utils.OperatorLabelSelector,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for spiffe csi DaemonSet rolling update to start")
			utils.WaitForDaemonSetRollingUpdate(testCtx, clientset, utils.SpireCSIDriverDaemonSetName, utils.OperatorNamespace, initialGen, utils.ShortTimeout)

			By("Waiting for spiffe csi DaemonSet to become Available")
			utils.WaitForDaemonSetAvailable(testCtx, clientset, utils.SpireCSIDriverDaemonSetName, utils.OperatorNamespace, utils.DefaultTimeout)

			By("Verifying if spiffe csi Pod is constrained to the labeled Node")
			newPods, err := clientset.CoreV1().Pods(utils.OperatorNamespace).List(testCtx, metav1.ListOptions{LabelSelector: utils.SpiffeCsiDriverPodLabel})
			Expect(err).NotTo(HaveOccurred())
			Expect(newPods.Items).To(HaveLen(1), "nodeAffinity should constrain daemonset to exactly one pod")
			Expect(newPods.Items[0].Spec.NodeName).To(Equal(targetNodeName), "pod should be scheduled on the labeled node")
			fmt.Fprintf(GinkgoWriter, "pod has been constrained to labeled node '%s'\n", targetNodeName)
		})

	})

})
