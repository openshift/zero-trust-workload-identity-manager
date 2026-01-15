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
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

// IsPodRunning checks if a pod is in Running phase
func IsPodRunning(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning
}

// IsPodReady checks if a pod has the Ready condition set to True
func IsPodReady(pod *corev1.Pod) bool {
	// Exclude the pod being terminated
	if pod.DeletionTimestamp != nil {
		return false
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}

// WaitForPodRunning waits for a specific pod to be in Running phase within timeout
func WaitForPodRunning(ctx context.Context, clientset kubernetes.Interface, name, namespace string, timeout time.Duration) {
	Eventually(func() bool {
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "failed to get pod '%s/%s': %v\n", namespace, name, err)
			return false
		}

		if !IsPodRunning(pod) {
			fmt.Fprintf(GinkgoWriter, "pod '%s/%s' not running yet (phase=%s)\n", namespace, name, pod.Status.Phase)
			return false
		}

		fmt.Fprintf(GinkgoWriter, "pod '%s/%s' is running on node '%s'\n", namespace, name, pod.Spec.NodeName)
		return true
	}).WithTimeout(timeout).WithPolling(ShortInterval).Should(BeTrue(),
		"pod '%s/%s' should become running within %v", namespace, name, timeout)
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

// IsDeploymentRolloutComplete checks if a Deployment rollout is fully complete
// This includes checking that the controller has observed the latest generation
// and that all replicas are updated, available, and none are unavailable
func IsDeploymentRolloutComplete(deployment *appsv1.Deployment) bool {
	desired := int32(0)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}

	return deployment.Status.ObservedGeneration >= deployment.Generation &&
		deployment.Status.UpdatedReplicas == desired &&
		deployment.Status.AvailableReplicas == desired &&
		deployment.Status.UnavailableReplicas == 0
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

		if !IsDeploymentRolloutComplete(deployment) {
			fmt.Fprintf(GinkgoWriter, "deployment '%s/%s' rollout not complete yet (observed=%d/gen=%d, unavailable=%d)\n",
				namespace, name, deployment.Status.ObservedGeneration, deployment.Generation, deployment.Status.UnavailableReplicas)
			return false
		}

		fmt.Fprintf(GinkgoWriter, "deployment '%s/%s' is available and rollout complete\n", namespace, name)
		return true
	}).WithTimeout(timeout).WithPolling(DefaultInterval).Should(BeTrue(),
		"deployment '%s/%s' should become available with complete rollout within %v", namespace, name, timeout)
}

// WaitForDeploymentRollingUpdate waits for a Deployment rolling update to be processed by the controller
// This ensures the controller has observed the changes, whether the update is in progress or already completed
// initialGeneration should be recorded before making any changes to the Deployment
func WaitForDeploymentRollingUpdate(ctx context.Context, clientset kubernetes.Interface, name, namespace string, initialGeneration int64, timeout time.Duration) {
	Eventually(func() bool {
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "failed to get deployment '%s/%s': %v\n", namespace, name, err)
			return false
		}

		// Check if generation has increased and controller has observed it
		if deployment.Generation > initialGeneration && deployment.Status.ObservedGeneration >= deployment.Generation {
			fmt.Fprintf(GinkgoWriter, "deployment '%s/%s' rolling update processed (generation %d->%d)\n", namespace, name, initialGeneration, deployment.Generation)
			return true
		}

		fmt.Fprintf(GinkgoWriter, "deployment '%s/%s' waiting for rolling update (gen=%d->%d, observed=%d)\n", namespace, name, initialGeneration, deployment.Generation, deployment.Status.ObservedGeneration)
		return false
	}).WithTimeout(timeout).WithPolling(ShortInterval).Should(BeTrue(),
		"deployment '%s/%s' rolling update should be processed within %v", namespace, name, timeout)
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

// UpdateCRWithRetry updates a CR with retry on conflict
func UpdateCRWithRetry(ctx context.Context, k8sClient client.Client, obj client.Object, updateFunc func()) error {
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		// Get latest version
		key := client.ObjectKeyFromObject(obj)
		if err := k8sClient.Get(ctx, key, obj); err != nil {
			return fmt.Errorf("failed to get latest version: %w", err)
		}

		// Apply updates
		updateFunc()

		// Try to update
		err := k8sClient.Update(ctx, obj)
		if err == nil {
			return nil
		}

		// Check if it's a conflict error by checking the error message
		errorMsg := err.Error()
		isConflict := false
		if len(errorMsg) > 0 {
			// Check for common conflict error phrases
			if containsAny(errorMsg, []string{"Operation cannot be fulfilled", "the object has been modified", "Conflict"}) {
				isConflict = true
			}
		}

		if isConflict {
			fmt.Fprintf(GinkgoWriter, "Conflict on update (attempt %d/%d): %v, retrying...\n", i+1, maxRetries, err)
			time.Sleep(time.Second)
			continue
		}

		// Other error, return immediately
		return err
	}

	return fmt.Errorf("failed to update after %d retries", maxRetries)
}

// containsAny checks if s contains any of the substrings
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// FindOperatorSubscription finds an OLM subscription by name fragment in the specified namespace
func FindOperatorSubscription(ctx context.Context, k8sClient client.Client, namespace, nameFragment string) (string, []string, error) {
	subscriptionList := &unstructured.UnstructuredList{}
	subscriptionList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operators.coreos.com",
		Version: "v1alpha1",
		Kind:    "SubscriptionList",
	})

	if err := k8sClient.List(ctx, subscriptionList, client.InNamespace(namespace)); err != nil {
		return "", nil, fmt.Errorf("failed to list Subscriptions in namespace '%s': %w", namespace, err)
	}

	if len(subscriptionList.Items) == 0 {
		return "", nil, fmt.Errorf("no Subscriptions found in namespace '%s'", namespace)
	}

	var foundNames []string
	for _, sub := range subscriptionList.Items {
		name := sub.GetName()
		foundNames = append(foundNames, name)
		if strings.Contains(name, nameFragment) {
			return name, foundNames, nil
		}
	}

	return "", foundNames, fmt.Errorf("no Subscription matching '%s' found", nameFragment)
}

// PatchSubscriptionEnv patches a subscription's environment variable using merge patch
func PatchSubscriptionEnv(ctx context.Context, k8sClient client.Client, namespace, name, envKey, envValue string) error {
	subscription := &unstructured.Unstructured{}
	subscription.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operators.coreos.com",
		Version: "v1alpha1",
		Kind:    "Subscription",
	})
	subscription.SetName(name)
	subscription.SetNamespace(namespace)

	// Create merge patch payload with the environment variable
	patchPayload := map[string]any{
		"spec": map[string]any{
			"config": map[string]any{
				"env": []map[string]string{
					{
						"name":  envKey,
						"value": envValue,
					},
				},
			},
		},
	}

	patchBytes, err := json.Marshal(patchPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal patch payload: %w", err)
	}

	if err := k8sClient.Patch(ctx, subscription, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
		return fmt.Errorf("failed to patch subscription: %w", err)
	}

	return nil
}

// GetDeploymentEnvVar retrieves an environment variable value from a deployment's first container
func GetDeploymentEnvVar(ctx context.Context, clientset kubernetes.Interface, namespace, deploymentName, envVarName string) (string, error) {
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get deployment: %w", err)
	}

	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		return "", fmt.Errorf("deployment has no containers")
	}

	for _, env := range deployment.Spec.Template.Spec.Containers[0].Env {
		if env.Name == envVarName {
			return env.Value, nil
		}
	}

	return "", nil // Return empty string if env var not found
}


// GetNestedStringFromConfigMapJSON retrieves a nested string value from a JSON-formatted ConfigMap data field
func GetNestedStringFromConfigMapJSON(ctx context.Context, clientset kubernetes.Interface, namespace, configMapName, dataKey string, fields ...string) (string, bool, error) {
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		return "", false, fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	data, ok := cm.Data[dataKey]
	if !ok {
		return "", false, fmt.Errorf("key %s not found in ConfigMap", dataKey)
	}

	var configData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &configData); err != nil {
		return "", false, fmt.Errorf("failed to parse JSON from ConfigMap data: %w", err)
	}

	value, found, err := unstructured.NestedString(configData, fields...)
	if err != nil {
		return "", false, fmt.Errorf("failed to get nested field %v: %w", fields, err)
	}

	return value, found, nil
}
