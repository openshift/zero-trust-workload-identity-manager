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
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	operatorv1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	serverEgressPolicyName = "ztwim-sys-spire-server-egress-allow-all"
	agentEgressPolicyName  = "ztwim-sys-spire-agent-egress-allow-all"
	oidcEgressPolicyName   = "ztwim-sys-spire-oidc-discovery-provider-egress-allow-all"
	federationPolicyName   = "ztwim-sys-spire-server-federation-ingress"

	networkPolicyProbeLabel = "e2e.openshift.io/networkpolicy-probe"
	probeContainerName      = "workload"
	// unmanagedDeleteWindow is long enough for another reconcile to recreate a
	// policy. While management is off, the object must stay absent for this long.
	unmanagedDeleteWindow = 45 * time.Second
	policyStabilityWindow = 10 * time.Second
)

// ingressNetworkPolicies are the policies created while managedNetworkPolicy is enabled.
var ingressNetworkPolicies = []struct {
	Name        string
	PodSelector string
}{
	{Name: "ztwim-sys-spire-server-ingress", PodSelector: "spire-server"},
	{Name: "ztwim-sys-spire-agent-ingress", PodSelector: "spire-agent"},
	{Name: "ztwim-sys-spire-oidc-discovery-provider-ingress", PodSelector: "spiffe-oidc-discovery-provider"},
}

// egressAllowAllPolicies are the policies created for server and agent egress.
var egressAllowAllPolicies = []struct {
	Name        string
	PodSelector string
}{
	{Name: serverEgressPolicyName, PodSelector: "spire-server"},
	{Name: agentEgressPolicyName, PodSelector: "spire-agent"},
}

// ManagedNetworkPolicyCase is one operand covered by the managedNetworkPolicy specs.
type ManagedNetworkPolicyCase struct {
	Kind         string
	PodSelector  string
	WorkloadName string
	WorkloadKind string
}

// ManagedNetworkPolicyCases are the operands that carry spec.managedNetworkPolicy.
var ManagedNetworkPolicyCases = []ManagedNetworkPolicyCase{
	{
		Kind:         "SpireServer",
		PodSelector:  SpireServerPodLabel,
		WorkloadName: SpireServerStatefulSetName,
		WorkloadKind: "StatefulSet",
	},
	{
		Kind:         "SpireAgent",
		PodSelector:  SpireAgentPodLabel,
		WorkloadName: SpireAgentDaemonSetName,
		WorkloadKind: "DaemonSet",
	},
	{
		Kind:         "SpireOIDCDiscoveryProvider",
		PodSelector:  SpireOIDCDiscoveryProviderPodLabel,
		WorkloadName: SpireOIDCDiscoveryProviderDeploymentName,
		WorkloadKind: "Deployment",
	},
}

// WorkloadPodIdentity is the stable identity of one running pod.
type WorkloadPodIdentity struct {
	Name     string
	UID      types.UID
	Restarts []int32
}

// DryRunMergePatch sends a merge patch with server-side dry-run so admission
// rejects it without writing the object.
func DryRunMergePatch(ctx context.Context, k8sClient client.Client, obj client.Object, patch []byte) error {
	return k8sClient.Patch(ctx, obj, client.RawPatch(types.MergePatchType, patch), client.DryRunAll)
}

// WorkloadPodIdentities lists running pods for selector. Restart counts come
// from containerStatuses. Terminating pods are dropped by FilterActivePods.
func WorkloadPodIdentities(
	ctx context.Context,
	clientset kubernetes.Interface,
	namespace,
	selector string,
) ([]WorkloadPodIdentity, error) {
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}

	active := FilterActivePods(pods.Items)
	identities := make([]WorkloadPodIdentity, 0, len(active))
	for _, pod := range active {
		restarts := make([]int32, 0, len(pod.Status.ContainerStatuses))
		for _, status := range pod.Status.ContainerStatuses {
			restarts = append(restarts, status.RestartCount)
		}
		identities = append(identities, WorkloadPodIdentity{
			Name:     pod.Name,
			UID:      pod.UID,
			Restarts: restarts,
		})
	}
	sort.Slice(identities, func(i, j int) bool { return identities[i].Name < identities[j].Name })
	return identities, nil
}

// WorkloadPodTemplateSpec returns spec.template.spec for a StatefulSet, DaemonSet, or Deployment.
func WorkloadPodTemplateSpec(
	ctx context.Context,
	clientset kubernetes.Interface,
	namespace,
	name,
	kind string,
) (corev1.PodSpec, error) {
	switch kind {
	case "StatefulSet":
		sts, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return corev1.PodSpec{}, err
		}
		return sts.Spec.Template.Spec, nil
	case "DaemonSet":
		ds, err := clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return corev1.PodSpec{}, err
		}
		return ds.Spec.Template.Spec, nil
	case "Deployment":
		deploy, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return corev1.PodSpec{}, err
		}
		return deploy.Spec.Template.Spec, nil
	default:
		return corev1.PodSpec{}, fmt.Errorf("unsupported workload kind %q", kind)
	}
}

// AssertManagedNetworkPolicyDefault checks the default "true" value, the ingress policies, Ready, and Upgradeable.
func AssertManagedNetworkPolicyDefault(
	ctx context.Context,
	k8sClient client.Client,
	clientset kubernetes.Interface,
	operatorConditionName string,
	tc ManagedNetworkPolicyCase,
) {
	By("Reading managedNetworkPolicy")
	_, value := getManagedNetworkPolicy(ctx, k8sClient, tc)
	Expect(value).To(Equal("true"), "%s managedNetworkPolicy should default to true", tc.Kind)

	assertManagedNetworkPolicyEnabled(ctx, k8sClient, clientset, operatorConditionName, tc)
}

// AssertManagedNetworkPolicyRejectsInvalid dry-runs rejected values and checks the object is unchanged.
func AssertManagedNetworkPolicyRejectsInvalid(
	ctx context.Context,
	k8sClient client.Client,
	tc ManagedNetworkPolicyCase,
) {
	before, value := getManagedNetworkPolicy(ctx, k8sClient, tc)
	generation := before.GetGeneration()

	rejected := []struct {
		name  string
		patch string
	}{
		{name: "invalid", patch: `{"spec":{"managedNetworkPolicy":"invalid"}}`},
		{name: "empty", patch: `{"spec":{"managedNetworkPolicy":""}}`},
		{name: "boolean false", patch: `{"spec":{"managedNetworkPolicy":false}}`},
	}
	for _, attempt := range rejected {
		By("Submitting " + attempt.name)
		obj := newManagedNetworkPolicyObject(tc.Kind)
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, obj)).To(Succeed())
		err := DryRunMergePatch(ctx, k8sClient, obj, []byte(attempt.patch))
		Expect(err).To(HaveOccurred(), "%s should reject managedNetworkPolicy %s", tc.Kind, attempt.name)
		Expect(err.Error()).To(ContainSubstring("managedNetworkPolicy"))
	}

	By("Verifying the stored configuration is unchanged")
	after, stored := getManagedNetworkPolicy(ctx, k8sClient, tc)
	Expect(stored).To(Equal(value))
	Expect(after.GetGeneration()).To(Equal(generation))
}

// AssertManagedNetworkPolicyToggleStable toggles the field without changing the pod template or pod identities.
// While the field is "false", the operand stays Ready and keeps running.
func AssertManagedNetworkPolicyToggleStable(
	ctx context.Context,
	k8sClient client.Client,
	clientset kubernetes.Interface,
	operatorConditionName string,
	tc ManagedNetworkPolicyCase,
) {
	restoreManagedNetworkPolicy(k8sClient, tc)

	By("Capturing the workload pod template and pod identities")
	template := managedNetworkPolicyPodTemplate(ctx, clientset, tc)
	identities, err := WorkloadPodIdentities(ctx, clientset, OperatorNamespace, tc.PodSelector)
	Expect(err).NotTo(HaveOccurred(), "failed to list %s pods", tc.Kind)
	Expect(identities).NotTo(BeEmpty(), "%s should have running pods", tc.Kind)

	for _, value := range []string{"true", "false", "true"} {
		By("Setting managedNetworkPolicy to " + value)
		setManagedNetworkPolicy(ctx, k8sClient, tc, value)

		if value == "false" {
			assertManagedNetworkPolicyDisabled(ctx, k8sClient, clientset, operatorConditionName, tc)
		} else {
			assertManagedNetworkPolicyEnabled(ctx, k8sClient, clientset, operatorConditionName, tc)
		}
		tc.waitWorkload(ctx, clientset)

		By("Comparing the workload pod template and pod identities")
		Expect(managedNetworkPolicyPodTemplate(ctx, clientset, tc)).To(
			Equal(template),
			"%s pod template changed after managedNetworkPolicy=%s",
			tc.Kind,
			value,
		)
		current, err := WorkloadPodIdentities(ctx, clientset, OperatorNamespace, tc.PodSelector)
		Expect(err).NotTo(HaveOccurred(), "failed to list %s pods", tc.Kind)
		Expect(current).To(Equal(identities), "%s pod identities changed after managedNetworkPolicy=%s", tc.Kind, value)
	}
}

func assertManagedNetworkPolicyEnabled(
	ctx context.Context,
	k8sClient client.Client,
	clientset kubernetes.Interface,
	operatorConditionName string,
	tc ManagedNetworkPolicyCase,
) {
	By("Checking managed NetworkPolicies and stable operand status")
	assertIngressNetworkPolicies(ctx, clientset)
	assertEgressAllowAllPresent(ctx, clientset)
	assertStableNetworkPolicyStatus(ctx, k8sClient, operatorConditionName, tc, true)
}

func assertManagedNetworkPolicyDisabled(
	ctx context.Context,
	k8sClient client.Client,
	clientset kubernetes.Interface,
	operatorConditionName string,
	tc ManagedNetworkPolicyCase,
) {
	By("Checking existing NetworkPolicies remain while management is disabled")
	assertIngressNetworkPolicies(ctx, clientset)
	assertEgressAllowAllPresent(ctx, clientset)
	assertStableNetworkPolicyStatus(ctx, k8sClient, operatorConditionName, tc, false)
}

// All operand fields are read from one API response. Stability checks catch
// intermediate status writes which still carry Ready from a previous reconcile.
func assertStableNetworkPolicyStatus(
	ctx context.Context,
	k8sClient client.Client,
	operatorConditionName string,
	tc ManagedNetworkPolicyCase,
	managed bool,
) {
	check := func(g Gomega) {
		conditions := assertOperandNetworkPolicyManagement(ctx, g, k8sClient, tc, managed)
		g.Expect(conditionError(apimeta.FindStatusCondition(conditions, operatorv1alpha1.Ready),
			metav1.ConditionTrue, operatorv1alpha1.ReasonReady, "")).To(Succeed())
		assertHealthyParentNetworkPolicyStatus(ctx, g, k8sClient, operatorConditionName)
	}
	Eventually(check).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(Succeed())
	Consistently(check).WithTimeout(policyStabilityWindow).WithPolling(ShortInterval).Should(Succeed())
}

func assertOperandNetworkPolicyManagement(
	ctx context.Context,
	g Gomega,
	k8sClient client.Client,
	tc ManagedNetworkPolicyCase,
	managed bool,
) []metav1.Condition {
	state := "Managed"
	if !managed {
		state = "Unmanaged"
	}
	observed, err := readOperandNetworkPolicyStatus(ctx, k8sClient, tc.Kind)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(observed.NetworkPolicy.ManagementState).To(Equal(state))
	for _, policy := range observed.NetworkPolicy.LastAppliedPolicies {
		g.Expect(policy.Name).NotTo(BeEmpty())
		g.Expect(policy.LastAppliedTime.IsZero()).To(BeFalse(),
			"%s policy %s must record its last successful create or update", tc.Kind, policy.Name)
	}
	return observed.Conditions
}

func assertHealthyParentNetworkPolicyStatus(
	ctx context.Context,
	g Gomega,
	k8sClient client.Client,
	operatorConditionName string,
) {
	parent := &operatorv1alpha1.ZeroTrustWorkloadIdentityManager{}
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, parent)).To(Succeed())
	g.Expect(conditionError(apimeta.FindStatusCondition(parent.Status.Conditions, operatorv1alpha1.Ready),
		metav1.ConditionTrue, operatorv1alpha1.ReasonReady, "")).To(Succeed())
	upgradeable, err := GetUpgradeableCondition(ctx, k8sClient, OperatorNamespace, operatorConditionName)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditionError(upgradeable, metav1.ConditionTrue, operatorv1alpha1.ReasonReady, "")).To(Succeed())
}

func conditionError(condition *metav1.Condition, status metav1.ConditionStatus, reason, message string) error {
	if condition == nil {
		return fmt.Errorf("condition is missing")
	}
	if condition.Status != status ||
		(reason != "" && condition.Reason != reason) ||
		(message != "" && condition.Message != message) {
		return fmt.Errorf(
			"condition %s: status=%s reason=%s message=%s",
			condition.Type,
			condition.Status,
			condition.Reason,
			condition.Message,
		)
	}
	return nil
}

func assertIngressNetworkPolicies(ctx context.Context, clientset kubernetes.Interface) {
	for _, want := range ingressNetworkPolicies {
		np, err := clientset.NetworkingV1().NetworkPolicies(OperatorNamespace).Get(ctx, want.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "NetworkPolicy %s should exist", want.Name)
		Expect(equality.Semantic.DeepEqual(np.Spec, expectedIngressSpec(want.PodSelector))).To(BeTrue(),
			"NetworkPolicy %s rules: %#v", want.Name, np.Spec)
	}
}

func (tc ManagedNetworkPolicyCase) waitOperandCondition(
	ctx context.Context,
	k8sClient client.Client,
	condType string,
	status metav1.ConditionStatus,
	reason,
	message string,
) {
	Eventually(func() error {
		conditions, err := operandStatusConditions(ctx, k8sClient, tc.Kind)
		if err != nil {
			return err
		}
		return conditionError(apimeta.FindStatusCondition(conditions, condType), status, reason, message)
	}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(Succeed(), "%s condition %s", tc.Kind, condType)
}

func operandStatusConditions(
	ctx context.Context,
	k8sClient client.Client,
	kind string,
) ([]metav1.Condition, error) {
	obj := newManagedNetworkPolicyObject(kind)
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, obj); err != nil {
		return nil, err
	}
	switch o := obj.(type) {
	case *operatorv1alpha1.SpireServer:
		return o.Status.Conditions, nil
	case *operatorv1alpha1.SpireAgent:
		return o.Status.Conditions, nil
	case *operatorv1alpha1.SpireOIDCDiscoveryProvider:
		return o.Status.Conditions, nil
	default:
		return nil, fmt.Errorf("unsupported operand type %T", obj)
	}
}

func (tc ManagedNetworkPolicyCase) waitWorkload(ctx context.Context, clientset kubernetes.Interface) {
	switch tc.WorkloadKind {
	case "StatefulSet":
		WaitForStatefulSetReady(ctx, clientset, tc.WorkloadName, OperatorNamespace, DefaultTimeout)
	case "DaemonSet":
		WaitForDaemonSetAvailable(ctx, clientset, tc.WorkloadName, OperatorNamespace, DefaultTimeout)
	case "Deployment":
		WaitForDeploymentAvailable(ctx, clientset, tc.WorkloadName, OperatorNamespace, DefaultTimeout)
	default:
		Fail(fmt.Sprintf("unsupported workload kind %q", tc.WorkloadKind))
	}
}

func managedNetworkPolicyPodTemplate(
	ctx context.Context,
	clientset kubernetes.Interface,
	tc ManagedNetworkPolicyCase,
) corev1.PodSpec {
	spec, err := WorkloadPodTemplateSpec(ctx, clientset, OperatorNamespace, tc.WorkloadName, tc.WorkloadKind)
	Expect(err).NotTo(HaveOccurred(), "failed to get %s pod template", tc.Kind)
	return spec
}

func newManagedNetworkPolicyObject(kind string) client.Object {
	switch kind {
	case "SpireServer":
		return &operatorv1alpha1.SpireServer{}
	case "SpireAgent":
		return &operatorv1alpha1.SpireAgent{}
	case "SpireOIDCDiscoveryProvider":
		return &operatorv1alpha1.SpireOIDCDiscoveryProvider{}
	default:
		Fail(fmt.Sprintf("unsupported operand kind %q", kind))
		return nil
	}
}

func getManagedNetworkPolicy(
	ctx context.Context,
	k8sClient client.Client,
	tc ManagedNetworkPolicyCase,
) (client.Object, string) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(operatorv1alpha1.GroupVersion.String())
	obj.SetKind(tc.Kind)
	Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, obj)).To(Succeed(), "failed to get %s", tc.Kind)
	value, found, err := unstructured.NestedString(obj.Object, "spec", "managedNetworkPolicy")
	Expect(err).NotTo(HaveOccurred(), "failed to read managedNetworkPolicy on %s", tc.Kind)
	if !found {
		value = ""
	}
	return obj, value
}

func setManagedNetworkPolicy(
	ctx context.Context,
	k8sClient client.Client,
	tc ManagedNetworkPolicyCase,
	value string,
) {
	var before operandNetworkPolicyStatus
	if value == "false" {
		var err error
		before, err = readOperandNetworkPolicyStatus(ctx, k8sClient, tc.Kind)
		Expect(err).NotTo(HaveOccurred(), "failed to snapshot %s policy history", tc.Kind)
	}
	obj := newManagedNetworkPolicyObject(tc.Kind)
	obj.SetName("cluster")
	patch := fmt.Sprintf(`{"spec":{"managedNetworkPolicy":%q}}`, value)
	err := k8sClient.Patch(ctx, obj, client.RawPatch(types.MergePatchType, []byte(patch)))
	Expect(err).NotTo(HaveOccurred(), "failed to set managedNetworkPolicy=%s on %s", value, tc.Kind)
	if value == "false" {
		check := func(g Gomega) {
			after, err := readOperandNetworkPolicyStatus(ctx, k8sClient, tc.Kind)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(after.NetworkPolicy.ManagementState).To(Equal("Unmanaged"))
			g.Expect(after.NetworkPolicy.LastAppliedPolicies).To(ConsistOf(before.NetworkPolicy.LastAppliedPolicies),
				"%s must preserve policy names and timestamps when management is disabled", tc.Kind)
		}
		Eventually(check).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(Succeed())
		Consistently(check).WithTimeout(policyStabilityWindow).WithPolling(ShortInterval).Should(Succeed())
	}
}

func restoreManagedNetworkPolicy(k8sClient client.Client, tc ManagedNetworkPolicyCase) {
	DeferCleanup(func(ctx context.Context) {
		By("Restoring managedNetworkPolicy to true on " + tc.Kind)
		setManagedNetworkPolicy(ctx, k8sClient, tc, "true")
	})
}

// AssertNetworkPolicyManagementCombination checks one managed operand, or all
// operands unmanaged when managedKind is empty. Intentional opt-out stays healthy.
func AssertNetworkPolicyManagementCombination(
	ctx context.Context,
	k8sClient client.Client,
	operatorConditionName, managedKind string,
) {
	managed := map[string]bool{}
	if managedKind != "" {
		managedNetworkPolicyCase(managedKind) // Validate before changing any CRs.
		managed[managedKind] = true
	}
	configureNetworkPolicyManagement(ctx, k8sClient, managed)
	assertNetworkPolicyCombinationHealthy(ctx, k8sClient, operatorConditionName, managed)
}

func configureNetworkPolicyManagement(ctx context.Context, k8sClient client.Client, managed map[string]bool) {
	original := make(map[string]string, len(ManagedNetworkPolicyCases))
	for _, tc := range ManagedNetworkPolicyCases {
		_, original[tc.Kind] = getManagedNetworkPolicy(ctx, k8sClient, tc)
	}
	DeferCleanup(func(cleanupCtx context.Context) {
		By("Restoring all original NetworkPolicy management settings")
		var restoreErrors []error
		for _, tc := range ManagedNetworkPolicyCases {
			patch := fmt.Sprintf(`{"spec":{"managedNetworkPolicy":%q}}`, original[tc.Kind])
			if original[tc.Kind] == "" {
				patch = `{"spec":{"managedNetworkPolicy":null}}`
			}
			obj := newManagedNetworkPolicyObject(tc.Kind)
			obj.SetName("cluster")
			err := k8sClient.Patch(cleanupCtx, obj, client.RawPatch(types.MergePatchType, []byte(patch)))
			if err != nil {
				restoreErrors = append(restoreErrors, fmt.Errorf("restore %s: %w", tc.Kind, err))
			}
		}
		Expect(errors.Join(restoreErrors...)).NotTo(HaveOccurred())
		Eventually(func(g Gomega) {
			for _, tc := range ManagedNetworkPolicyCases {
				wasManaged := original[tc.Kind] != "false"
				assertOperandNetworkPolicyManagement(cleanupCtx, g, k8sClient, tc, wasManaged)
			}
		}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(Succeed())
	})

	for _, tc := range ManagedNetworkPolicyCases {
		By(fmt.Sprintf("Setting %s managedNetworkPolicy to %t", tc.Kind, managed[tc.Kind]))
		setManagedNetworkPolicy(ctx, k8sClient, tc, strconv.FormatBool(managed[tc.Kind]))
	}
	Eventually(func(g Gomega) {
		for _, tc := range ManagedNetworkPolicyCases {
			assertOperandNetworkPolicyManagement(ctx, g, k8sClient, tc, managed[tc.Kind])
		}
	}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(Succeed())
}

func assertNetworkPolicyCombinationHealthy(
	ctx context.Context,
	k8sClient client.Client,
	operatorConditionName string,
	managed map[string]bool,
) {
	check := func(g Gomega) {
		for _, tc := range ManagedNetworkPolicyCases {
			conditions := assertOperandNetworkPolicyManagement(ctx, g, k8sClient, tc, managed[tc.Kind])
			g.Expect(conditionError(apimeta.FindStatusCondition(conditions, operatorv1alpha1.Ready),
				metav1.ConditionTrue, operatorv1alpha1.ReasonReady, "")).To(Succeed())
		}
		assertHealthyParentNetworkPolicyStatus(ctx, g, k8sClient, operatorConditionName)
	}
	Eventually(check).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(Succeed())
	Consistently(check).WithTimeout(policyStabilityWindow).WithPolling(ShortInterval).Should(Succeed())
}

// AssertNetworkPolicyDrift checks that managed policies are repaired, while
// administrator edits survive until management is re-enabled when disabled.
func AssertNetworkPolicyDrift(
	ctx context.Context,
	k8sClient client.Client,
	clientset kubernetes.Interface,
	operatorConditionName string,
	tc ManagedNetworkPolicyCase,
	managed bool,
) {
	assertManagedNetworkPolicyEnabled(ctx, k8sClient, clientset, operatorConditionName, tc)
	original := snapshotNetworkPolicySpecs(ctx, clientset, operandManagedPolicies(tc))
	DeferCleanup(func(cleanupCtx context.Context) {
		setManagedNetworkPolicy(cleanupCtx, k8sClient, tc, "true")
		restoreNetworkPolicySpecs(cleanupCtx, clientset, original)
	})

	if !managed {
		By("Disabling NetworkPolicy management before editing policies")
		setManagedNetworkPolicy(ctx, k8sClient, tc, "false")
		assertManagedNetworkPolicyDisabled(ctx, k8sClient, clientset, operatorConditionName, tc)
	}

	By("Adding a harmless extra rule to each operand policy")
	edited := driftNetworkPolicySpecs(ctx, clientset, original)
	if !managed {
		By("Checking administrator edits are not overwritten while management is disabled")
		Consistently(func(g Gomega) {
			assertNetworkPolicySpecs(g, ctx, clientset, edited)
		}).WithTimeout(unmanagedDeleteWindow).WithPolling(ShortInterval).Should(Succeed())
		By("Re-enabling NetworkPolicy management")
		setManagedNetworkPolicy(ctx, k8sClient, tc, "true")
	}

	By("Waiting for the operator to restore the original policy rules")
	waitNetworkPolicySpecs(ctx, clientset, original)
	for name := range original {
		assertRecordedNetworkPolicies(ctx, k8sClient, tc.Kind, name)
	}
	assertManagedNetworkPolicyEnabled(ctx, k8sClient, clientset, operatorConditionName, tc)
}

// AssertOIDCIngressNamespaceLabel exercises both adding and removing the
// empty-valued ingress label on the same source namespace and destination pod.
func AssertOIDCIngressNamespaceLabel(ctx context.Context, clientset kubernetes.Interface) {
	namespace := createProbeNamespace(ctx, clientset)
	createProbePod(
		ctx,
		clientset,
		namespace,
		"np-oidc-client",
		map[string]string{networkPolicyProbeLabel: "true"},
		[]string{"sleep", "3600"},
	)
	oidcIP := operandPodIP(ctx, clientset, SpireOIDCDiscoveryProviderPodLabel)

	By("Rejecting a direct OIDC connection from an unlabelled namespace")
	expectTCPBlocked(ctx, namespace, "np-oidc-client", oidcIP, 8443)

	By("Allowing the same client after adding the empty-valued ingress namespace label")
	_, err := clientset.CoreV1().Namespaces().Patch(ctx, namespace, types.MergePatchType,
		[]byte(`{"metadata":{"labels":{"policy-group.network.openshift.io/ingress":""}}}`), metav1.PatchOptions{})
	Expect(err).NotTo(HaveOccurred())
	Eventually(func() error {
		return dialFromPod(ctx, namespace, "np-oidc-client", oidcIP, 8443)
	}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(Succeed())

	By("Rejecting new direct connections after removing the namespace label")
	_, err = clientset.CoreV1().Namespaces().Patch(ctx, namespace, types.MergePatchType,
		[]byte(`{"metadata":{"labels":{"policy-group.network.openshift.io/ingress":null}}}`), metav1.PatchOptions{})
	Expect(err).NotTo(HaveOccurred())
	expectTCPBlocked(ctx, namespace, "np-oidc-client", oidcIP, 8443)
}

// AssertNetworkPoliciesCreateOnly uses the existing OLM rollout pattern to
// check all operands together, restoring the Subscription environment on exit.
func AssertNetworkPoliciesCreateOnly(
	ctx context.Context,
	k8sClient client.Client,
	clientset kubernetes.Interface,
	subscriptionName string,
) {
	var policies []operandPolicy
	for _, tc := range ManagedNetworkPolicyCases {
		_, value := getManagedNetworkPolicy(ctx, k8sClient, tc)
		Expect(value).To(Equal("true"))
		policies = append(policies, operandManagedPolicies(tc)...)
	}
	original := snapshotNetworkPolicySpecs(ctx, clientset, policies)
	subscription := &unstructured.Unstructured{}
	subscription.SetAPIVersion("operators.coreos.com/v1alpha1")
	subscription.SetKind("Subscription")
	Expect(
		k8sClient.Get(ctx, client.ObjectKey{Namespace: OperatorNamespace, Name: subscriptionName}, subscription),
	).To(Succeed())
	originalEnv, _, err := unstructured.NestedSlice(subscription.Object, "spec", "config", "env")
	Expect(err).NotTo(HaveOccurred())
	originalMode, err := GetDeploymentEnvVar(
		ctx,
		clientset,
		OperatorNamespace,
		OperatorDeploymentName,
		CreateOnlyModeEnvVar,
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(strings.EqualFold(originalMode, "true")).To(BeFalse(), "this test starts with create-only mode disabled")

	// Preserve unrelated Subscription environment variables, including image settings.
	var createOnlyEnv []interface{}
	for _, item := range originalEnv {
		env, ok := item.(map[string]interface{})
		Expect(ok).To(BeTrue())
		if env["name"] != CreateOnlyModeEnvVar {
			createOnlyEnv = append(createOnlyEnv, item)
		}
	}
	createOnlyEnv = append(createOnlyEnv, map[string]interface{}{"name": CreateOnlyModeEnvVar, "value": "true"})

	DeferCleanup(func(cleanupCtx context.Context) {
		patchNetworkPolicySubscriptionEnv(cleanupCtx, k8sClient, subscription, originalEnv)
		WaitForDeploymentEnvVar(
			cleanupCtx,
			clientset,
			OperatorNamespace,
			OperatorDeploymentName,
			CreateOnlyModeEnvVar,
			originalMode,
			DefaultTimeout,
		)
		WaitForDeploymentAvailable(cleanupCtx, clientset, OperatorDeploymentName, OperatorNamespace, DefaultTimeout)
		restoreNetworkPolicySpecs(cleanupCtx, clientset, original)
	})

	By("Enabling create-only mode through the Subscription")
	patchNetworkPolicySubscriptionEnv(ctx, k8sClient, subscription, createOnlyEnv)
	WaitForDeploymentEnvVar(
		ctx,
		clientset,
		OperatorNamespace,
		OperatorDeploymentName,
		CreateOnlyModeEnvVar,
		"true",
		DefaultTimeout,
	)
	WaitForDeploymentAvailable(ctx, clientset, OperatorDeploymentName, OperatorNamespace, DefaultTimeout)
	for _, tc := range ManagedNetworkPolicyCases {
		tc.waitOperandCondition(ctx, k8sClient, "CreateOnlyMode", metav1.ConditionTrue, "", "")
	}

	By("Editing policies and checking create-only mode preserves the edits")
	edited := driftNetworkPolicySpecs(ctx, clientset, original)
	Consistently(func(g Gomega) {
		assertNetworkPolicySpecs(g, ctx, clientset, edited)
	}).WithTimeout(unmanagedDeleteWindow).WithPolling(ShortInterval).Should(Succeed())

	By("Restoring normal management and waiting for policy repair")
	patchNetworkPolicySubscriptionEnv(ctx, k8sClient, subscription, originalEnv)
	WaitForDeploymentEnvVar(
		ctx,
		clientset,
		OperatorNamespace,
		OperatorDeploymentName,
		CreateOnlyModeEnvVar,
		originalMode,
		DefaultTimeout,
	)
	WaitForDeploymentAvailable(ctx, clientset, OperatorDeploymentName, OperatorNamespace, DefaultTimeout)
	for _, tc := range ManagedNetworkPolicyCases {
		tc.waitOperandCondition(ctx, k8sClient, "CreateOnlyMode", metav1.ConditionFalse, "", "")
	}
	waitNetworkPolicySpecs(ctx, clientset, original)
}

func patchNetworkPolicySubscriptionEnv(
	ctx context.Context,
	k8sClient client.Client,
	subscription client.Object,
	env []interface{},
) {
	patch, err := json.Marshal(
		map[string]interface{}{"spec": map[string]interface{}{"config": map[string]interface{}{"env": env}}},
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient.Patch(ctx, subscription, client.RawPatch(types.MergePatchType, patch))).To(Succeed())
}

func snapshotNetworkPolicySpecs(
	ctx context.Context,
	clientset kubernetes.Interface,
	policies []operandPolicy,
) map[string]networkingv1.NetworkPolicySpec {
	specs := make(map[string]networkingv1.NetworkPolicySpec, len(policies))
	for _, policy := range policies {
		specs[policy.Name] = *getNetworkPolicy(ctx, clientset, policy.Name).Spec.DeepCopy()
	}
	return specs
}

func updateNetworkPolicySpec(
	ctx context.Context,
	clientset kubernetes.Interface,
	name string,
	spec networkingv1.NetworkPolicySpec,
) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		np, err := clientset.NetworkingV1().NetworkPolicies(OperatorNamespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		np.Spec = *spec.DeepCopy()
		_, err = clientset.NetworkingV1().NetworkPolicies(OperatorNamespace).Update(ctx, np, metav1.UpdateOptions{})
		return err
	})
}

func driftNetworkPolicySpecs(
	ctx context.Context,
	clientset kubernetes.Interface,
	original map[string]networkingv1.NetworkPolicySpec,
) map[string]networkingv1.NetworkPolicySpec {
	edited := make(map[string]networkingv1.NetworkPolicySpec, len(original))
	for name, spec := range original {
		changed := spec.DeepCopy()
		// Add a rule for a non-matching test label instead of removing working
		// ports, so drift tests do not interrupt SPIRE or monitoring traffic.
		peer := networkingv1.NetworkPolicyPeer{
			PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{networkPolicyProbeLabel: "no-such-pod"}},
		}
		ports := tcpPolicyPorts(65530)
		if len(changed.Ingress) > 0 {
			changed.Ingress = append(
				changed.Ingress,
				networkingv1.NetworkPolicyIngressRule{From: []networkingv1.NetworkPolicyPeer{peer}, Ports: ports},
			)
		} else {
			changed.Egress = append(
				changed.Egress,
				networkingv1.NetworkPolicyEgressRule{To: []networkingv1.NetworkPolicyPeer{peer}, Ports: ports},
			)
		}
		Expect(updateNetworkPolicySpec(ctx, clientset, name, *changed)).To(Succeed(), "failed to edit policy %s", name)
		edited[name] = *changed
	}
	return edited
}

func restoreNetworkPolicySpecs(
	ctx context.Context,
	clientset kubernetes.Interface,
	original map[string]networkingv1.NetworkPolicySpec,
) {
	for name, spec := range original {
		Expect(updateNetworkPolicySpec(ctx, clientset, name, spec)).To(Succeed(), "failed to restore policy %s", name)
	}
}

func assertNetworkPolicySpecs(
	g Gomega,
	ctx context.Context,
	clientset kubernetes.Interface,
	expected map[string]networkingv1.NetworkPolicySpec,
) {
	for name, spec := range expected {
		np, err := clientset.NetworkingV1().NetworkPolicies(OperatorNamespace).Get(ctx, name, metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred(), "failed to get policy %s", name)
		g.Expect(equality.Semantic.DeepEqual(np.Spec, spec)).To(
			BeTrue(),
			"unexpected rules in policy %s: %#v",
			name,
			np.Spec,
		)
	}
}

func waitNetworkPolicySpecs(
	ctx context.Context,
	clientset kubernetes.Interface,
	expected map[string]networkingv1.NetworkPolicySpec,
) {
	Eventually(func(g Gomega) {
		assertNetworkPolicySpecs(g, ctx, clientset, expected)
	}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(Succeed())
}

// AssertFederationNetworkPolicyBasic enables federation if absent on the
// disposable test cluster. Federation remains enabled until cluster teardown
// because the CRD does not allow it to be removed once configured.
func AssertFederationNetworkPolicyBasic(
	ctx context.Context,
	k8sClient client.Client,
	clientset kubernetes.Interface,
) {
	_, err := clientset.NetworkingV1().NetworkPolicies(OperatorNamespace).Get(
		ctx, federationPolicyName, metav1.GetOptions{},
	)
	Expect(err == nil || apierrors.IsNotFound(err)).To(BeTrue(), "failed to inspect federation policy: %v", err)
	policyAbsent := apierrors.IsNotFound(err)
	server, host := configureBasicFederation(ctx, k8sClient)
	assertBasicFederationResources(ctx, k8sClient, clientset, server, host)
	tc := managedNetworkPolicyCase("SpireServer")

	By("Checking server readiness and reported NetworkPolicy status")
	Eventually(func(g Gomega) {
		conditions := assertOperandNetworkPolicyManagement(ctx, g, k8sClient, tc, true)
		g.Expect(conditionError(apimeta.FindStatusCondition(conditions, operatorv1alpha1.Ready),
			metav1.ConditionTrue, operatorv1alpha1.ReasonReady, "")).To(Succeed())
	}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(Succeed())
	if policyAbsent {
		assertRecordedNetworkPolicies(ctx, k8sClient, tc.Kind, federationPolicyName)
	}

	assertBasicFederationBundle(ctx, clientset, host)
}

func configureBasicFederation(ctx context.Context, k8sClient client.Client) (*operatorv1alpha1.SpireServer, string) {
	server := &operatorv1alpha1.SpireServer{}
	Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, server)).To(Succeed())
	tc := managedNetworkPolicyCase("SpireServer")
	_, managed := getManagedNetworkPolicy(ctx, k8sClient, tc)
	Expect(managed).To(Equal("true"), "basic federation validation requires policy management enabled")
	if server.Spec.Federation == nil {
		By("Enabling https_web federation with a managed Route and OpenShift certificates")
		Expect(UpdateCRWithRetry(ctx, k8sClient, server, func() {
			server.Spec.Federation = &operatorv1alpha1.FederationConfig{
				ManagedRoute: "true",
				BundleEndpoint: operatorv1alpha1.BundleEndpointConfig{
					Profile:     operatorv1alpha1.HttpsWebProfile,
					RefreshHint: 60,
					HttpsWeb: &operatorv1alpha1.HttpsWebConfig{
						ServingCert: &operatorv1alpha1.ServingCertConfig{FileSyncInterval: 3600},
					},
				},
			}
		})).To(Succeed(), "failed to enable federation on the disposable test cluster")
	}
	Expect(server.Spec.Federation.ManagedRoute).To(Equal("true"))
	Expect(server.Spec.Federation.BundleEndpoint.Profile).To(
		Equal(operatorv1alpha1.HttpsWebProfile),
		"this test requires https_web/servingCert; existing immutable profiles are not changed",
	)
	Expect(server.Spec.Federation.BundleEndpoint.HttpsWeb).NotTo(BeNil())
	Expect(server.Spec.Federation.BundleEndpoint.HttpsWeb.ServingCert).NotTo(BeNil())
	parent := &operatorv1alpha1.ZeroTrustWorkloadIdentityManager{}
	Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, parent)).To(Succeed())
	host := "federation." + parent.Spec.TrustDomain

	return server, host
}

func assertBasicFederationResources(
	ctx context.Context,
	k8sClient client.Client,
	clientset kubernetes.Interface,
	server *operatorv1alpha1.SpireServer,
	host string,
) {

	By("Checking federation policy rules, Service port, admitted Route and completed server rollout")
	Eventually(func(g Gomega) {
		np, err := clientset.NetworkingV1().NetworkPolicies(OperatorNamespace).Get(
			ctx, federationPolicyName, metav1.GetOptions{},
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(np.Spec.PodSelector).To(
			Equal(metav1.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/name": "spire-server"}}),
		)
		g.Expect(np.Spec.PolicyTypes).To(Equal([]networkingv1.PolicyType{networkingv1.PolicyTypeIngress}))
		g.Expect(np.Spec.Ingress).To(Equal([]networkingv1.NetworkPolicyIngressRule{{
			From: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"policy-group.network.openshift.io/ingress": ""},
					},
				},
			},
			Ports: tcpPolicyPorts(8443),
		}}))
		owner := metav1.GetControllerOf(np)
		g.Expect(owner).NotTo(BeNil())
		g.Expect(owner.UID).To(Equal(server.UID))
		svc, err := clientset.CoreV1().Services(OperatorNamespace).Get(ctx, "spire-server", metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(svc.Spec.Selector).To(HaveKeyWithValue("app.kubernetes.io/name", "spire-server"))
		g.Expect(svc.Spec.Ports).To(ContainElement(And(
			HaveField("Name", "federation"), HaveField("Port", int32(8443)),
			HaveField("TargetPort", intstr.FromInt32(8443)), HaveField("Protocol", corev1.ProtocolTCP),
		)))
		route := &unstructured.Unstructured{}
		route.SetAPIVersion("route.openshift.io/v1")
		route.SetKind("Route")
		g.Expect(
			k8sClient.Get(ctx, client.ObjectKey{Namespace: OperatorNamespace, Name: "spire-server-federation"}, route),
		).To(Succeed())
		for path, expected := range map[string]string{
			"host":            host,
			"to.name":         "spire-server",
			"port.targetPort": "federation",
			"tls.termination": "reencrypt",
		} {
			value, _, err := unstructured.NestedString(
				route.Object,
				append([]string{"spec"}, strings.Split(path, ".")...)...,
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(value).To(Equal(expected), "Route spec.%s", path)
		}
		ingress, _, err := unstructured.NestedSlice(route.Object, "status", "ingress")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(ingress).To(ContainElement(HaveKeyWithValue("conditions", ContainElement(And(
			HaveKeyWithValue("type", "Admitted"), HaveKeyWithValue("status", "True"),
		)))))
		sts, err := clientset.AppsV1().StatefulSets(OperatorNamespace).Get(
			ctx, SpireServerStatefulSetName, metav1.GetOptions{},
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(sts.Status.ObservedGeneration).To(BeNumerically(">=", sts.Generation))
		g.Expect(sts.Status.CurrentRevision).To(Equal(sts.Status.UpdateRevision))
		g.Expect(sts.Status.UpdatedReplicas).To(Equal(*sts.Spec.Replicas))
		g.Expect(IsStatefulSetReady(sts)).To(BeTrue())
	}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(Succeed())

}

func assertBasicFederationBundle(ctx context.Context, clientset kubernetes.Interface, host string) {

	By("Fetching a federation trust bundle through the Route with verified HTTPS")
	ca, err := clientset.CoreV1().ConfigMaps("openshift-config-managed").Get(
		ctx, "default-ingress-cert", metav1.GetOptions{},
	)
	Expect(err).NotTo(HaveOccurred())
	roots, err := x509.SystemCertPool()
	Expect(err).NotTo(HaveOccurred())
	Expect(roots.AppendCertsFromPEM([]byte(ca.Data["ca-bundle.crt"]))).To(BeTrue())
	transport := &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
	}
	defer transport.CloseIdleConnections()
	httpClient := &http.Client{Transport: transport, Timeout: 15 * time.Second}
	Eventually(func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+host, nil)
		if err != nil {
			return err
		}
		response, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("federation endpoint returned HTTP %d", response.StatusCode)
		}
		var bundle struct {
			Keys []struct {
				Use string   `json:"use"`
				X5C []string `json:"x5c"`
			} `json:"keys"`
		}
		if err := json.NewDecoder(io.LimitReader(response.Body, 1024*1024)).Decode(&bundle); err != nil {
			return err
		}
		for _, key := range bundle.Keys {
			if key.Use == "x509-svid" && len(key.X5C) == 1 {
				der, err := base64.StdEncoding.DecodeString(key.X5C[0])
				if err != nil {
					return err
				}
				cert, err := x509.ParseCertificate(der)
				if err != nil {
					return err
				}
				if cert.IsCA && !time.Now().Before(cert.NotBefore) && !time.Now().After(cert.NotAfter) {
					return nil
				}
			}
		}
		return fmt.Errorf("federation response contains no currently valid X.509 CA certificate")
	}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(Succeed())

}

// AssertOperandNetworkPolicy checks the operand's policies and live traffic while
// managedNetworkPolicy is "true". discoveryIssuer is https://oidc-discovery.apps.<cluster>.
func AssertOperandNetworkPolicy(
	ctx context.Context,
	k8sClient client.Client,
	clientset kubernetes.Interface,
	tc ManagedNetworkPolicyCase,
	discoveryIssuer string,
) {
	_, value := getManagedNetworkPolicy(ctx, k8sClient, tc)
	Expect(value).To(Equal("true"), "%s managedNetworkPolicy should be true", tc.Kind)
	Eventually(func(g Gomega) {
		assertOperandNetworkPolicyManagement(ctx, g, k8sClient, tc, true)
	}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(Succeed())

	switch tc.Kind {
	case "SpireServer":
		By("Checking the server ingress TCP 8081 rule")
		np := getNetworkPolicy(ctx, clientset, managedIngressPolicyName("spire-server"))
		Expect(
			equality.Semantic.DeepEqual(np.Spec, expectedIngressSpec("spire-server")),
		).To(BeTrue(), "server ingress should allow TCP 8081 from spire-agent, got %#v", np.Spec)
		assertAllowAllEgress(ctx, clientset, serverEgressPolicyName, "spire-server")
		assertNoSampleNetworkPolicies(ctx, clientset)

		listenerNS := createProbeNamespace(ctx, clientset)
		By("Creating the listener and the agent-labeled client")
		createProbePod(
			ctx,
			clientset,
			listenerNS,
			"np-listener",
			map[string]string{networkPolicyProbeLabel: "true"},
			[]string{"sleep", "3600"},
		)
		createProbePod(ctx, clientset, OperatorNamespace, "np-agent-client", map[string]string{
			"app.kubernetes.io/name": "spire-agent",
			networkPolicyProbeLabel:  "true",
		}, []string{"sleep", "3600"})
		serverIP := operandPodIP(ctx, clientset, SpireServerPodLabel)
		By("Opening TCP 8081 from an agent-labeled pod")
		Eventually(func() error {
			return dialFromPod(ctx, OperatorNamespace, "np-agent-client", serverIP, 8081)
		}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(Succeed(), "agent should open TCP 8081")
		By("Refusing TCP 8081 from a pod that is not an agent")
		expectTCPBlocked(ctx, listenerNS, "np-listener", serverIP, 8081)

		listenerIP := createHTTPProbeListener(ctx, clientset, listenerNS)
		createProbePod(ctx, clientset, OperatorNamespace, "np-server-client", map[string]string{
			"app.kubernetes.io/name": "spire-server",
			networkPolicyProbeLabel:  "true",
		}, []string{"sleep", "3600"})
		expectHTTP200(ctx, OperatorNamespace, "np-server-client", "http://"+listenerIP+":8080/")
		expectHTTP200(ctx, OperatorNamespace, "np-server-client", "https://kubernetes.default.svc.cluster.local/readyz")
	case "SpireAgent":
		assertAllowAllEgress(ctx, clientset, agentEgressPolicyName, "spire-agent")
		listenerNS := createProbeNamespace(ctx, clientset)
		listenerIP := createHTTPProbeListener(ctx, clientset, listenerNS)
		createProbePod(ctx, clientset, OperatorNamespace, "np-agent-client", map[string]string{
			"app.kubernetes.io/name": "spire-agent",
			networkPolicyProbeLabel:  "true",
		}, []string{"sleep", "3600"})
		expectHTTP200(ctx, OperatorNamespace, "np-agent-client", "http://"+listenerIP+":8080/")
	case "SpireOIDCDiscoveryProvider":
		Expect(discoveryIssuer).NotTo(BeEmpty(), "OIDC issuer URL is empty")
		_, err := clientset.NetworkingV1().NetworkPolicies(OperatorNamespace).Get(
			ctx, oidcEgressPolicyName, metav1.GetOptions{},
		)
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "OIDC egress policy should be absent, err=%v", err)
		listenerNS := createProbeNamespace(ctx, clientset)
		createProbePod(
			ctx,
			clientset,
			listenerNS,
			"np-listener",
			map[string]string{networkPolicyProbeLabel: "true"},
			[]string{"sleep", "3600"},
		)
		oidcIP := operandPodIP(ctx, clientset, SpireOIDCDiscoveryProviderPodLabel)
		url := discoveryIssuer + "/.well-known/openid-configuration"
		By("Checking the same client can read OIDC discovery through the Route")
		body := expectHTTP200(ctx, listenerNS, "np-listener", url)
		var discovery struct {
			Issuer string `json:"issuer"`
		}
		Expect(json.Unmarshal([]byte(body), &discovery)).To(Succeed())
		Expect(discovery.Issuer).To(Equal(discoveryIssuer))
		By("Checking direct TCP 8443 times out while Route access remains available")
		expectTCPBlocked(ctx, listenerNS, "np-listener", oidcIP, 8443)
		expectHTTP200(ctx, listenerNS, "np-listener", url)

	default:
		Fail(fmt.Sprintf("unsupported operand kind %q", tc.Kind))
	}
}

// AssertDisabledManagementRecreatesDeletedPolicies sets managedNetworkPolicy to
// "false" on one operand. ManagementState becomes Unmanaged, application history
// is preserved, and Ready stays True while its policies remain. Deleting those policies
// leaves them absent until the field is "true" again, which makes reconciliation
// recreate them and Ready stay True.
func AssertDisabledManagementRecreatesDeletedPolicies(
	ctx context.Context,
	k8sClient client.Client,
	clientset kubernetes.Interface,
	operatorConditionName string,
	tc ManagedNetworkPolicyCase,
) {
	policies := operandManagedPolicies(tc)
	DeferCleanup(func(cleanupCtx context.Context) {
		By("Restoring managedNetworkPolicy to true on " + tc.Kind)
		setManagedNetworkPolicy(cleanupCtx, k8sClient, tc, "true")
		for _, policy := range policies {
			waitForNetworkPolicy(cleanupCtx, clientset, policy.Name, policy.ready)
		}
	})

	By("Setting managedNetworkPolicy to false on " + tc.Kind)
	setManagedNetworkPolicy(ctx, k8sClient, tc, "false")
	_, stored := getManagedNetworkPolicy(ctx, k8sClient, tc)
	Expect(stored).To(Equal("false"), "%s should persist managedNetworkPolicy=false", tc.Kind)
	assertManagedNetworkPolicyDisabled(ctx, k8sClient, clientset, operatorConditionName, tc)

	By("Deleting managed NetworkPolicies while management is off")
	for _, policy := range policies {
		deleteNetworkPolicy(ctx, clientset, policy.Name)
	}
	for _, policy := range policies {
		name := policy.Name
		Eventually(func() bool {
			return networkPolicyAbsent(ctx, clientset, name)
		}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(BeTrue(), "delete should remove %s", name)
	}
	Consistently(func() string {
		for _, policy := range policies {
			if !networkPolicyAbsent(ctx, clientset, policy.Name) {
				return policy.Name
			}
		}
		return ""
	}).WithTimeout(unmanagedDeleteWindow).WithPolling(ShortInterval).Should(BeEmpty(),
		"controller recreated a NetworkPolicy while managedNetworkPolicy is false")

	By("Setting managedNetworkPolicy to true on " + tc.Kind)
	setManagedNetworkPolicy(ctx, k8sClient, tc, "true")
	_, stored = getManagedNetworkPolicy(ctx, k8sClient, tc)
	Expect(stored).To(Equal("true"), "%s should persist managedNetworkPolicy=true", tc.Kind)

	By("Waiting for the deleted NetworkPolicies to be recreated")
	for _, policy := range policies {
		waitForNetworkPolicy(ctx, clientset, policy.Name, policy.ready)
		assertRecordedNetworkPolicies(ctx, k8sClient, tc.Kind, policy.Name)
	}
	assertManagedNetworkPolicyEnabled(ctx, k8sClient, clientset, operatorConditionName, tc)
	tc.waitWorkload(ctx, clientset)
}

func managedNetworkPolicyCase(kind string) ManagedNetworkPolicyCase {
	for _, tc := range ManagedNetworkPolicyCases {
		if tc.Kind == kind {
			return tc
		}
	}
	Fail(fmt.Sprintf("no managedNetworkPolicy case for %s", kind))
	return ManagedNetworkPolicyCase{}
}

func managedIngressPolicyName(podSelector string) string {
	for _, policy := range ingressNetworkPolicies {
		if policy.PodSelector == podSelector {
			return policy.Name
		}
	}
	Fail(fmt.Sprintf("no ingress policy for selector %s", podSelector))
	return ""
}

type operandPolicy struct {
	Name  string
	ready func(*networkingv1.NetworkPolicy) bool
}

func operandManagedPolicies(tc ManagedNetworkPolicyCase) []operandPolicy {
	selector := strings.TrimPrefix(tc.PodSelector, "app.kubernetes.io/name=")
	var policies []operandPolicy
	for _, policy := range ingressNetworkPolicies {
		if policy.PodSelector != selector {
			continue
		}
		name := policy.Name
		policies = append(policies, operandPolicy{Name: name, ready: func(np *networkingv1.NetworkPolicy) bool {
			return equality.Semantic.DeepEqual(np.Spec, expectedIngressSpec(selector))
		}})
	}
	for _, policy := range egressAllowAllPolicies {
		if policy.PodSelector == selector {
			policies = append(policies, operandPolicy{Name: policy.Name, ready: allowAllEgress})
		}
	}
	Expect(policies).NotTo(BeEmpty(), "%s has no managed NetworkPolicies", tc.Kind)
	return policies
}

func networkPolicyAbsent(ctx context.Context, clientset kubernetes.Interface, name string) bool {
	_, err := clientset.NetworkingV1().NetworkPolicies(OperatorNamespace).Get(ctx, name, metav1.GetOptions{})
	return apierrors.IsNotFound(err)
}

func assertEgressAllowAllPresent(ctx context.Context, clientset kubernetes.Interface) {
	for _, want := range egressAllowAllPolicies {
		assertAllowAllEgress(ctx, clientset, want.Name, want.PodSelector)
	}
}

func assertAllowAllEgress(ctx context.Context, clientset kubernetes.Interface, name, podSelector string) {
	np := getNetworkPolicy(ctx, clientset, name)
	Expect(np.Spec.PodSelector.MatchLabels).To(HaveKeyWithValue("app.kubernetes.io/name", podSelector),
		"NetworkPolicy %s pod selector", name)
	Expect(allowAllEgress(np)).To(BeTrue(), "NetworkPolicy %s should allow all egress, got %#v", name, np.Spec)
}

func assertNoSampleNetworkPolicies(ctx context.Context, clientset kubernetes.Interface) {
	list, err := clientset.NetworkingV1().NetworkPolicies(OperatorNamespace).List(ctx, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())
	for _, item := range list.Items {
		if item.Name == federationPolicyName {
			continue // Managed federation ingress is not a reference-only sample.
		}
		for _, fragment := range []string{"baseline", "database", "vault", "proxy", "federation"} {
			Expect(item.Name).NotTo(ContainSubstring(fragment), "operator reconciled sample policy %s", item.Name)
		}
	}
}

func expectHTTP200(ctx context.Context, namespace, pod, url string) string {
	var body string
	By("Opening " + url + " from " + pod)
	Eventually(func() string {
		stdout, stderr, err := wgetFromPod(ctx, namespace, pod, url)
		if err != nil {
			return err.Error() + " " + stderr
		}
		if status, err := responseStatus(stderr); err != nil || status != http.StatusOK {
			return stderr
		}
		body = stdout
		return ""
	}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(
		BeEmpty(), "%s should get HTTP 200 from %s", pod, url,
	)
	return body
}

// responseStatus reads the final HTTP status line, not digits in URLs or logs.
func responseStatus(output string) (int, error) {
	status := 0
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasPrefix(fields[0], "HTTP/") {
			value, err := strconv.Atoi(fields[1])
			if err != nil {
				return 0, fmt.Errorf("invalid HTTP status line %q: %w", line, err)
			}
			status = value
		}
	}
	if status == 0 {
		return 0, fmt.Errorf("missing HTTP status in response: %s", output)
	}
	return status, nil
}

// Read NetworkPolicy status through unstructured objects so these E2Es can be
// built independently of the operator API version installed on the cluster.
type operandNetworkPolicyStatus struct {
	Conditions    []metav1.Condition `json:"conditions"`
	NetworkPolicy struct {
		ManagementState     string `json:"managementState"`
		LastAppliedPolicies []struct {
			Name            string      `json:"name"`
			LastAppliedTime metav1.Time `json:"lastAppliedTime"`
		} `json:"lastAppliedPolicies"`
	} `json:"networkPolicy"`
}

func readOperandNetworkPolicyStatus(
	ctx context.Context,
	k8sClient client.Client,
	kind string,
) (operandNetworkPolicyStatus, error) {
	var observed operandNetworkPolicyStatus
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(operatorv1alpha1.GroupVersion.String())
	obj.SetKind(kind)
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, obj); err != nil {
		return observed, err
	}
	statusMap, _, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil {
		return observed, err
	}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(statusMap, &observed)
	return observed, err
}

// assertRecordedNetworkPolicies is used after successful creates or updates.
// A no-op reconciliation preserves existing history and timestamps.
func assertRecordedNetworkPolicies(
	ctx context.Context,
	k8sClient client.Client,
	kind string,
	names ...string,
) {
	Eventually(func(g Gomega) {
		observed, err := readOperandNetworkPolicyStatus(ctx, k8sClient, kind)
		g.Expect(err).NotTo(HaveOccurred())
		applied := make([]string, 0, len(observed.NetworkPolicy.LastAppliedPolicies))
		for _, policy := range observed.NetworkPolicy.LastAppliedPolicies {
			g.Expect(policy.LastAppliedTime.IsZero()).To(BeFalse(), "policy %s lastAppliedTime", policy.Name)
			applied = append(applied, policy.Name)
		}
		g.Expect(applied).To(ContainElements(names), "%s successfully applied policies", kind)
	}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(Succeed())
}

func getNetworkPolicy(
	ctx context.Context,
	clientset kubernetes.Interface,
	name string,
) *networkingv1.NetworkPolicy {
	np, err := clientset.NetworkingV1().NetworkPolicies(OperatorNamespace).Get(ctx, name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "NetworkPolicy %s should exist", name)
	return np
}

func deleteNetworkPolicy(ctx context.Context, clientset kubernetes.Interface, name string) {
	err := clientset.NetworkingV1().NetworkPolicies(OperatorNamespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		Expect(err).NotTo(HaveOccurred(), "failed to delete NetworkPolicy %s", name)
	}
}

func waitForNetworkPolicy(
	ctx context.Context,
	clientset kubernetes.Interface,
	name string,
	ready func(*networkingv1.NetworkPolicy) bool,
) {
	Eventually(func() string {
		np, err := clientset.NetworkingV1().NetworkPolicies(OperatorNamespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err.Error()
		}
		if !ready(np) {
			return fmt.Sprintf("%#v", np.Spec)
		}
		return ""
	}).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(
		BeEmpty(), "NetworkPolicy %s was not reconciled", name,
	)
}

// expectedIngressSpec is the single baseline definition used for initial
// validation and for policies recreated after management is re-enabled.
func expectedIngressSpec(selector string) networkingv1.NetworkPolicySpec {
	namespacePeer := func(key, value string) networkingv1.NetworkPolicyPeer {
		return networkingv1.NetworkPolicyPeer{
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{key: value}},
		}
	}
	monitoring := []networkingv1.NetworkPolicyPeer{
		namespacePeer("kubernetes.io/metadata.name", "openshift-monitoring"),
	}
	var rules []networkingv1.NetworkPolicyIngressRule
	switch selector {
	case "spire-server":
		rules = []networkingv1.NetworkPolicyIngressRule{
			{
				From: []networkingv1.NetworkPolicyPeer{
					{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/name": "spire-agent"}}},
				},
				Ports: tcpPolicyPorts(8081),
			},
			{Ports: tcpPolicyPorts(9443)},
			{From: monitoring, Ports: tcpPolicyPorts(9402, 8082)},
		}
	case "spire-agent":
		rules = []networkingv1.NetworkPolicyIngressRule{{From: monitoring, Ports: tcpPolicyPorts(9402)}}
	case "spiffe-oidc-discovery-provider":
		rules = []networkingv1.NetworkPolicyIngressRule{{From: []networkingv1.NetworkPolicyPeer{
			namespacePeer("policy-group.network.openshift.io/ingress", ""),
		}, Ports: tcpPolicyPorts(8443)}}
	default:
		panic("unknown operand selector: " + selector)
	}
	return networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/name": selector}},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}, Ingress: rules,
	}
}

func tcpPolicyPorts(ports ...int32) []networkingv1.NetworkPolicyPort {
	result := make([]networkingv1.NetworkPolicyPort, 0, len(ports))
	for _, port := range ports {
		result = append(
			result,
			networkingv1.NetworkPolicyPort{Protocol: ptr.To(corev1.ProtocolTCP), Port: ptr.To(intstr.FromInt32(port))},
		)
	}
	return result
}

func allowAllEgress(np *networkingv1.NetworkPolicy) bool {
	if len(np.Spec.PolicyTypes) != 1 || np.Spec.PolicyTypes[0] != networkingv1.PolicyTypeEgress {
		return false
	}
	if len(np.Spec.Egress) != 1 {
		return false
	}
	return len(np.Spec.Egress[0].To) == 0 && len(np.Spec.Egress[0].Ports) == 0
}

func operandPodIP(ctx context.Context, clientset kubernetes.Interface, selector string) string {
	pods, err := clientset.CoreV1().Pods(OperatorNamespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	Expect(err).NotTo(HaveOccurred(), "failed to list pods %s", selector)
	for _, pod := range FilterActivePods(pods.Items) {
		if pod.Labels[networkPolicyProbeLabel] == "true" || !IsPodReady(&pod) || pod.Status.PodIP == "" {
			continue
		}
		return pod.Status.PodIP
	}
	Fail(fmt.Sprintf("no ready pod IP for %s", selector))
	return ""
}

func createProbeNamespace(ctx context.Context, clientset kubernetes.Interface) string {
	buf := make([]byte, 3)
	_, _ = rand.Read(buf)
	name := "e2e-np-" + hex.EncodeToString(buf)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: name,
		Labels: map[string]string{
			"pod-security.kubernetes.io/enforce": "restricted",
			"pod-security.kubernetes.io/audit":   "restricted",
			"pod-security.kubernetes.io/warn":    "restricted",
		},
	}}
	_, err := clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to create namespace %s", name)
	DeferCleanup(func(cleanupCtx context.Context) {
		err := clientset.CoreV1().Namespaces().Delete(cleanupCtx, name, metav1.DeleteOptions{})
		if !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred(), "cleanup: failed to delete namespace %s", name)
		}
	})
	return name
}

func createHTTPProbeListener(ctx context.Context, clientset kubernetes.Interface, namespace string) string {
	return createProbePod(
		ctx, clientset, namespace, "np-httpd",
		map[string]string{networkPolicyProbeLabel: "true"},
		[]string{"sh", "-c", "mkdir -p /tmp/www && echo ok > /tmp/www/index.html && exec httpd -f -p 8080 -h /tmp/www"},
	)
}

func createProbePod(
	ctx context.Context,
	clientset kubernetes.Interface,
	namespace,
	name string,
	labels map[string]string,
	command []string,
) string {
	securityContext := &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   ptr.To(true),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Containers: []corev1.Container{{
				Name:            probeContainerName,
				Image:           AppContainerImage,
				Command:         command,
				SecurityContext: securityContext,
			}},
		},
	}
	_, err := clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to create pod %s/%s", namespace, name)
	DeferCleanup(func(cleanupCtx context.Context) {
		err := clientset.CoreV1().Pods(namespace).Delete(cleanupCtx, name, metav1.DeleteOptions{})
		if !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred(), "cleanup: failed to delete pod %s/%s", namespace, name)
		}
	})
	WaitForPodReady(ctx, clientset, name, namespace, DefaultTimeout)
	created, err := clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to get pod %s/%s", namespace, name)
	Expect(created.Status.PodIP).NotTo(BeEmpty(), "pod %s/%s has no IP", namespace, name)
	// Prove the probe command and its flags work before interpreting a failed
	// connection as NetworkPolicy enforcement. Loopback is local to this pod.
	_, stderr, err := ExecInPod(ctx, namespace, name, probeContainerName, []string{
		"sh", "-c", "command -v nc && command -v wget && httpd -p 18080 -h /tmp && nc -z -v -w 5 127.0.0.1 18080",
	})
	Expect(err).NotTo(HaveOccurred(), "probe preflight failed in %s/%s: %s", namespace, name, stderr)
	return created.Status.PodIP
}

func dialFromPod(ctx context.Context, namespace, pod, ip string, port int) error {
	connected, err := probeTCPConnection(ctx, namespace, pod, ip, port)
	if err != nil {
		return err
	}
	if !connected {
		return fmt.Errorf("TCP connection to %s:%d failed", ip, port)
	}
	return nil
}

func probeTCPConnection(ctx context.Context, namespace, pod, ip string, port int) (bool, error) {
	// The shell returns success for both TCP outcomes. An oc exec failure or
	// unexpected nc exit code remains a test error, never evidence of blocking.
	stdout, stderr, err := ExecInPod(ctx, namespace, pod, probeContainerName, []string{
		"sh", "-c", `nc -z -v -w 5 "$1" "$2"
result=$?
case "$result" in
  0) echo connected;;
  1) echo unreachable;;
  *) exit "$result";;
esac`,
		"probe", ip, fmt.Sprintf("%d", port),
	})
	if err != nil {
		return false, fmt.Errorf("probe execution failed: %w: %s", err, strings.TrimSpace(stderr))
	}
	return parseTCPProbeResult(stdout, stderr)
}

func parseTCPProbeResult(stdout, stderr string) (bool, error) {
	switch strings.TrimSpace(stdout) {
	case "connected":
		return true, nil
	case "unreachable":
		if strings.Contains(strings.ToLower(stderr), "timed out") {
			return false, nil
		}
		return false, fmt.Errorf("probe failed without a network timeout: %s", stderr)
	default:
		return false, fmt.Errorf("unexpected probe output %q: %s", stdout, stderr)
	}
}

func expectTCPBlocked(ctx context.Context, namespace, pod, ip string, port int) {
	check := func(g Gomega) {
		connected, err := probeTCPConnection(ctx, namespace, pod, ip, port)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(connected).To(BeFalse())
	}
	Eventually(check).WithTimeout(DefaultTimeout).WithPolling(ShortInterval).Should(Succeed(),
		"TCP %s:%d should time out from %s/%s", ip, port, namespace, pod)
	Consistently(check).WithTimeout(policyStabilityWindow).WithPolling(ShortInterval).Should(Succeed())
}

func wgetFromPod(ctx context.Context, namespace, pod, url string) (string, string, error) {
	return ExecInPod(ctx, namespace, pod, probeContainerName, []string{
		"wget", "-S", "-O", "-", "-T", "10", url,
	})
}
