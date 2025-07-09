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
