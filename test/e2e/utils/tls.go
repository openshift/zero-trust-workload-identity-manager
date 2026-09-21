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
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/component-base/cli/flag"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// goInsecureCipherSuites is the set of cipher suite names classified as insecure by crypto/tls
// (via k8s.io/component-base/cli/flag). Operand configs use IANA names that match this set.
var goInsecureCipherSuites = sets.NewString(flag.InsecureTLSCipherNames()...)

const (
	APIServerTLSProfileIntermediate = "Intermediate"
	APIServerTLSProfileModern       = "Modern"
	APIServerTLSProfileOld          = "Old"

	TLSConfigKey     = "tls_config" // JSON: SPIRE server/agent/OIDC
	TLSConfigYAMLKey = "tlsConfig"  // YAML: controller-manager

	MinTLSVersionK8sTLS12 = "VersionTLS12"
	MinTLSVersionK8sTLS13 = "VersionTLS13"

	SpireControllerManagerConfigMapName   = "spire-controller-manager"
	SpireControllerManagerConfigKey       = "controller-manager-config.yaml"
	SpireServerPodName                    = "spire-server-0"
	SpireServerGRPCPort                   = 8081
	SpireControllerManagerWebhookPort     = 9443
	SpireOIDCDiscoveryProviderServiceName = "spire-spiffe-oidc-discovery-provider"
	SpireOIDCDiscoveryProviderServicePort = 443

	// TLSOpenSSLProbeImage is a UBI image that ships openssl for in-cluster wire checks.
	// Override with TLSPROBE_OPENSSL_IMAGE if the cluster cannot pull this image.
	TLSOpenSSLProbeImage     = "registry.access.redhat.com/ubi9/ubi:latest"
	TLSOpenSSLProbePodName   = "ztwim-tls-openssl-probe"
	TLSOpenSSLProbeContainer = "openssl"

	TLSProfileRolloutTimeout             = 15 * time.Minute
	TLSProfilePatchStabilizationInterval = 30 * time.Second
	// TLSWireProbeTimeout is the retry budget for one openssl door (oc exec + handshake).
	TLSWireProbeTimeout = 5 * time.Minute
	// TLSWireAssertTimeout covers all defaultTLSWireDoors (four doors × TLSWireProbeTimeout).
	TLSWireAssertTimeout = 4 * TLSWireProbeTimeout
)

// ---------------------------------------------------------------------------
// APIServer TLS profile helpers
// ---------------------------------------------------------------------------

// IsAPIServerClusterAccessible reports whether apiservers/cluster can be read.
func IsAPIServerClusterAccessible(ctx context.Context, configClient configv1.ConfigV1Interface) bool {
	_, err := configClient.APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	return err == nil
}

// GetAPIServerTLSProfileType returns the current tlsSecurityProfile.type value.
func GetAPIServerTLSProfileType(ctx context.Context, configClient configv1.ConfigV1Interface) (string, error) {
	apiServer, err := configClient.APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get APIServer cluster config: %w", err)
	}
	if apiServer.Spec.TLSSecurityProfile == nil {
		return "", nil
	}
	return string(apiServer.Spec.TLSSecurityProfile.Type), nil
}

// ExpectedOperandTLSForAPIServerProfile maps an APIServer TLS profile to the expected
// operand ConfigMap injection. ok=false means the profile is not covered (e.g. Custom)
// and the caller should Skip.
func ExpectedOperandTLSForAPIServerProfile(profileType string) (minVersion string, requireCiphers bool, ok bool) {
	switch profileType {
	case "", APIServerTLSProfileIntermediate:
		return MinTLSVersionK8sTLS12, true, true
	case APIServerTLSProfileModern:
		return MinTLSVersionK8sTLS13, false, true
	case APIServerTLSProfileOld:
		// Operands do not take Old; they stay at TLS 1.2 with secure ciphers.
		return MinTLSVersionK8sTLS12, true, true
	default:
		return "", false, false
	}
}

func patchAPIServerTLSProfile(ctx context.Context, configClient configv1.ConfigV1Interface, profileType string) error {
	subObjectKey := strings.ToLower(profileType)
	profile := map[string]interface{}{"type": profileType}
	// JSON merge patch keeps sibling union members. Null them so a switch from
	// Modern/Old/Custom does not leave stale keys next to the active type.
	for _, key := range []string{"old", "intermediate", "modern", "custom"} {
		profile[key] = nil
	}
	profile[subObjectKey] = map[string]interface{}{}
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"tlsSecurityProfile": profile,
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal APIServer TLS profile patch: %w", err)
	}

	_, err = configClient.APIServers().Patch(ctx, "cluster", types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch APIServer TLS profile to %s: %w", profileType, err)
	}
	fmt.Fprintf(GinkgoWriter, "patched APIServer tlsSecurityProfile.type=%s\n", profileType)
	return nil
}

// RestoreAPIServerIntermediate restores the cluster APIServer to Intermediate profile.
func RestoreAPIServerIntermediate(ctx context.Context, configClient configv1.ConfigV1Interface) error {
	return patchAPIServerTLSProfile(ctx, configClient, APIServerTLSProfileIntermediate)
}

// ---------------------------------------------------------------------------
// Operand ConfigMap TLS assertions (sanity + full-profile rollout)
// ---------------------------------------------------------------------------

type operandTLSConfig struct {
	source  string
	minTLS  string
	ciphers []string
}

type operandTLSReader struct {
	name string
	read func(ctx context.Context, clientset kubernetes.Interface) (string, []string, error)
}

func jsonOperandTLSReader(cmName, dataKey string, tlsPath []string) operandTLSReader {
	return operandTLSReader{
		name: cmName,
		read: func(ctx context.Context, clientset kubernetes.Interface) (string, []string, error) {
			return readJSONOperandTLS(ctx, clientset, cmName, dataKey, tlsPath)
		},
	}
}

func operandTLSReaders() []operandTLSReader {
	return []operandTLSReader{
		jsonOperandTLSReader(SpireServerConfigMapName, SpireServerConfigKey, []string{"server", TLSConfigKey}),
		jsonOperandTLSReader(SpireAgentConfigMapName, SpireAgentConfigKey, []string{"agent", TLSConfigKey}),
		jsonOperandTLSReader(SpireOIDCDiscoveryProviderConfigMapName, SpireOIDCDiscoveryProviderConfigKey, []string{TLSConfigKey}),
		{name: SpireControllerManagerConfigMapName, read: readControllerManagerOperandTLS},
	}
}

func collectOperandTLSConfigs(ctx context.Context, clientset kubernetes.Interface) ([]operandTLSConfig, error) {
	readers := operandTLSReaders()
	configs := make([]operandTLSConfig, 0, len(readers))
	for _, src := range readers {
		minTLS, ciphers, err := src.read(ctx, clientset)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", src.name, err)
		}
		configs = append(configs, operandTLSConfig{source: src.name, minTLS: minTLS, ciphers: ciphers})
	}
	return configs, nil
}

func readJSONOperandTLS(ctx context.Context, clientset kubernetes.Interface, cmName, dataKey string, tlsPath []string) (string, []string, error) {
	cfg, err := configMapAsMap(ctx, clientset, cmName, dataKey)
	if err != nil {
		return "", nil, err
	}

	minPath := append(slices.Clone(tlsPath), "min_tls_version")
	minTLS, found, err := unstructured.NestedString(cfg, minPath...)
	if err != nil {
		return "", nil, err
	}
	if !found {
		return "", nil, fmt.Errorf("missing %s", strings.Join(minPath, "."))
	}

	ciphers, _, err := unstructured.NestedStringSlice(cfg, append(slices.Clone(tlsPath), "cipher_suites")...)
	if err != nil {
		return minTLS, nil, err
	}
	return minTLS, ciphers, nil
}

// configMapAsMap GETs a ConfigMap and unmarshals data[dataKey] into a nested map.
// YAML unmarshal is used because SPIRE/OIDC configs are JSON (valid YAML) and
// controller-manager config is YAML — one parser covers both.
func configMapAsMap(ctx context.Context, clientset kubernetes.Interface, cmName, dataKey string) (map[string]interface{}, error) {
	cm, err := clientset.CoreV1().ConfigMaps(OperatorNamespace).Get(ctx, cmName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap %s: %w", cmName, err)
	}
	raw, ok := cm.Data[dataKey]
	if !ok {
		return nil, fmt.Errorf("ConfigMap %s missing key %s", cmName, dataKey)
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse ConfigMap %s key %s: %w", cmName, dataKey, err)
	}
	if cfg == nil {
		return nil, fmt.Errorf("ConfigMap %s key %s is empty", cmName, dataKey)
	}
	return cfg, nil
}

func readControllerManagerOperandTLS(ctx context.Context, clientset kubernetes.Interface) (string, []string, error) {
	cfg, err := configMapAsMap(ctx, clientset, SpireControllerManagerConfigMapName, SpireControllerManagerConfigKey)
	if err != nil {
		return "", nil, err
	}

	minTLS, found, err := unstructured.NestedString(cfg, TLSConfigYAMLKey, "minTLSVersion")
	if err != nil {
		return "", nil, err
	}
	if !found {
		return "", nil, fmt.Errorf("controller-manager ConfigMap missing tlsConfig.minTLSVersion")
	}

	ciphers, _, err := unstructured.NestedStringSlice(cfg, TLSConfigYAMLKey, "cipherSuites")
	if err != nil {
		return minTLS, nil, err
	}
	return minTLS, ciphers, nil
}

func validateOperandTLSConfigs(configs []operandTLSConfig, minVersion string, requireCiphers bool) error {
	if len(configs) == 0 {
		return fmt.Errorf("no operand TLS configs collected")
	}

	var reference []string
	for i, cfg := range configs {
		if cfg.minTLS != minVersion {
			return fmt.Errorf("%s min TLS version mismatch: got %s want %s", cfg.source, cfg.minTLS, minVersion)
		}
		for _, cipher := range cfg.ciphers {
			if isGoInsecureOperandCipher(cipher) {
				return fmt.Errorf("%s must not inject Go-insecure cipher %q", cfg.source, cipher)
			}
		}
		if !requireCiphers {
			continue
		}
		if len(cfg.ciphers) == 0 {
			return fmt.Errorf("%s cipher_suites must be non-empty", cfg.source)
		}
		if i == 0 {
			reference = cfg.ciphers
			continue
		}
		if !slices.Equal(cfg.ciphers, reference) {
			return fmt.Errorf("%s cipher_suites must match %s", cfg.source, configs[0].source)
		}
	}
	return nil
}

func isGoInsecureOperandCipher(cipher string) bool {
	return goInsecureCipherSuites.Has(cipher)
}

func logOperandTLSConfigs(configs []operandTLSConfig) {
	for _, cfg := range configs {
		if len(cfg.ciphers) == 0 {
			fmt.Fprintf(GinkgoWriter, "TLS config [%s]: min=%s ciphers=(none — TLS 1.3 min or Go negotiates)\n",
				cfg.source, cfg.minTLS)
			continue
		}
		fmt.Fprintf(GinkgoWriter, "TLS config [%s]: min=%s ciphers=%v\n", cfg.source, cfg.minTLS, cfg.ciphers)
	}
}

// AssertTLSProfileCompliance validates cluster APIServer TLS profile and operand ConfigMap injection.
//
// Validation performed:
//   - APIServer tlsSecurityProfile.type matches expected (if expectedAPIServerProfile is non-empty)
//   - All four operand ConfigMaps have correct min_tls_version/minTLSVersion
//   - No Go-classified insecure ciphers are present (flag.InsecureTLSCipherNames; always checked)
//   - When requireCiphers=true, cipher_suites must be non-empty and consistent across operands
func AssertTLSProfileCompliance(ctx context.Context, configClient configv1.ConfigV1Interface,
	clientset kubernetes.Interface, expectedAPIServerProfile string, minVersion string, requireCiphers bool) {
	By(fmt.Sprintf("Asserting TLS profile compliance (APIServer profile=%q, min=%s)", expectedAPIServerProfile, minVersion))

	if expectedAPIServerProfile != "" {
		profileType, err := GetAPIServerTLSProfileType(ctx, configClient)
		Expect(err).NotTo(HaveOccurred(), "failed to read APIServer tlsSecurityProfile.type")
		Expect(profileType).To(Equal(expectedAPIServerProfile),
			"APIServer tlsSecurityProfile.type should match expected profile")
	}

	configs, err := collectOperandTLSConfigs(ctx, clientset)
	Expect(err).NotTo(HaveOccurred(), "failed to read operand TLS ConfigMaps")
	Expect(validateOperandTLSConfigs(configs, minVersion, requireCiphers)).To(Succeed())
	logOperandTLSConfigs(configs)
}

// ---------------------------------------------------------------------------
// Full TLS profile rollout helpers (patch APIServer, wait for operand rollout)
// ---------------------------------------------------------------------------

// AssertAllOperandsPodsReady verifies operator and operand workloads are ready.
func AssertAllOperandsPodsReady(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface) {
	assertAllOperandsPodsReady(ctx, k8sClient, clientset, DefaultTimeout)
}

func assertAllOperandsPodsReady(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface, timeout time.Duration) {
	By("Verifying operator Deployment is Available")
	WaitForDeploymentAvailable(ctx, clientset, OperatorDeploymentName, OperatorNamespace, timeout)

	By("Verifying SPIRE Server StatefulSet is Ready")
	WaitForStatefulSetReady(ctx, clientset, SpireServerStatefulSetName, OperatorNamespace, timeout)

	By("Verifying SPIRE Agent DaemonSet is Available")
	WaitForDaemonSetAvailable(ctx, clientset, SpireAgentDaemonSetName, OperatorNamespace, timeout)

	By("Verifying SPIFFE CSI Driver DaemonSet is Available")
	WaitForDaemonSetAvailable(ctx, clientset, SpiffeCSIDriverDaemonSetName, OperatorNamespace, timeout)

	By("Verifying OIDC Discovery Provider Deployment is Available")
	WaitForDeploymentAvailable(ctx, clientset, SpireOIDCDiscoveryProviderDeploymentName, OperatorNamespace, timeout)

	By("Verifying ZeroTrustWorkloadIdentityManager CR is Ready")
	WaitForZeroTrustWorkloadIdentityManagerConditions(ctx, k8sClient, "cluster", map[string]metav1.ConditionStatus{
		"Ready":             metav1.ConditionTrue,
		"OperandsAvailable": metav1.ConditionTrue,
	}, timeout)
}

func waitForAllSpireAgentPodsReady(ctx context.Context, clientset kubernetes.Interface) {
	By("Waiting for all SPIRE agent pods to be Ready")
	Eventually(func(g Gomega) {
		pods, err := clientset.CoreV1().Pods(OperatorNamespace).List(ctx, metav1.ListOptions{LabelSelector: SpireAgentPodLabel})
		g.Expect(err).NotTo(HaveOccurred(), "failed to list spire-agent pods")
		active := FilterActivePods(pods.Items)
		g.Expect(active).NotTo(BeEmpty(), "at least one spire-agent pod must exist")
		for _, pod := range active {
			g.Expect(IsPodReady(&pod)).To(BeTrue(), "spire-agent pod %s must be Ready", pod.Name)
		}
	}).WithTimeout(TLSProfileRolloutTimeout).WithPolling(DefaultInterval).Should(Succeed(),
		"all spire-agent pods should become ready within %v", TLSProfileRolloutTimeout)
}

func waitForFourConfigMapsTLS(ctx context.Context, clientset kubernetes.Interface, minVersion string, requireCiphers bool) {
	By(fmt.Sprintf("Waiting for four ConfigMaps to contain tls_config with min version %s", minVersion))
	Eventually(func() error {
		configs, err := collectOperandTLSConfigs(ctx, clientset)
		if err != nil {
			return err
		}
		return validateOperandTLSConfigs(configs, minVersion, requireCiphers)
	}).WithTimeout(TLSProfileRolloutTimeout).WithPolling(DefaultInterval).Should(Succeed(),
		"four ConfigMaps should reflect min TLS version %s within %v", minVersion, TLSProfileRolloutTimeout)
}

// WaitForTLSProfileRolloutComplete blocks until operands and ConfigMaps reflect the expected TLS profile.
func WaitForTLSProfileRolloutComplete(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface,
	minVersion string, requireCiphers bool) {
	By(fmt.Sprintf("Waiting for full TLS profile rollout (min version %s)", minVersion))
	assertAllOperandsPodsReady(ctx, k8sClient, clientset, TLSProfileRolloutTimeout)
	waitForFourConfigMapsTLS(ctx, clientset, minVersion, requireCiphers)
	WaitForPodReady(ctx, clientset, SpireServerPodName, OperatorNamespace, TLSProfileRolloutTimeout)
	waitForAllSpireAgentPodsReady(ctx, clientset)
}

func waitForTLSProfileClusterStabilization(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface) {
	By(fmt.Sprintf("Waiting %v for cluster stabilization after TLS profile rollout", TLSProfilePatchStabilizationInterval))
	assertAllOperandsPodsReady(ctx, k8sClient, clientset, ShortTimeout)

	timer := time.NewTimer(TLSProfilePatchStabilizationInterval)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		Expect(ctx.Err()).NotTo(HaveOccurred(), "context canceled during TLS profile stabilization buffer")
	}
}

// ApplyTLSProfileAndWaitForRollout patches the APIServer profile (unless profileType is empty),
// waits for the full operand rollout, then buffers before the next profile change.
func ApplyTLSProfileAndWaitForRollout(ctx context.Context, configClient configv1.ConfigV1Interface,
	k8sClient client.Client, clientset kubernetes.Interface, profileType string,
	minVersion string, requireCiphers bool) {
	if profileType != "" {
		By(fmt.Sprintf("Applying APIServer TLS profile %s", profileType))
		Expect(patchAPIServerTLSProfile(ctx, configClient, profileType)).To(Succeed())
	} else {
		By("Skipping APIServer TLS profile patch (PROFILE-DEFAULT / current baseline)")
	}
	WaitForTLSProfileRolloutComplete(ctx, k8sClient, clientset, minVersion, requireCiphers)
	waitForTLSProfileClusterStabilization(ctx, k8sClient, clientset)
}

// ---------------------------------------------------------------------------
// Wire-level TLS checks via in-cluster openssl probe
// ---------------------------------------------------------------------------

type tlsWireDoor struct {
	name        string
	resolveAddr func(ctx context.Context, clientset kubernetes.Interface) (string, error)
	openSSLArgs []string
}

var (
	tlsProtocolLineRE = regexp.MustCompile(`(?i)Protocol\s*:\s*(TLS[v]?[\d.]+)`)
	tlsNewCipherRE    = regexp.MustCompile(`(?i)New,\s*(TLS[\w.]+)`)
)

func defaultTLSWireDoors() []tlsWireDoor {
	return []tlsWireDoor{
		{
			name: "operator-metrics",
			resolveAddr: func(ctx context.Context, clientset kubernetes.Interface) (string, error) {
				return resolveServiceHostPort(ctx, clientset, OperatorMetricsServiceName, OperatorMetricsPort)
			},
		},
		{
			name: "spire-server-grpc",
			resolveAddr: func(ctx context.Context, clientset kubernetes.Interface) (string, error) {
				return resolvePodHostPort(ctx, clientset, SpireServerPodName, SpireServerGRPCPort)
			},
			openSSLArgs: []string{"-alpn", "h2"},
		},
		{
			name: "oidc-discovery",
			resolveAddr: func(ctx context.Context, clientset kubernetes.Interface) (string, error) {
				return resolveServiceHostPort(ctx, clientset, SpireOIDCDiscoveryProviderServiceName, SpireOIDCDiscoveryProviderServicePort)
			},
		},
		{
			name: "controller-manager-webhook",
			resolveAddr: func(ctx context.Context, clientset kubernetes.Interface) (string, error) {
				return resolvePodHostPort(ctx, clientset, SpireServerPodName, SpireControllerManagerWebhookPort)
			},
		},
	}
}

func opensslProbeImage() string {
	if img := strings.TrimSpace(os.Getenv("TLSPROBE_OPENSSL_IMAGE")); img != "" {
		return img
	}
	return TLSOpenSSLProbeImage
}

func restrictedContainerSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		RunAsNonRoot:             ptr.To(true),
		SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
}

// EnsureOpenSSLProbePod creates a short-lived UBI pod with openssl and waits until Ready.
// Registers DeferCleanup to delete the pod. RunAsUser is omitted so restricted-v2 SCC
// can assign a UID from the operator namespace uid-range.
func EnsureOpenSSLProbePod(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface) {
	By(fmt.Sprintf("Ensuring openssl probe pod %s/%s", OperatorNamespace, TLSOpenSSLProbePodName))

	_ = clientset.CoreV1().Pods(OperatorNamespace).Delete(ctx, TLSOpenSSLProbePodName, metav1.DeleteOptions{})
	Eventually(func() bool {
		_, err := clientset.CoreV1().Pods(OperatorNamespace).Get(ctx, TLSOpenSSLProbePodName, metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(ShortTimeout).WithPolling(ShortInterval).Should(BeTrue(),
		"previous openssl probe pod should be deleted")

	image := opensslProbeImage()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TLSOpenSSLProbePodName,
			Namespace: OperatorNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "ztwim-tls-openssl-probe",
				"app.kubernetes.io/managed-by": "ztwim-e2e",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:            TLSOpenSSLProbeContainer,
				Image:           image,
				Command:         []string{"sleep", "3600"},
				SecurityContext: restrictedContainerSecurityContext(),
			}},
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   ptr.To(true),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
		},
	}

	Expect(k8sClient.Create(ctx, pod)).To(Succeed(), "failed to create openssl probe pod with image %s", image)
	DeferCleanup(func(cleanupCtx context.Context) {
		_ = k8sClient.Delete(cleanupCtx, pod)
	})

	WaitForPodReady(ctx, clientset, TLSOpenSSLProbePodName, OperatorNamespace, DefaultTimeout)

	stdout, stderr, err := ExecInPod(ctx, OperatorNamespace, TLSOpenSSLProbePodName, TLSOpenSSLProbeContainer,
		[]string{"openssl", "version"})
	Expect(err).NotTo(HaveOccurred(), "openssl must be available in probe pod (stderr=%s)", strings.TrimSpace(stderr))
	fmt.Fprintf(GinkgoWriter, "openssl probe ready: %s\n", strings.TrimSpace(stdout))
}

// AssertTLSWireCompliance dials each default TLS door via in-cluster openssl and asserts
// the negotiated protocol meets expectedMinVersion (VersionTLS12 / VersionTLS13).
func AssertTLSWireCompliance(ctx context.Context, clientset kubernetes.Interface, expectedMinVersion string) {
	By(fmt.Sprintf("Asserting wire-level TLS via openssl (min=%s)", expectedMinVersion))
	for _, door := range defaultTLSWireDoors() {
		By(fmt.Sprintf("openssl probe door %s", door.name))
		addr, err := door.resolveAddr(ctx, clientset)
		Expect(err).NotTo(HaveOccurred(), "failed to resolve address for door %s", door.name)
		Expect(addr).NotTo(BeEmpty(), "door %s resolved empty address", door.name)

		protocol := ""
		Eventually(func(g Gomega) {
			var probeErr error
			protocol, probeErr = opensslProbeProtocol(ctx, addr, door.openSSLArgs...)
			g.Expect(probeErr).NotTo(HaveOccurred(), "openssl probe failed for door %s", door.name)
			g.Expect(protocol).NotTo(BeEmpty(), "openssl did not report Protocol for door %s", door.name)
		}).WithTimeout(TLSWireProbeTimeout).WithPolling(ShortInterval).Should(Succeed(),
			"door %s should complete TLS handshake within %v", door.name, TLSWireProbeTimeout)

		fmt.Fprintf(GinkgoWriter, "door %s negotiated %s\n", door.name, protocol)
		Expect(tlsProtocolMeetsMinimum(protocol, expectedMinVersion)).To(BeTrue(),
			"door %s negotiated %s, want >= %s", door.name, protocol, expectedMinVersion)
	}
}

// AssertClusterSPIFFEIDWorksUnderCurrentTLS verifies ClusterSPIFFEID apply/delete under the
// active TLS profile (FUNC-005): SVID issuance succeeds, then explicit delete succeeds.
func AssertClusterSPIFFEIDWorksUnderCurrentTLS(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface) {
	By("FUNC-005: ClusterSPIFFEID apply under current TLS profile")
	f := SetupAttestationTest(ctx, k8sClient, clientset, "tls-func005", nil)
	Expect(f.ClusterSPIFFEIDName).NotTo(BeEmpty())

	By("FUNC-005: deleting ClusterSPIFFEID explicitly")
	cspiffeID := &spiffev1alpha1.ClusterSPIFFEID{ObjectMeta: metav1.ObjectMeta{Name: f.ClusterSPIFFEIDName}}
	Expect(k8sClient.Delete(ctx, cspiffeID)).To(Succeed(),
		"ClusterSPIFFEID delete must succeed under current TLS profile (webhook path)")
	fmt.Fprintf(GinkgoWriter, "FUNC-005: ClusterSPIFFEID %q apply+delete succeeded in ns %s\n",
		f.ClusterSPIFFEIDName, f.Namespace)
}

func opensslProbeProtocol(ctx context.Context, hostPort string, extraArgs ...string) (string, error) {
	host, _, _ := strings.Cut(hostPort, ":")
	args := append([]string{"s_client", "-connect", hostPort, "-servername", host}, extraArgs...)

	quoted := make([]string, 0, len(args))
	for _, a := range args {
		quoted = append(quoted, shellQuote(a))
	}
	cmd := []string{"sh", "-c", "echo | openssl " + strings.Join(quoted, " ") + " 2>&1 || true"}

	stdout, stderr, err := ExecInPod(ctx, OperatorNamespace, TLSOpenSSLProbePodName, TLSOpenSSLProbeContainer, cmd)
	out := stdout + "\n" + stderr
	if err != nil && !strings.Contains(out, "Protocol") && !strings.Contains(out, "New, TLSv") {
		fmt.Fprintf(GinkgoWriter, "openssl s_client exec failed: %s\n", opensslDiagnostic(out))
		return "", fmt.Errorf("openssl s_client exec failed")
	}

	if m := tlsProtocolLineRE.FindStringSubmatch(out); len(m) == 2 {
		return normalizeTLSProtocol(m[1]), nil
	}
	if m := tlsNewCipherRE.FindStringSubmatch(out); len(m) == 2 {
		return normalizeTLSProtocol(m[1]), nil
	}
	fmt.Fprintf(GinkgoWriter, "openssl s_client protocol parse failed: %s\n", opensslDiagnostic(out))
	return "", fmt.Errorf("openssl s_client protocol parse failed")
}

// opensslDiagnostic keeps protocol/verify/alert lines only. No PEM, session, or addresses.
func opensslDiagnostic(out string) string {
	var kept []string
	for _, line := range strings.Split(out, "\n") {
		l := strings.TrimSpace(line)
		if l == "" || !opensslDiagnosticLine(l) {
			continue
		}
		kept = append(kept, l)
		if len(kept) >= 6 {
			break
		}
	}
	if len(kept) == 0 {
		return "no protocol or verify status in openssl output"
	}
	return strings.Join(kept, "; ")
}

func opensslDiagnosticLine(l string) bool {
	lower := strings.ToLower(l)
	switch {
	case strings.Contains(lower, "protocol") && strings.Contains(lower, ":"):
		return true
	case strings.Contains(lower, "verify return code"):
		return true
	case strings.Contains(lower, "verify error"):
		return true
	case strings.HasPrefix(lower, "new, tls"):
		return true
	case strings.Contains(lower, "handshake failure"):
		return true
	case strings.Contains(lower, "no peer certificate"):
		return true
	case strings.Contains(lower, "alert") && (strings.Contains(lower, "ssl") || strings.Contains(lower, "tls")):
		return true
	default:
		return false
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func normalizeTLSProtocol(p string) string {
	p = strings.TrimSpace(p)
	upper := strings.ToUpper(p)
	rest := strings.TrimPrefix(upper, "TLSV")
	if rest == upper {
		rest = strings.TrimPrefix(upper, "TLS")
	}
	return "TLSv" + rest
}

func tlsProtocolMeetsMinimum(actualProtocol, expectedMinK8s string) bool {
	minProtocol := map[string]string{
		MinTLSVersionK8sTLS12: "TLSv1.2",
		MinTLSVersionK8sTLS13: "TLSv1.3",
	}[expectedMinK8s]
	if minProtocol == "" {
		return false
	}
	actual := tlsProtocolRank(actualProtocol)
	return actual > 0 && actual >= tlsProtocolRank(minProtocol)
}

func tlsProtocolRank(protocol string) int {
	p := strings.ToLower(strings.TrimSpace(protocol))
	p = strings.TrimPrefix(p, "tlsv")
	p = strings.TrimPrefix(p, "tls")
	parts := strings.SplitN(p, ".", 2)
	if parts[0] == "" {
		return 0
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0
	}
	minor := 0
	if len(parts) == 2 {
		minor, _ = strconv.Atoi(parts[1])
	}
	return major*10 + minor
}

func resolveServiceHostPort(ctx context.Context, clientset kubernetes.Interface, serviceName string, port int) (string, error) {
	svc, err := clientset.CoreV1().Services(OperatorNamespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == "None" {
		return "", fmt.Errorf("service %s has no ClusterIP", serviceName)
	}
	return fmt.Sprintf("%s:%d", svc.Spec.ClusterIP, port), nil
}

func resolvePodHostPort(ctx context.Context, clientset kubernetes.Interface, podName string, port int) (string, error) {
	pod, err := clientset.CoreV1().Pods(OperatorNamespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if pod.Status.PodIP == "" {
		return "", fmt.Errorf("pod %s has no PodIP", podName)
	}
	return fmt.Sprintf("%s:%d", pod.Status.PodIP, port), nil
}
