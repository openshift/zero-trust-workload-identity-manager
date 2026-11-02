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
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"
	operatorv1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
					Command: []string{"sh", "-c", fmt.Sprintf(`
while [ ! -f /certs/svid.pem ]; do sleep 2; done
if ! command -v openssl >/dev/null 2>&1; then
  echo "openssl not found in container image"
  exit 1
fi
while [ ! -f /certs/mtls-ca.pem ]; do sleep 2; done
echo "Certs available, starting TLS server..."
openssl s_server \
  -cert /certs/svid.pem \
  -key /certs/svid_key.pem \
  -CAfile /certs/mtls-ca.pem \
  -Verify 1 \
  -accept %d \
  -quiet
`, MTLSServerPort)},
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

// FederationSpiffeHelperConfig returns spiffe-helper config for cross-cluster federation mTLS tests.
func FederationSpiffeHelperConfig() SpiffeHelperConfig {
	cfg := DefaultAttestationSpiffeHelperConfig()
	cfg.IncludeFederatedDomains = true
	return cfg
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
		Data:       map[string]string{"helper.conf": FederationSpiffeHelperConfig().String()},
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
			ClassName: SpireControllerManagerClass,
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
// Trust domain comes from the ZTWIM CR; appsDomain is used for the JWT issuer URL.
func CreateFederationSpireServer(ctx context.Context, k8sClient client.Client, appsDomain string) {
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
				MaxOpenConns:     100,
				MaxIdleConns:     2,
				ConnMaxLifetime:  3600,
				DisableMigration: "false",
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

// NewFederationClusterFederatedTrustDomain builds a ClusterFederatedTrustDomain for cross-cluster federation.
// trustDomainBundle must contain the remote trust domain bundle in SPIFFE JWKS format when using https_spiffe,
// which bootstraps SPIFFE authentication for the first bundle fetch.
func NewFederationClusterFederatedTrustDomain(name, remoteTrustDomain, bundleRouteHost, trustDomainBundle string) *spiffev1alpha1.ClusterFederatedTrustDomain {
	spec := spiffev1alpha1.ClusterFederatedTrustDomainSpec{
		TrustDomain:       remoteTrustDomain,
		BundleEndpointURL: fmt.Sprintf("https://%s:%d", bundleRouteHost, FederationRoutePort),
		BundleEndpointProfile: spiffev1alpha1.BundleEndpointProfile{
			Type:             spiffev1alpha1.HTTPSSPIFFEProfileType,
			EndpointSPIFFEID: fmt.Sprintf("spiffe://%s/spire/server", remoteTrustDomain),
		},
		ClassName:         SpireControllerManagerClass,
		TrustDomainBundle: trustDomainBundle,
	}
	return &spiffev1alpha1.ClusterFederatedTrustDomain{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       spec,
	}
}

// GetServerLocalTrustBundle returns the local SPIRE server trust bundle in SPIFFE JWKS format.
func GetServerLocalTrustBundle(ctx context.Context, clientset kubernetes.Interface, kubeconfig string) string {
	By("Exporting local SPIRE server trust bundle")
	var bundle string
	Eventually(func() error {
		output, err := showServerLocalTrustBundle(ctx, clientset, kubeconfig)
		if err != nil {
			return err
		}
		if strings.TrimSpace(output) == "" {
			return fmt.Errorf("local trust bundle output is empty")
		}
		bundle = output
		return nil
	}).WithTimeout(DefaultTimeout).WithPolling(DefaultInterval).Should(Succeed(),
		"local SPIRE server trust bundle should be available")
	return bundle
}

func showServerLocalTrustBundle(ctx context.Context, clientset kubernetes.Interface, kubeconfig string) (string, error) {
	return showServerTrustBundle(ctx, clientset, kubeconfig, "spiffe")
}

func showServerLocalTrustBundlePEM(ctx context.Context, clientset kubernetes.Interface, kubeconfig string) (string, error) {
	return showServerTrustBundle(ctx, clientset, kubeconfig, "pem")
}

func showServerTrustBundle(ctx context.Context, clientset kubernetes.Interface, kubeconfig, format string) (string, error) {
	podName, err := GetSpireServerPodName(ctx, clientset)
	if err != nil {
		return "", err
	}
	command := []string{
		"/opt/spire/bin/spire-server", "bundle", "show",
		"-format", format,
		"-socketPath", SpireServerAPISocket,
	}
	if kubeconfig == "" {
		return execInPodCapture(ctx, OperatorNamespace, podName, "spire-server", command)
	}
	return execInPodCaptureWithKubeconfig(ctx, kubeconfig, OperatorNamespace, podName, "spire-server", command)
}

// RefreshServerFederatedBundle triggers an immediate bundle fetch for a remote trust domain.
func RefreshServerFederatedBundle(ctx context.Context, clientset kubernetes.Interface, kubeconfig, remoteTrustDomain string) error {
	podName, err := GetSpireServerPodName(ctx, clientset)
	if err != nil {
		return err
	}
	command := []string{
		"/opt/spire/bin/spire-server", "federation", "refresh",
		"-id", fmt.Sprintf("spiffe://%s", remoteTrustDomain),
		"-socketPath", SpireServerAPISocket,
	}
	if kubeconfig == "" {
		_, err = execInPodCapture(ctx, OperatorNamespace, podName, "spire-server", command)
	} else {
		_, err = execInPodCaptureWithKubeconfig(ctx, kubeconfig, OperatorNamespace, podName, "spire-server", command)
	}
	return err
}

// serverBundleListContainsTrustDomain reports whether bundle list output references a trust domain.
func serverBundleListContainsTrustDomain(output, trustDomain string) bool {
	candidates := []string{
		trustDomain,
		fmt.Sprintf("spiffe://%s", trustDomain),
	}
	for _, candidate := range candidates {
		if strings.Contains(output, candidate) {
			return true
		}
	}
	return false
}

// GetSpireServerPodName returns the name of a Running SPIRE server pod.
func GetSpireServerPodName(ctx context.Context, clientset kubernetes.Interface) (string, error) {
	return getRunningPodName(ctx, clientset, SpireServerPodLabel, "SPIRE server")
}

func getRunningPodName(ctx context.Context, clientset kubernetes.Interface, labelSelector, role string) (string, error) {
	pods, err := clientset.CoreV1().Pods(OperatorNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return "", err
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			return pod.Name, nil
		}
	}
	if len(pods.Items) > 0 {
		return pods.Items[0].Name, nil
	}
	return "", fmt.Errorf("no %s pods found in namespace %s", role, OperatorNamespace)
}

// ListServerFederatedBundles lists federated bundles from a SPIRE server pod.
func ListServerFederatedBundles(ctx context.Context, clientset kubernetes.Interface, kubeconfig string) (string, error) {
	podName, err := GetSpireServerPodName(ctx, clientset)
	if err != nil {
		return "", err
	}

	command := []string{
		"/opt/spire/bin/spire-server", "bundle", "list",
		"-format", "spiffe",
		"-socketPath", SpireServerAPISocket,
	}
	if kubeconfig == "" {
		return execInPodCapture(ctx, OperatorNamespace, podName, "spire-server", command)
	}
	return execInPodCaptureWithKubeconfig(ctx, kubeconfig, OperatorNamespace, podName, "spire-server", command)
}

func execInPodCapture(ctx context.Context, namespace, podName, containerName string, command []string) (string, error) {
	stdout, stderr, err := ExecInPod(ctx, namespace, podName, containerName, command)
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr))
	}
	return stdout, nil
}

func execInPodCaptureWithKubeconfig(ctx context.Context, kubeconfig, namespace, podName, containerName string, command []string) (string, error) {
	stdout, stderr, err := ExecInPodWithKubeconfig(ctx, kubeconfig, namespace, podName, containerName, command)
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr))
	}
	return stdout, nil
}

// WaitForServerFederatedBundle waits until the SPIRE server has fetched a remote trust domain bundle.
func WaitForServerFederatedBundle(ctx context.Context, clientset kubernetes.Interface, kubeconfig, remoteTrustDomain string, timeout time.Duration) {
	By(fmt.Sprintf("Waiting for federated bundle %s on SPIRE server", remoteTrustDomain))
	Eventually(func() bool {
		output, err := ListServerFederatedBundles(ctx, clientset, kubeconfig)
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "spire-server bundle list failed: %v\n", err)
			return false
		}
		hasBundle := serverBundleListContainsTrustDomain(output, remoteTrustDomain)
		if hasBundle {
			return true
		}

		if err := RefreshServerFederatedBundle(ctx, clientset, kubeconfig, remoteTrustDomain); err != nil {
			fmt.Fprintf(GinkgoWriter, "spire-server bundle refresh for %s failed: %v\n", remoteTrustDomain, err)
		}

		output, err = ListServerFederatedBundles(ctx, clientset, kubeconfig)
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "spire-server bundle list after refresh failed: %v\n", err)
			return false
		}
		hasBundle = serverBundleListContainsTrustDomain(output, remoteTrustDomain)
		if !hasBundle {
			fmt.Fprintf(GinkgoWriter, "SPIRE server does not yet have bundle for %s; bundle list output:\n%s\n", remoteTrustDomain, output)
		}
		return hasBundle
	}).WithTimeout(timeout).WithPolling(30 * time.Second).Should(BeTrue(),
		"SPIRE server should have federated bundle for %s", remoteTrustDomain)
}

// CreateMTLSServerRoute exposes the mTLS server Service on Cluster B via passthrough TLS.
func CreateMTLSServerRoute(ctx context.Context, k8sClient client.Client, namespace, routeName, serviceName string) *routev1.Route {
	By(fmt.Sprintf("Creating passthrough Route %s/%s for mTLS server", namespace, routeName))
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: namespace,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind:   "Service",
				Name:   serviceName,
				Weight: ptr.To(int32(100)),
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString("tls"),
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationPassthrough,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
			WildcardPolicy: routev1.WildcardPolicyNone,
		},
	}
	Expect(k8sClient.Create(ctx, route)).To(Succeed(), "failed to create mTLS server route")
	return route
}

// ErrMTLSOpenSSLTimeout indicates openssl s_client was killed by timeout(1) (exit 124).
var ErrMTLSOpenSSLTimeout = errors.New("openssl s_client timed out")

// AttemptMTLSConnection executes an openssl s_client command from the client pod to test mTLS.
// Uses the default KUBECONFIG (Cluster A). Returns stdout, stderr, and error.
func AttemptMTLSConnection(ctx context.Context, namespace, podName, serverHost string, serverPort int) (string, string, error) {
	cmd := []string{
		"sh", "-c",
		fmt.Sprintf(
			`CAFILE=%q; [ -f "$CAFILE" ] || CAFILE=/certs/bundle.pem; timeout %d openssl s_client -connect %s:%d -servername %s -cert /certs/svid.pem -key /certs/svid_key.pem -CAfile "$CAFILE" -verify_return_error -brief </dev/null 2>&1; echo "EXIT_CODE=$?"`,
			MTLSCombinedCAPath, MTLSOpenSSLTimeoutSeconds, serverHost, serverPort, serverHost,
		),
	}
	stdout, stderr, err := ExecInPod(ctx, namespace, podName, "tls-client", cmd)
	if err != nil {
		return stdout, stderr, err
	}
	exitCode, ok := parseShellExitCode(stdout)
	if !ok {
		return stdout, stderr, fmt.Errorf("mTLS check output missing EXIT_CODE marker")
	}
	if exitCode == 124 {
		return stdout, stderr, ErrMTLSOpenSSLTimeout
	}
	if exitCode != 0 {
		return stdout, stderr, fmt.Errorf("openssl s_client exited with code %d", exitCode)
	}
	return stdout, stderr, nil
}

func parseShellExitCode(stdout string) (int, bool) {
	const prefix = "EXIT_CODE="
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			code, err := strconv.Atoi(strings.TrimPrefix(line, prefix))
			if err != nil {
				return 0, false
			}
			return code, true
		}
	}
	return 0, false
}

// ClearMTLSCombinedCA removes federated trust material written for cross-cluster mTLS tests.
func ClearMTLSCombinedCA(ctx context.Context, namespace, podName, containerName string) error {
	command := []string{"sh", "-c", fmt.Sprintf("rm -f %q", MTLSCombinedCAPath)}
	_, err := execInPodCapture(ctx, namespace, podName, containerName, command)
	return err
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

// GetServerAllTrustBundlesPEM exports local and federated trust bundles from a SPIRE server in PEM format.
// bundle show returns the local trust domain bundle; bundle list returns federated bundles only.
func GetServerAllTrustBundlesPEM(ctx context.Context, clientset kubernetes.Interface, kubeconfig string) string {
	By("Exporting SPIRE server trust bundles (local + federated) in PEM format")
	var bundle string
	Eventually(func() error {
		localPEM, err := showServerLocalTrustBundlePEM(ctx, clientset, kubeconfig)
		if err != nil {
			return err
		}
		federatedPEM, err := listServerTrustBundlesPEM(ctx, clientset, kubeconfig)
		if err != nil {
			return err
		}

		var combined strings.Builder
		combined.WriteString(strings.TrimSpace(localPEM))
		if federated := strings.TrimSpace(federatedPEM); federated != "" {
			combined.WriteString("\n")
			combined.WriteString(federated)
		}
		output := combined.String()
		if strings.TrimSpace(output) == "" {
			return fmt.Errorf("trust bundle PEM output is empty")
		}
		certs, err := ParseAllPEMCertificates(output)
		if err != nil {
			return fmt.Errorf("trust bundle PEM is not parseable: %w", err)
		}
		if len(certs) < 2 {
			return fmt.Errorf("expected local and federated CA certificates, got %d", len(certs))
		}
		bundle = output
		return nil
	}).WithTimeout(DefaultTimeout).WithPolling(DefaultInterval).Should(Succeed(),
		"SPIRE server should expose local and federated trust bundles in PEM format")
	return bundle
}

func listServerTrustBundlesPEM(ctx context.Context, clientset kubernetes.Interface, kubeconfig string) (string, error) {
	podName, err := GetSpireServerPodName(ctx, clientset)
	if err != nil {
		return "", err
	}
	command := []string{
		"/opt/spire/bin/spire-server", "bundle", "list",
		"-format", "pem",
		"-socketPath", SpireServerAPISocket,
	}
	if kubeconfig == "" {
		return execInPodCapture(ctx, OperatorNamespace, podName, "spire-server", command)
	}
	return execInPodCaptureWithKubeconfig(ctx, kubeconfig, OperatorNamespace, podName, "spire-server", command)
}

// PrepareMTLSCombinedCA writes a CA file containing local and federated trust bundles for cross-cluster mTLS.
func PrepareMTLSCombinedCA(ctx context.Context, namespace, podName, containerName, podKubeconfig string, serverClientset kubernetes.Interface, serverKubeconfig string) {
	By(fmt.Sprintf("Preparing combined CA bundle in %s/%s from SPIRE server trust bundles", namespace, podName))
	allBundlesPEM := GetServerAllTrustBundlesPEM(ctx, serverClientset, serverKubeconfig)
	bundleB64 := base64.StdEncoding.EncodeToString([]byte(allBundlesPEM))
	command := []string{
		"sh", "-c",
		fmt.Sprintf(
			`echo %q | base64 -d > %q && test -s %q`,
			bundleB64, MTLSCombinedCAPath, MTLSCombinedCAPath,
		),
	}
	var err error
	if podKubeconfig == "" {
		_, err = execInPodCapture(ctx, namespace, podName, containerName, command)
	} else {
		_, err = execInPodCaptureWithKubeconfig(ctx, podKubeconfig, namespace, podName, containerName, command)
	}
	Expect(err).NotTo(HaveOccurred(), "failed to prepare combined CA bundle in %s/%s", namespace, podName)
}

// WorkloadBundleCACount returns the number of CA certificates in the active client trust store
// (mtls-ca.pem when present, otherwise spiffe-helper bundle.pem).
func WorkloadBundleCACount(ctx context.Context, namespace, podName, containerName string) int {
	command := []string{"sh", "-c", fmt.Sprintf(`if [ -f %q ]; then cat %q; else cat /certs/bundle.pem; fi`, MTLSCombinedCAPath, MTLSCombinedCAPath)}
	return countPEMCertsFromPod(ctx, namespace, podName, containerName, command)
}

// WorkloadSPIFFEBundleCACount returns CA certificates in spiffe-helper bundle.pem only.
func WorkloadSPIFFEBundleCACount(ctx context.Context, namespace, podName, containerName string) int {
	return countPEMCertsFromPod(ctx, namespace, podName, containerName, []string{"cat", "/certs/bundle.pem"})
}

func countPEMCertsFromPod(ctx context.Context, namespace, podName, containerName string, command []string) int {
	bundlePEM, err := execInPodCapture(ctx, namespace, podName, containerName, command)
	if err != nil {
		return 0
	}
	certs, err := ParseAllPEMCertificates(bundlePEM)
	if err != nil {
		return 0
	}
	return len(certs)
}

// ExpectServerCombinedTrustBundlesPEM asserts the SPIRE server exposes local and federated CA PEMs.
// Agents source federated material from the server for SDS buildAll/ROOTCA handling.
func ExpectServerCombinedTrustBundlesPEM(ctx context.Context, clientset kubernetes.Interface, kubeconfig string, minCACerts int) {
	pem := GetServerAllTrustBundlesPEM(ctx, clientset, kubeconfig)
	certs, err := ParseAllPEMCertificates(pem)
	Expect(err).NotTo(HaveOccurred(), "SPIRE server combined trust bundle PEM should be parseable")
	Expect(len(certs)).To(BeNumerically(">=", minCACerts),
		"SPIRE server should expose at least %d CA certificates (local + federated)", minCACerts)
}

// IsMTLSExecFailure reports whether an AttemptMTLSConnection error is from oc/kubectl exec rather than TLS verification.
func IsMTLSExecFailure(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "exec ")
}

// IsMTLSTLSVerifyFailure reports whether openssl failed certificate verification.
func IsMTLSTLSVerifyFailure(stdout string, err error) bool {
	if strings.Contains(stdout, "verify error") {
		return true
	}
	exitCode, ok := parseShellExitCode(stdout)
	return ok && exitCode == 1 && strings.Contains(stdout, "certificate verify failed")
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
