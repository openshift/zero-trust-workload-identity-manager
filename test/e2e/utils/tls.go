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
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ocpconfigv1 "github.com/openshift/api/config/v1"
	configv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	openshifttls "github.com/openshift/controller-runtime-common/pkg/tls"
	pkgtls "github.com/openshift/zero-trust-workload-identity-manager/pkg/tls"
	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/component-base/cli/flag"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// goInsecureCipherSuites is the set of cipher suite names classified as insecure by crypto/tls
// (via k8s.io/component-base/cli/flag). Operand configs use IANA names that match this set.
var goInsecureCipherSuites = sets.NewString(flag.InsecureTLSCipherNames()...)

var errNoMachineConfigPools = errors.New("no MachineConfigPools found")

const (
	APIServerTLSProfileIntermediate = "Intermediate"
	APIServerTLSProfileModern       = "Modern"
	APIServerTLSProfileOld          = "Old"
	APIServerTLSProfileCustom       = "Custom"

	TLSConfigKey     = "tls_config" // JSON: SPIRE server/agent/OIDC
	TLSConfigYAMLKey = "tlsConfig"  // YAML: controller-manager

	MinTLSVersionK8sTLS12 = "VersionTLS12"
	MinTLSVersionK8sTLS13 = "VersionTLS13"

	SpireControllerManagerConfigMapName = "spire-controller-manager"
	SpireControllerManagerConfigKey     = "controller-manager-config.yaml"
	SpireServerPodName                  = "spire-server-0"

	TLSProfileRolloutTimeout = 15 * time.Minute
	// MachineConfigReadyTimeout is how long to poll MachineConfigPools and nodes
	// before a TLS profile patch. A 6-node CI roll finished in about 30 minutes;
	// 45 minutes leaves slack. The poll returns as soon as the cluster is ready.
	MachineConfigReadyTimeout = 45 * time.Minute
	// TLSProfileApplyTimeout is the context budget for one profile apply:
	// machine poll, operand rollout, machine poll, then tls-scanner.
	TLSProfileApplyTimeout = 2*MachineConfigReadyTimeout + TLSProfileRolloutTimeout + DefaultTimeout
	// TLSProfileRestoreTimeout is the context budget to restore the APIServer
	// TLS profile, wait for operands, and wait for Machine Config to finish.
	TLSProfileRestoreTimeout = MachineConfigReadyTimeout + TLSProfileRolloutTimeout
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

// ExpectedOperandTLSForLiveAPIServer resolves operand TLS expectations from the
// current APIServer cluster config using the same GetTLSProfileSpec path as the operator.
func ExpectedOperandTLSForLiveAPIServer(ctx context.Context, configClient configv1.ConfigV1Interface) (minVersion string, requireCiphers bool, ok bool, err error) {
	apiServer, err := configClient.APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return "", false, false, fmt.Errorf("failed to get APIServer cluster config: %w", err)
	}
	profileSpec, err := openshifttls.GetTLSProfileSpec(apiServer.Spec.TLSSecurityProfile)
	if err != nil {
		return "", false, false, fmt.Errorf("failed to resolve APIServer TLS profile spec: %w", err)
	}
	tlsCfg := pkgtls.GetTLSConfigForSpiffeSpire(&profileSpec)
	if tlsCfg == nil {
		return "", false, false, nil
	}
	mv := fmt.Sprint(tlsCfg["min_tls_version"])
	ciphers, _ := tlsCfg["cipher_suites"].([]string)
	return mv, len(ciphers) > 0, true, nil
}

// SavedProfileType returns the profile type string for a saved *TLSSecurityProfile.
// Returns "" (default/Intermediate-equivalent) when saved is nil.
func SavedProfileType(saved *ocpconfigv1.TLSSecurityProfile) string {
	if saved == nil {
		return ""
	}
	return string(saved.Type)
}

// ExpectedOperandTLSForAPIServerProfile derives operand expectations from the same
// pkg/tls injection the operator uses. It resolves a profile type string to the
// expected minVersion and whether ciphers must be present.
func ExpectedOperandTLSForAPIServerProfile(profileType string) (minVersion string, requireCiphers bool, ok bool) {
	var spec *ocpconfigv1.TLSProfileSpec
	switch profileType {
	case "", APIServerTLSProfileIntermediate:
		s := ocpconfigv1.TLSProfiles[ocpconfigv1.TLSProfileIntermediateType]
		spec = s
	case APIServerTLSProfileModern:
		s := ocpconfigv1.TLSProfiles[ocpconfigv1.TLSProfileModernType]
		spec = s
	case APIServerTLSProfileOld:
		s := ocpconfigv1.TLSProfiles[ocpconfigv1.TLSProfileOldType]
		spec = s
	case APIServerTLSProfileCustom:
		// Well-known Custom spec used by the e2e TLS profile suite.
		// Must match E2ECustomTLSProfileSpec().
		spec = E2ECustomTLSProfileSpec()
	default:
		return "", false, false
	}

	tlsCfg := pkgtls.GetTLSConfigForSpiffeSpire(spec)
	if tlsCfg == nil {
		return "", false, false
	}

	mv := fmt.Sprint(tlsCfg["min_tls_version"])
	ciphers, _ := tlsCfg["cipher_suites"].([]string)
	return mv, len(ciphers) > 0, true
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

// RestoreAPIServerTLSSecurityProfile restores the cluster APIServer to the saved TLS profile.
// If saved is nil the patch clears tlsSecurityProfile entirely (cluster default).
func RestoreAPIServerTLSSecurityProfile(ctx context.Context, configClient configv1.ConfigV1Interface, saved *ocpconfigv1.TLSSecurityProfile) error {
	// Null all union members first so stale keys do not remain.
	profile := map[string]interface{}{
		"old":          nil,
		"intermediate": nil,
		"modern":       nil,
		"custom":       nil,
	}

	var patchSpec map[string]interface{}
	if saved == nil {
		// No profile was set originally — clear it.
		patchSpec = map[string]interface{}{
			"tlsSecurityProfile": nil,
		}
	} else {
		profile["type"] = string(saved.Type)
		subKey := strings.ToLower(string(saved.Type))
		if saved.Type == ocpconfigv1.TLSProfileCustomType && saved.Custom != nil {
			customPatch := map[string]interface{}{}
			if saved.Custom.MinTLSVersion != "" {
				customPatch["minTLSVersion"] = string(saved.Custom.MinTLSVersion)
			}
			if len(saved.Custom.Ciphers) > 0 {
				customPatch["ciphers"] = append([]string(nil), saved.Custom.Ciphers...)
			}
			profile["custom"] = customPatch
		} else {
			profile[subKey] = map[string]interface{}{}
		}
		patchSpec = map[string]interface{}{
			"tlsSecurityProfile": profile,
		}
	}

	patch := map[string]interface{}{"spec": patchSpec}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal APIServer TLS restore patch: %w", err)
	}

	_, err = configClient.APIServers().Patch(ctx, "cluster", types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to restore APIServer TLS profile: %w", err)
	}
	if saved == nil {
		fmt.Fprintf(GinkgoWriter, "restored APIServer tlsSecurityProfile to nil (cluster default)\n")
	} else {
		fmt.Fprintf(GinkgoWriter, "restored APIServer tlsSecurityProfile.type=%s\n", saved.Type)
	}
	return nil
}

// E2ECustomTLSProfileSpec returns the well-known Custom TLS profile used by the
// e2e TLS profile suite. It uses TLS 1.2 with a small set of GCM ciphers so
// the operator can map it via pkg/tls.GetTLSConfigForSpiffeSpire.
func E2ECustomTLSProfileSpec() *ocpconfigv1.TLSProfileSpec {
	return &ocpconfigv1.TLSProfileSpec{
		MinTLSVersion: ocpconfigv1.VersionTLS12,
		Ciphers: []string{
			"ECDHE-RSA-AES128-GCM-SHA256",
			"ECDHE-ECDSA-AES128-GCM-SHA256",
		},
	}
}

// PatchAPIServerCustomTLSProfile patches APIServer/cluster to use a Custom TLS
// profile with the e2e well-known spec (E2ECustomTLSProfileSpec).
func PatchAPIServerCustomTLSProfile(ctx context.Context, configClient configv1.ConfigV1Interface) error {
	spec := E2ECustomTLSProfileSpec()
	ciphers := make([]interface{}, len(spec.Ciphers))
	for i, c := range spec.Ciphers {
		ciphers[i] = c
	}

	profile := map[string]interface{}{
		"type":         "Custom",
		"old":          nil,
		"intermediate": nil,
		"modern":       nil,
		"custom": map[string]interface{}{
			"minTLSVersion": string(spec.MinTLSVersion),
			"ciphers":       ciphers,
		},
	}
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"tlsSecurityProfile": profile,
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal APIServer Custom TLS profile patch: %w", err)
	}

	_, err = configClient.APIServers().Patch(ctx, "cluster", types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch APIServer TLS profile to Custom: %w", err)
	}
	fmt.Fprintf(GinkgoWriter, "patched APIServer tlsSecurityProfile.type=Custom (minTLS=%s, ciphers=%v)\n",
		spec.MinTLSVersion, spec.Ciphers)
	return nil
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

// WaitForClusterMachinesReady polls until every MachineConfigPool is Updated and
// every node is Ready and not cordoned. It returns as soon as that is true.
// If MachineConfigReadyTimeout elapses, the spec fails with pool and node status.
func WaitForClusterMachinesReady(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface) {
	By(fmt.Sprintf("Waiting up to %v for MachineConfigPools and nodes to be ready", MachineConfigReadyTimeout))
	Eventually(func() error {
		err := clusterMachinesNotReady(ctx, k8sClient, clientset)
		if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) || errors.Is(err, errNoMachineConfigPools) {
			Expect(err).NotTo(HaveOccurred(), "MachineConfigPool API is required for TLS profile tests")
		}
		return err
	}).WithContext(ctx).WithTimeout(MachineConfigReadyTimeout).WithPolling(DefaultInterval).Should(Succeed(),
		"MachineConfigPools and nodes were not ready within %v; refusing TLS profile change until they are ready",
		MachineConfigReadyTimeout)
	fmt.Fprintf(GinkgoWriter, "MachineConfigPools and nodes are ready\n")
}

func clusterMachinesNotReady(ctx context.Context, k8sClient client.Client, clientset kubernetes.Interface) error {
	pools := &unstructured.UnstructuredList{}
	pools.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "machineconfiguration.openshift.io",
		Version: "v1",
		Kind:    "MachineConfigPoolList",
	})
	if err := k8sClient.List(ctx, pools); err != nil {
		return fmt.Errorf("failed to list MachineConfigPools: %w", err)
	}
	if len(pools.Items) == 0 {
		return errNoMachineConfigPools
	}

	var problems []string
	for _, pool := range pools.Items {
		if msg := machineConfigPoolNotReady(pool); msg != "" {
			problems = append(problems, msg)
		}
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}
	if len(nodes.Items) == 0 {
		problems = append(problems, "no nodes found")
	}
	for _, node := range nodes.Items {
		if msg := nodeNotReadyForTLS(node); msg != "" {
			problems = append(problems, msg)
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return errors.New(strings.Join(problems, "; "))
}

func machineConfigPoolNotReady(pool unstructured.Unstructured) string {
	updated, hasUpdated := mcpConditionStatus(pool, "Updated")
	updating, hasUpdating := mcpConditionStatus(pool, "Updating")
	degraded, hasDegraded := mcpConditionStatus(pool, "Degraded")
	machineCount, hasMachineCount := nestedInt64(pool.Object, "status", "machineCount")
	readyCount, hasReadyCount := nestedInt64(pool.Object, "status", "readyMachineCount")
	updatedCount, hasUpdatedCount := nestedInt64(pool.Object, "status", "updatedMachineCount")

	countsReady := hasMachineCount && machineCount > 0 &&
		hasReadyCount && readyCount == machineCount &&
		hasUpdatedCount && updatedCount == machineCount
	if hasUpdated && updated == string(metav1.ConditionTrue) &&
		hasUpdating && updating == string(metav1.ConditionFalse) &&
		hasDegraded && degraded == string(metav1.ConditionFalse) &&
		countsReady {
		return ""
	}

	return fmt.Sprintf("MachineConfigPool %s not ready: Updated=%s Updating=%s Degraded=%s ready=%d/%d updatedMachines=%d",
		pool.GetName(),
		conditionOrMissing(updated, hasUpdated),
		conditionOrMissing(updating, hasUpdating),
		conditionOrMissing(degraded, hasDegraded),
		readyCount, machineCount, updatedCount)
}

func mcpConditionStatus(pool unstructured.Unstructured, conditionType string) (string, bool) {
	conditions, found, err := unstructured.NestedSlice(pool.Object, "status", "conditions")
	if err != nil || !found {
		return "", false
	}
	for _, condition := range conditions {
		fields, ok := condition.(map[string]interface{})
		if !ok {
			continue
		}
		gotType, _, _ := unstructured.NestedString(fields, "type")
		if gotType != conditionType {
			continue
		}
		status, statusFound, _ := unstructured.NestedString(fields, "status")
		return status, statusFound
	}
	return "", false
}

func nestedInt64(obj map[string]interface{}, fields ...string) (int64, bool) {
	value, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if err != nil || !found || value == nil {
		return 0, false
	}
	switch n := value.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	case json.Number:
		parsed, parseErr := n.Int64()
		if parseErr != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func conditionOrMissing(status string, found bool) string {
	if !found || status == "" {
		return "missing"
	}
	return status
}

func nodeNotReadyForTLS(node corev1.Node) string {
	ready := false
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			ready = true
			break
		}
	}
	if ready && !node.Spec.Unschedulable {
		return ""
	}
	state := "NotReady"
	if ready {
		state = "Ready"
	}
	if node.Spec.Unschedulable {
		return fmt.Sprintf("node %s %s cordoned", node.Name, state)
	}
	return fmt.Sprintf("node %s %s", node.Name, state)
}

// ApplyTLSProfileAndWaitForRollout waits until Machine Config is idle, patches the
// APIServer profile (unless profileType is empty), waits for operands, then waits
// again until pools and nodes are ready.
func ApplyTLSProfileAndWaitForRollout(ctx context.Context, configClient configv1.ConfigV1Interface,
	k8sClient client.Client, clientset kubernetes.Interface, profileType string,
	minVersion string, requireCiphers bool) {
	WaitForClusterMachinesReady(ctx, k8sClient, clientset)
	if profileType == APIServerTLSProfileCustom {
		By("Applying APIServer TLS profile Custom")
		Expect(PatchAPIServerCustomTLSProfile(ctx, configClient)).To(Succeed())
	} else if profileType != "" {
		By(fmt.Sprintf("Applying APIServer TLS profile %s", profileType))
		Expect(patchAPIServerTLSProfile(ctx, configClient, profileType)).To(Succeed())
	} else {
		By("Skipping APIServer TLS profile patch (PROFILE-DEFAULT / current baseline)")
	}
	WaitForTLSProfileRolloutComplete(ctx, k8sClient, clientset, minVersion, requireCiphers)
	WaitForClusterMachinesReady(ctx, k8sClient, clientset)
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
