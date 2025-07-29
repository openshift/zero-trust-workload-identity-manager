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

package utils

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetTestDir returns the directory to write test results to
func GetTestDir() string {
	// Test is running in the Prow CI, use ARTIFACT_DIR environment variable
	if os.Getenv("OPENSHIFT_CI") == "true" {
		return os.Getenv("ARTIFACT_DIR")
	}

	return "/tmp"
}

// GetKubeConfig returns the Kubernetes configuration
func GetKubeConfig() (*rest.Config, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		return nil, fmt.Errorf("KUBECONFIG environment variable is not set")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from KUBECONFIG: %w", err)
	}

	return config, nil
}

// GetClusterBaseDomain gets the cluster base domain from the DNS cluster object
func GetClusterBaseDomain(ctx context.Context, configClient configv1.ConfigV1Interface) (string, error) {
	dns, err := configClient.DNSes().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get base domain from DNS cluster object: %w", err)
	}

	return dns.Spec.BaseDomain, nil
}

// IsCRDEstablished checks if a CRD is Established
func IsCRDEstablished(crd *apiextv1.CustomResourceDefinition) bool {
	// Check if the CRD has the Established condition set to True
	for _, condition := range crd.Status.Conditions {
		if condition.Type == apiextv1.Established && condition.Status == apiextv1.ConditionTrue {
			return true
		}
	}

	return false
}

// WaitForCRDEstablished waits for a CRD to be Established within timeout
func WaitForCRDEstablished(ctx context.Context, apiextClient apiextclient.Interface, name string, timeout time.Duration) {
	Eventually(func() bool {
		crd, err := apiextClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "failed to get CRD '%s': %v\n", name, err)
			return false
		}

		if !IsCRDEstablished(crd) {
			fmt.Fprintf(GinkgoWriter, "CRD '%s' not established yet\n", name)
			return false
		}

		fmt.Fprintf(GinkgoWriter, "CRD '%s' is established\n", name)
		return true
	}).WithTimeout(timeout).WithPolling(ShortInterval).Should(BeTrue(),
		"CRD '%s' should be established within %v", name, timeout)
}

// IsDeploymentAvailable checks if a Deployment has the Available condition set to True
func IsDeploymentAvailable(deployment *appsv1.Deployment) bool {
	// Check if deployment has Available condition set to True
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentAvailable && condition.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}

// WaitForDeploymentAvailable waits for a Deployment to become Available within timeout
func WaitForDeploymentAvailable(ctx context.Context, clientset kubernetes.Interface, name, namespace string, timeout time.Duration) {
	Eventually(func() bool {
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "failed to get deployment '%s/%s': %v\n", namespace, name, err)
			return false
		}

		if !IsDeploymentAvailable(deployment) {
			fmt.Fprintf(GinkgoWriter, "deployment '%s/%s' not available yet\n", namespace, name)
			return false
		}

		fmt.Fprintf(GinkgoWriter, "deployment '%s/%s' is available\n", namespace, name)
		return true
	}).WithTimeout(timeout).WithPolling(DefaultInterval).Should(BeTrue(),
		"deployment '%s/%s' should become available within %v", namespace, name, timeout)
}

// IsStatefulSetReady checks if a StatefulSet is Ready
func IsStatefulSetReady(sts *appsv1.StatefulSet) bool {
	return sts.Status.ReadyReplicas == *sts.Spec.Replicas && sts.Status.CurrentReplicas == *sts.Spec.Replicas
}

// WaitForStatefulSetReady waits for a StatefulSet to be Ready within timeout
func WaitForStatefulSetReady(ctx context.Context, clientset kubernetes.Interface, name, namespace string, timeout time.Duration) {
	Eventually(func() bool {
		sts, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "failed to get statefulset '%s/%s': %v\n", namespace, name, err)
			return false
		}

		if !IsStatefulSetReady(sts) {
			fmt.Fprintf(GinkgoWriter, "statefulset '%s/%s' not ready yet (%d/%d replicas ready)\n", namespace, name, sts.Status.ReadyReplicas, *sts.Spec.Replicas)
			return false
		}

		fmt.Fprintf(GinkgoWriter, "statefulset '%s/%s' is ready (%d/%d replicas)\n", namespace, name, sts.Status.ReadyReplicas, *sts.Spec.Replicas)
		return true
	}).WithTimeout(timeout).WithPolling(DefaultInterval).Should(BeTrue(),
		"statefulset '%s/%s' should become ready within %v", namespace, name, timeout)
}

// WaitForStatefulSetRollingUpdate waits for a StatefulSet rolling update to be processed by the controller
// This ensures the controller has observed the changes, whether the update is in progress or already completed
// initialGeneration should be recorded before making any changes to the StatefulSet
func WaitForStatefulSetRollingUpdate(ctx context.Context, clientset kubernetes.Interface, name, namespace string, initialGeneration int64, timeout time.Duration) {
	Eventually(func() bool {
		sts, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "failed to get statefulset '%s/%s': %v\n", namespace, name, err)
			return false
		}

		// Check if generation has increased and controller has observed it
		if sts.Generation > initialGeneration && sts.Status.ObservedGeneration >= sts.Generation {
			fmt.Fprintf(GinkgoWriter, "statefulset '%s/%s' rolling update processed (generation %d->%d)\n", namespace, name, initialGeneration, sts.Generation)
			return true
		}

		fmt.Fprintf(GinkgoWriter, "statefulset '%s/%s' waiting for rolling update (gen=%d->%d, observed=%d)\n", namespace, name, initialGeneration, sts.Generation, sts.Status.ObservedGeneration)
		return false
	}).WithTimeout(timeout).WithPolling(ShortInterval).Should(BeTrue(),
		"statefulset '%s/%s' rolling update should be processed within %v", namespace, name, timeout)
}

// IsDaemonSetAvailable checks if a DaemonSet has all desired pods Up-to-date and Available
func IsDaemonSetAvailable(ds *appsv1.DaemonSet) bool {
	desired := ds.Status.DesiredNumberScheduled
	return desired > 0 && ds.Status.NumberAvailable == desired && ds.Status.UpdatedNumberScheduled == desired
}

// WaitForDaemonSetAvailable waits for a DaemonSet to have all desired pods available within timeout
func WaitForDaemonSetAvailable(ctx context.Context, clientset kubernetes.Interface, name, namespace string, timeout time.Duration) {
	Eventually(func() bool {
		ds, err := clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "failed to get daemonset '%s/%s': %v\n", namespace, name, err)
			return false
		}

		if !IsDaemonSetAvailable(ds) {
			fmt.Fprintf(GinkgoWriter, "daemonset '%s/%s' not available yet (%d/%d pods available)\n", namespace, name, ds.Status.NumberAvailable, ds.Status.DesiredNumberScheduled)
			return false
		}

		fmt.Fprintf(GinkgoWriter, "daemonset '%s/%s' is available (%d/%d pods)\n", namespace, name, ds.Status.NumberAvailable, ds.Status.DesiredNumberScheduled)
		return true
	}).WithTimeout(timeout).WithPolling(DefaultInterval).Should(BeTrue(),
		"daemonset '%s/%s' should become available within %v", namespace, name, timeout)
}

// WaitForDaemonSetRollingUpdate waits for a DaemonSet rolling update to be processed by the controller
// This ensures the controller has observed the changes, whether the update is in progress or already completed
// initialGeneration should be recorded before making any changes to the DaemonSet
func WaitForDaemonSetRollingUpdate(ctx context.Context, clientset kubernetes.Interface, name, namespace string, initialGeneration int64, timeout time.Duration) {
	Eventually(func() bool {
		ds, err := clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "failed to get daemonset '%s/%s': %v\n", namespace, name, err)
			return false
		}

		// Check if generation has increased and controller has observed it
		if ds.Generation > initialGeneration && ds.Status.ObservedGeneration >= ds.Generation {
			fmt.Fprintf(GinkgoWriter, "daemonset '%s/%s' rolling update processed (generation %d->%d)\n", namespace, name, initialGeneration, ds.Generation)
			return true
		}

		fmt.Fprintf(GinkgoWriter, "daemonset '%s/%s' waiting for rolling update (gen=%d->%d, observed=%d)\n", namespace, name, initialGeneration, ds.Generation, ds.Status.ObservedGeneration)
		return false
	}).WithTimeout(timeout).WithPolling(ShortInterval).Should(BeTrue(),
		"daemonset '%s/%s' rolling update should be processed within %v", namespace, name, timeout)
}

// WaitForCRConditionsTrue waits for all required conditions of the operator managed cluster CR object to be True within timeout
func WaitForCRConditionsTrue(ctx context.Context, k8sClient client.Client, cr client.Object, requiredConditionTypes []string, timeout time.Duration) {
	Eventually(func() bool {
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, cr); err != nil {
			fmt.Fprintf(GinkgoWriter, "failed to get object '%T': %v\n", cr, err)
			return false
		}

		// Use reflection to get .Status.Conditions []metav1.Condition
		statusField := reflect.ValueOf(cr).Elem().FieldByName("Status")
		conditionsField := statusField.FieldByName("Conditions")
		conditions, _ := conditionsField.Interface().([]metav1.Condition)

		conditionMap := make(map[string]metav1.Condition)
		for _, condition := range conditions {
			conditionMap[condition.Type] = condition
		}

		var notTrue []string
		for _, required := range requiredConditionTypes {
			if condition, exists := conditionMap[required]; !exists {
				notTrue = append(notTrue, fmt.Sprintf("{Type '%s' missing}", required))
			} else if condition.Status != metav1.ConditionTrue {
				notTrue = append(notTrue, fmt.Sprintf("{%v}", condition))
			}
		}

		if len(notTrue) > 0 {
			fmt.Fprintf(GinkgoWriter, "not all conditions are in true status yet: %v\n", notTrue)
			return false
		}

		fmt.Fprintf(GinkgoWriter, "all conditions are true: %v\n", requiredConditionTypes)
		return true
	}).WithTimeout(timeout).WithPolling(ShortInterval).Should(BeTrue(),
		"all conditions of '%T' object should be true within %v", cr, timeout)
}

// VerifyContainerResources verifies that all containers in the provided pods have the expected resource limits and requests
func VerifyContainerResources(pods []corev1.Pod, expectedResources *corev1.ResourceRequirements) {
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			// Verify limits
			if expectedResources.Limits != nil {
				for resourceName, expectedQuantity := range expectedResources.Limits {
					actualQuantity := container.Resources.Limits[resourceName]
					Expect(actualQuantity.String()).To(Equal(expectedQuantity.String()),
						"resource limit '%s' should be '%s' for container '%s' in pod '%s'", resourceName, expectedQuantity.String(), container.Name, pod.Name)
				}
			}

			// Verify requests
			if expectedResources.Requests != nil {
				for resourceName, expectedQuantity := range expectedResources.Requests {
					actualQuantity := container.Resources.Requests[resourceName]
					Expect(actualQuantity.String()).To(Equal(expectedQuantity.String()),
						"resource request '%s' should be '%s' for container '%s' in pod '%s'", resourceName, expectedQuantity.String(), container.Name, pod.Name)
				}
			}

			fmt.Fprintf(GinkgoWriter, "container '%s' in pod '%s' has expected resources\n", container.Name, pod.Name)
		}
	}
}

// VerifyPodScheduling verifies that pods are scheduled to nodes with the required nodeSelector labels
func VerifyPodScheduling(ctx context.Context, clientset kubernetes.Interface, pods []corev1.Pod, requiredNodeLabels map[string]string) {
	for _, pod := range pods {
		// Get the node where the pod is scheduled
		node, err := clientset.CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to get node '%s'", pod.Spec.NodeName)

		// Check if the node has all required labels from nodeSelector
		nodeLabels := node.GetLabels()
		for labelKey, expectedValue := range requiredNodeLabels {
			if expectedValue == "" {
				Expect(nodeLabels).To(HaveKey(labelKey),
					"pod %s is scheduled on node '%s' which should have label '%s'", pod.Name, pod.Spec.NodeName, labelKey)
			} else {
				Expect(nodeLabels).To(HaveKeyWithValue(labelKey, expectedValue),
					"pod %s is scheduled on node '%s' which should have label '%s=%s'", pod.Name, pod.Spec.NodeName, labelKey, expectedValue)
			}
		}

		fmt.Fprintf(GinkgoWriter, "pod '%s' is scheduled on node '%s' with required labels [%v]\n", pod.Name, pod.Spec.NodeName, requiredNodeLabels)
	}
}

// VerifyPodTolerations verifies that pods are scheduled to nodes that have taints matching the pod's tolerations
func VerifyPodTolerations(ctx context.Context, clientset kubernetes.Interface, pods []corev1.Pod, expectedTolerations []*corev1.Toleration) {
	for _, pod := range pods {
		// Get the node where the pod is scheduled
		node, err := clientset.CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to get node '%s'", pod.Spec.NodeName)

		// Check if the node has taints that match the pod's tolerations
		nodeTaints := node.Spec.Taints
		for _, expectedToleration := range expectedTolerations {
			tolerationMatched := false
			for _, taint := range nodeTaints {
				if taint.Key == expectedToleration.Key && taint.Effect == expectedToleration.Effect {
					tolerationMatched = true
					break
				}
			}

			if tolerationMatched {
				fmt.Fprintf(GinkgoWriter, "pod '%s' is scheduled on node '%s' with matched toleration [%v]\n", pod.Name, pod.Spec.NodeName, expectedToleration)
			}

			// Note that we don't fail if the taint is not found, as tolerations allow scheduling to nodes both with and without the taint
		}
	}
}
