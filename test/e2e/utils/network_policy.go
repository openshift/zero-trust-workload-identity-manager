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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	operatorv1alpha1 "github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	SpireServerNetworkPolicyIngressName           = "ztwim-sys-spire-server-ingress"
	SpireServerNetworkPolicyEgressBaselineName    = "ztwim-sys-spire-server-egress-baseline"
	SpireServerNetworkPolicyEgressAllowAllName    = "ztwim-sys-spire-server-egress-allow-all"
	SpireServerNetworkPolicyFederationIngressName = "ztwim-sys-spire-server-federation-ingress"
	SpireServerNetworkPolicyFederationEgressName  = "ztwim-sys-spire-server-federation-egress"
	SpireServerNetworkPolicyVaultEgressName       = "ztwim-sys-spire-server-vault-egress"
	SpireServerNetworkPolicyDatabaseEgressName    = "ztwim-sys-spire-server-database-egress"
	SpireServerNetworkPolicyProxyEgressName       = "ztwim-sys-spire-server-proxy-egress"

	SpireAgentNetworkPolicyIngressName        = "ztwim-sys-spire-agent-ingress"
	SpireAgentNetworkPolicyEgressBaselineName = "ztwim-sys-spire-agent-egress-baseline"
	SpireAgentNetworkPolicyEgressAllowAllName = "ztwim-sys-spire-agent-egress-allow-all"
	SpireAgentNetworkPolicyProxyEgressName    = "ztwim-sys-spire-agent-proxy-egress"

	SpireOIDCNetworkPolicyIngressName = "ztwim-sys-spire-oidc-discovery-provider-ingress"
	SpireOIDCNetworkPolicyProxyName   = "ztwim-sys-spire-oidc-discovery-provider-proxy-egress"

	OperandCRName = "cluster"
)

// WaitForNetworkPolicy waits until a NetworkPolicy exists in the namespace.
func WaitForNetworkPolicy(ctx context.Context, clientset kubernetes.Interface, namespace, name string, timeout time.Duration) *networkingv1.NetworkPolicy {
	var np *networkingv1.NetworkPolicy
	Eventually(func() error {
		got, err := clientset.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		np = got
		return nil
	}).WithTimeout(timeout).WithPolling(ShortInterval).Should(Succeed(),
		"NetworkPolicy %s/%s should exist within %v", namespace, name, timeout)
	return np
}

// WaitForNetworkPolicyAbsent waits until a NetworkPolicy is deleted.
func WaitForNetworkPolicyAbsent(ctx context.Context, clientset kubernetes.Interface, namespace, name string, timeout time.Duration) {
	Eventually(func() bool {
		_, err := clientset.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}).WithTimeout(timeout).WithPolling(ShortInterval).Should(BeTrue(),
		"NetworkPolicy %s/%s should be absent within %v", namespace, name, timeout)
}

// NetworkPolicyAllowsAllEgress reports whether any egress rule allows all destinations and ports.
func NetworkPolicyAllowsAllEgress(np *networkingv1.NetworkPolicy) bool {
	for _, rule := range np.Spec.Egress {
		if len(rule.To) == 0 && len(rule.Ports) == 0 {
			return true
		}
	}
	return false
}

// NetworkPolicyHasEgressTCPPort returns true if any egress rule exposes port on TCP.
func NetworkPolicyHasEgressTCPPort(np *networkingv1.NetworkPolicy, port int32) bool {
	for _, rule := range np.Spec.Egress {
		for _, p := range rule.Ports {
			if p.Protocol != nil && *p.Protocol != corev1.ProtocolTCP {
				continue
			}
			if p.Port != nil && p.Port.IntVal == port {
				return true
			}
		}
	}
	return false
}

// ExpectNetworkPolicyOwner verifies controller owner reference on a NetworkPolicy.
func ExpectNetworkPolicyOwner(np *networkingv1.NetworkPolicy, kind, name string) {
	Expect(np.OwnerReferences).NotTo(BeEmpty())
	var found bool
	for _, ref := range np.OwnerReferences {
		if ref.Kind == kind && ref.Name == name && ref.Controller != nil && *ref.Controller {
			found = true
			break
		}
	}
	Expect(found).To(BeTrue(), "NetworkPolicy %s should be owned by %s/%s", np.Name, kind, name)
}

// NetworkPolicySelectsPodLabels returns true when the policy podSelector matches all given labels.
func NetworkPolicySelectsPodLabels(np *networkingv1.NetworkPolicy, labels map[string]string) bool {
	if np.Spec.PodSelector.MatchLabels == nil {
		return false
	}
	for k, v := range labels {
		if np.Spec.PodSelector.MatchLabels[k] != v {
			return false
		}
	}
	return true
}

// WaitForOperandNetworkPoliciesCondition waits for NetworkPoliciesAvailable on an operand CR.
func WaitForOperandNetworkPoliciesCondition(ctx context.Context, k8sClient client.Client, kind string, name string,
	status metav1.ConditionStatus, reason string, timeout time.Duration) {
	Eventually(func() bool {
		cond, err := getOperandNetworkPoliciesCondition(ctx, k8sClient, kind, name)
		if err != nil {
			fmt.Fprintf(GinkgoWriter, "get condition: %v\n", err)
			return false
		}
		if cond == nil {
			return false
		}
		if cond.Status != status {
			return false
		}
		if reason != "" && cond.Reason != reason {
			return false
		}
		return true
	}).WithTimeout(timeout).WithPolling(ShortInterval).Should(BeTrue(),
		"%s %s NetworkPoliciesAvailable should be %s (reason %q) within %v", kind, name, status, reason, timeout)
}

func getOperandNetworkPoliciesCondition(ctx context.Context, k8sClient client.Client, kind, name string) (*metav1.Condition, error) {
	switch kind {
	case "SpireServer":
		cr := &operatorv1alpha1.SpireServer{}
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: name}, cr); err != nil {
			return nil, err
		}
		return findCondition(cr.Status.Conditions, operatorv1alpha1.NetworkPoliciesAvailable)
	case "SpireAgent":
		cr := &operatorv1alpha1.SpireAgent{}
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: name}, cr); err != nil {
			return nil, err
		}
		return findCondition(cr.Status.Conditions, operatorv1alpha1.NetworkPoliciesAvailable)
	case "SpireOIDCDiscoveryProvider":
		cr := &operatorv1alpha1.SpireOIDCDiscoveryProvider{}
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: name}, cr); err != nil {
			return nil, err
		}
		return findCondition(cr.Status.Conditions, operatorv1alpha1.NetworkPoliciesAvailable)
	default:
		return nil, fmt.Errorf("unsupported kind %q", kind)
	}
}

func findCondition(conditions []metav1.Condition, typ string) (*metav1.Condition, error) {
	for i := range conditions {
		if conditions[i].Type == typ {
			return &conditions[i], nil
		}
	}
	return nil, nil
}
