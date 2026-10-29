package utils

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// NetworkPolicyClient is the subset of client operations needed for NetworkPolicy reconcile.
type NetworkPolicyClient interface {
	Get(ctx context.Context, key client.ObjectKey, obj client.Object) error
	Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
	Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
	Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error
}

// NetworkPolicyApplyResult summarizes a single NetworkPolicy reconcile.
type NetworkPolicyApplyResult struct {
	Name    string
	Created bool
	Updated bool
}

// ApplyNetworkPolicy creates or updates desired when shouldApply is true; otherwise deletes by name.
func ApplyNetworkPolicy(
	ctx context.Context,
	cli NetworkPolicyClient,
	scheme *runtime.Scheme,
	owner metav1.Object,
	namespace, name string,
	shouldApply bool,
	buildDesired func() *networkingv1.NetworkPolicy,
	createOnlyMode bool,
) (NetworkPolicyApplyResult, error) {
	if !shouldApply {
		if err := DeleteNetworkPolicyIfExists(ctx, cli, namespace, name); err != nil {
			return NetworkPolicyApplyResult{}, err
		}
		return NetworkPolicyApplyResult{Name: name}, nil
	}

	desired := buildDesired()
	if desired.Namespace == "" {
		desired.Namespace = namespace
	}
	if desired.Name == "" {
		desired.Name = name
	}

	if err := controllerutil.SetControllerReference(owner, desired, scheme); err != nil {
		return NetworkPolicyApplyResult{}, err
	}

	existing := &networkingv1.NetworkPolicy{}
	err := cli.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return NetworkPolicyApplyResult{}, err
		}
		if err := cli.Create(ctx, desired); err != nil {
			return NetworkPolicyApplyResult{}, err
		}
		return NetworkPolicyApplyResult{Name: desired.Name, Created: true}, nil
	}

	if createOnlyMode {
		return NetworkPolicyApplyResult{Name: desired.Name}, nil
	}

	if ResourceNeedsUpdate(existing, desired) {
		desired.ResourceVersion = existing.ResourceVersion
		if err := cli.Update(ctx, desired); err != nil {
			return NetworkPolicyApplyResult{}, err
		}
		return NetworkPolicyApplyResult{Name: desired.Name, Updated: true}, nil
	}
	return NetworkPolicyApplyResult{Name: desired.Name}, nil
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

// DeleteNetworkPolicyIfExists removes a NetworkPolicy when present.
func DeleteNetworkPolicyIfExists(ctx context.Context, cli NetworkPolicyClient, namespace, name string) error {
	existing := &networkingv1.NetworkPolicy{}
	err := cli.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, existing)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return cli.Delete(ctx, existing)
}
