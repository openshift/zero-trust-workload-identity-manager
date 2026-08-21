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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	operatorv1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FederationClusterInfo holds cluster-specific details discovered during federation setup.
type FederationClusterInfo struct {
	AppsDomain      string
	TrustDomain     string
	BundleEndpoint  string
	FederationRoute string
}

// NewMTLSServerPod builds a pod that runs a TLS server using SPIRE-issued certificates.
// The server uses openssl s_server to listen with mTLS (mutual TLS verification).
func NewMTLSServerPod(name, namespace, saName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"app": MTLSServerAppLabel},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: saName,
			Containers: []corev1.Container{
				{
					Name:  SpiffeHelperContainerName,
					Image: SpiffeHelperImage,
					Args:  []string{"-config", "/run/spiffe-helper/helper.conf"},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "spiffe-workload-api", MountPath: "/spiffe-workload-api", ReadOnly: true},
						{Name: "certs", MountPath: "/certs"},
						{Name: "spiffe-helper-config", MountPath: "/run/spiffe-helper", ReadOnly: true},
					},
					SecurityContext: restrictedSecurityContext(),
				},
				{
					Name:  "tls-server",
					Image: MTLSServerImage,
					Command: []string{"sh", "-c", `
while [ ! -f /certs/svid.pem ]; do sleep 2; done
echo "Certs available, starting TLS server..."
exec openssl s_server \
  -cert /certs/svid.pem \
  -key /certs/svid_key.pem \
  -CAfile /certs/bundle.pem \
  -Verify 1 \
  -accept 8443 \
  -www \
  -quiet 2>&1 || sleep 3600
`},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "certs", MountPath: "/certs"},
					},
					SecurityContext: restrictedSecurityContext(),
				},
			},
			Volumes: []corev1.Volume{
				{Name: "spiffe-workload-api", VolumeSource: corev1.VolumeSource{CSI: &corev1.CSIVolumeSource{Driver: "csi.spiffe.io", ReadOnly: ptr.To(true)}}},
				{Name: "certs", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
				{Name: "spiffe-helper-config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: SpiffeHelperConfigMapName}}}},
			},
		},
	}
}

// NewMTLSClientPod builds a pod that can act as a TLS client using SPIRE-issued certificates.
func NewMTLSClientPod(name, namespace, saName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"app": MTLSClientAppLabel},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: saName,
			Containers: []corev1.Container{
				{
					Name:  SpiffeHelperContainerName,
					Image: SpiffeHelperImage,
					Args:  []string{"-config", "/run/spiffe-helper/helper.conf"},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "spiffe-workload-api", MountPath: "/spiffe-workload-api", ReadOnly: true},
						{Name: "certs", MountPath: "/certs"},
						{Name: "spiffe-helper-config", MountPath: "/run/spiffe-helper", ReadOnly: true},
					},
					SecurityContext: restrictedSecurityContext(),
				},
				{
					Name:    "tls-client",
					Image:   MTLSServerImage,
					Command: []string{"sleep", "3600"},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "certs", MountPath: "/certs"},
					},
					SecurityContext: restrictedSecurityContext(),
				},
			},
			Volumes: []corev1.Volume{
				{Name: "spiffe-workload-api", VolumeSource: corev1.VolumeSource{CSI: &corev1.CSIVolumeSource{Driver: "csi.spiffe.io", ReadOnly: ptr.To(true)}}},
				{Name: "certs", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
				{Name: "spiffe-helper-config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: SpiffeHelperConfigMapName}}}},
			},
		},
	}
}

// SetupFederationNamespace creates a namespace with proper labels for ClusterSPIFFEID matching
// and the required resources (ServiceAccount, spiffe-helper ConfigMap).
func SetupFederationNamespace(ctx context.Context, k8sClient client.Client, namespace, saName string) {
	By(fmt.Sprintf("Creating federation test namespace %s", namespace))
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   namespace,
			Labels: map[string]string{"kubernetes.io/metadata.name": namespace},
		},
	}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed(), "failed to create namespace %s", namespace)

	By(fmt.Sprintf("Creating ServiceAccount %s/%s", namespace, saName))
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: saName, Namespace: namespace},
	}
	Expect(k8sClient.Create(ctx, sa)).To(Succeed(), "failed to create ServiceAccount %s/%s", namespace, saName)

	By(fmt.Sprintf("Creating spiffe-helper ConfigMap in %s", namespace))
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: SpiffeHelperConfigMapName, Namespace: namespace},
		Data:       map[string]string{"helper.conf": DefaultAttestationSpiffeHelperConfig().String()},
	}
	Expect(k8sClient.Create(ctx, cm)).To(Succeed(), "failed to create spiffe-helper ConfigMap in %s", namespace)
}

// CreateFederationClusterSPIFFEID creates a ClusterSPIFFEID for a federation test namespace.
func CreateFederationClusterSPIFFEID(ctx context.Context, k8sClient client.Client, name, namespace, appLabel string) {
	By(fmt.Sprintf("Creating ClusterSPIFFEID %s for namespace %s", name, namespace))
	cspiffeID := &spiffev1alpha1.ClusterSPIFFEID{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: spiffev1alpha1.ClusterSPIFFEIDSpec{
			SPIFFEIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}",
			PodSelector:      &metav1.LabelSelector{MatchLabels: map[string]string{"app": appLabel}},
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace},
			},
			ClassName: "zero-trust-workload-identity-manager-spire",
		},
	}
	Expect(k8sClient.Create(ctx, cspiffeID)).To(Succeed(), "failed to create ClusterSPIFFEID %s", name)
}

// WaitForSpireReady waits for both SpireServer and SpireAgent to report Ready on a cluster.
func WaitForSpireReady(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface, timeout time.Duration) {
	By("Waiting for SpireServer to become Ready")
	WaitForSpireServerConditions(ctx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
		"Ready": metav1.ConditionTrue,
	}, timeout)

	By("Waiting for SpireAgent to become Ready")
	WaitForSpireAgentConditions(ctx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
		"Ready": metav1.ConditionTrue,
	}, timeout)

	By("Waiting for SPIRE Server StatefulSet to be ready")
	WaitForStatefulSetReady(ctx, clientset, SpireServerStatefulSetName, OperatorNamespace, timeout)

	By("Waiting for SPIRE Agent DaemonSet to be available")
	WaitForDaemonSetAvailable(ctx, clientset, SpireAgentDaemonSetName, OperatorNamespace, timeout)
}

// CreateFederationSpireServer creates the SpireServer CR with federation config enabled.
func CreateFederationSpireServer(ctx context.Context, k8sClient client.Client, trustDomain, appsDomain string) {
	By("Creating SpireServer with federation enabled")
	spireServer := &operatorv1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: operatorv1alpha1.SpireServerSpec{
			JwtIssuer:           fmt.Sprintf("https://oidc-discovery.%s", appsDomain),
			CAValidity:          metav1.Duration{Duration: 24 * time.Hour},
			DefaultX509Validity: metav1.Duration{Duration: 1 * time.Hour},
			DefaultJWTValidity:  metav1.Duration{Duration: 5 * time.Minute},
			CAKeyType:           "rsa-2048",
			CASubject: operatorv1alpha1.CASubject{
				CommonName:   "SPIRE CA",
				Country:      "US",
				Organization: "E2E Test",
			},
			Persistence: operatorv1alpha1.Persistence{
				Size:       "1Gi",
				AccessMode: "ReadWriteOnce",
			},
			Datastore: operatorv1alpha1.DataStore{
				DatabaseType:     "sqlite3",
				ConnectionString: "/run/spire/data/datastore.sqlite3",
			},
			Federation: &operatorv1alpha1.FederationConfig{
				BundleEndpoint: operatorv1alpha1.BundleEndpointConfig{
					Profile:     operatorv1alpha1.HttpsSpiffeProfile,
					RefreshHint: 300,
				},
				ManagedRoute: "true",
			},
		},
	}
	Expect(k8sClient.Create(ctx, spireServer)).To(Succeed(), "failed to create SpireServer with federation config")
}

// CreateFederationSpireAgent creates the SpireAgent CR for federation testing.
func CreateFederationSpireAgent(ctx context.Context, k8sClient client.Client) {
	By("Creating SpireAgent")
	spireAgent := &operatorv1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
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
	Expect(k8sClient.Create(ctx, spireAgent)).To(Succeed(), "failed to create SpireAgent")
}

// CreateFederationSpiffeCSIDriver creates the SpiffeCSIDriver CR for federation testing.
func CreateFederationSpiffeCSIDriver(ctx context.Context, k8sClient client.Client) {
	By("Creating SpiffeCSIDriver")
	csiDriver := &operatorv1alpha1.SpiffeCSIDriver{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec:       operatorv1alpha1.SpiffeCSIDriverSpec{},
	}
	Expect(k8sClient.Create(ctx, csiDriver)).To(Succeed(), "failed to create SpiffeCSIDriver")
}

// CreateFederationZTWIM creates the ZeroTrustWorkloadIdentityManager CR for federation testing.
func CreateFederationZTWIM(ctx context.Context, k8sClient client.Client, trustDomain, clusterName string) {
	By(fmt.Sprintf("Creating ZeroTrustWorkloadIdentityManager with trustDomain=%s, clusterName=%s", trustDomain, clusterName))
	ztwim := &operatorv1alpha1.ZeroTrustWorkloadIdentityManager{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: operatorv1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
			BundleConfigMap: "spire-bundle",
			TrustDomain:     trustDomain,
			ClusterName:     clusterName,
		},
	}
	Expect(k8sClient.Create(ctx, ztwim)).To(Succeed(), "failed to create ZeroTrustWorkloadIdentityManager")
}

// AttemptMTLSConnection executes an openssl s_client command from the client pod to test mTLS.
// Uses the default KUBECONFIG (Cluster A). Returns stdout, stderr, and error.
func AttemptMTLSConnection(ctx context.Context, namespace, podName, serverHost string, serverPort int) (string, string, error) {
	cmd := []string{
		"sh", "-c",
		fmt.Sprintf(
			`echo "FEDERATION-MTLS-TEST" | timeout 15 openssl s_client -connect %s:%d -cert /certs/svid.pem -key /certs/svid_key.pem -CAfile /certs/bundle.pem -verify_return_error -quiet 2>&1; echo "EXIT_CODE=$?"`,
			serverHost, serverPort,
		),
	}
	return ExecInPod(ctx, namespace, podName, "tls-client", cmd)
}

// ExecInPodOnClusterB runs a command in a pod on Cluster B by setting KUBECONFIG_CLUSTER_B.
func ExecInPodOnClusterB(ctx context.Context, namespace, podName, containerName string, command []string) (string, string, error) {
	kubeconfigB := os.Getenv("KUBECONFIG_CLUSTER_B")
	if kubeconfigB == "" {
		return "", "", fmt.Errorf("KUBECONFIG_CLUSTER_B not set")
	}
	return ExecInPodWithKubeconfig(ctx, kubeconfigB, namespace, podName, containerName, command)
}

// WaitForSVIDsReady waits until SVID files appear in /certs/ of the specified pod container
// using the default KUBECONFIG (Cluster A).
func WaitForSVIDsReady(ctx context.Context, namespace, podName, containerName string, timeout time.Duration) {
	By(fmt.Sprintf("Waiting for SVID files in pod %s/%s", namespace, podName))
	Eventually(func() string {
		stdout, _, err := ExecInPod(ctx, namespace, podName, containerName, []string{"ls", "/certs/"})
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "exec ls /certs/ in %s/%s failed: %v\n", namespace, podName, err)
			return ""
		}
		return stdout
	}).WithTimeout(timeout).WithPolling(DefaultInterval).Should(
		And(
			ContainSubstring("svid.pem"),
			ContainSubstring("svid_key.pem"),
			ContainSubstring("bundle.pem"),
		), "SVID files should appear in /certs/ of %s/%s", namespace, podName)
}

// WaitForSVIDsReadyOnClusterB waits until SVID files appear in /certs/ on a Cluster B pod.
func WaitForSVIDsReadyOnClusterB(ctx context.Context, namespace, podName, containerName string, timeout time.Duration) {
	By(fmt.Sprintf("Waiting for SVID files in pod %s/%s on Cluster B", namespace, podName))
	Eventually(func() string {
		stdout, _, err := ExecInPodOnClusterB(ctx, namespace, podName, containerName, []string{"ls", "/certs/"})
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "exec ls /certs/ in %s/%s (Cluster B) failed: %v\n", namespace, podName, err)
			return ""
		}
		return stdout
	}).WithTimeout(timeout).WithPolling(DefaultInterval).Should(
		And(
			ContainSubstring("svid.pem"),
			ContainSubstring("svid_key.pem"),
			ContainSubstring("bundle.pem"),
		), "SVID files should appear in /certs/ of %s/%s on Cluster B", namespace, podName)
}

// restrictedSecurityContext returns the standard restricted PSA-compatible security context.
func restrictedSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		RunAsNonRoot:             ptr.To(true),
		RunAsUser:                ptr.To(int64(1000)),
		SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
}
