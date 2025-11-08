package utils

import (
	"reflect"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResourceNeedsUpdate determines if a resource needs to be updated based on its type
// This checks labels, annotations, and type-specific fields
func ResourceNeedsUpdate(existing, desired client.Object) bool {
	// Compare labels - only check if desired labels are present and match
	existingLabels := existing.GetLabels()
	desiredLabels := desired.GetLabels()
	if !labelsMatch(existingLabels, desiredLabels) {
		return true
	}

	// Compare annotations - only check if desired annotations are present and match
	existingAnnotations := existing.GetAnnotations()
	desiredAnnotations := desired.GetAnnotations()
	if !annotationsMatch(existingAnnotations, desiredAnnotations) {
		return true
	}

	// Type-specific comparison
	var typeSpecificResult bool
	switch existingTyped := existing.(type) {
	case *corev1.Service:
		typeSpecificResult = ServiceNeedsUpdate(existingTyped, desired.(*corev1.Service))
	case *corev1.ServiceAccount:
		typeSpecificResult = ServiceAccountNeedsUpdate(existingTyped, desired.(*corev1.ServiceAccount))
	case *rbacv1.ClusterRole:
		typeSpecificResult = ClusterRoleNeedsUpdate(existingTyped, desired.(*rbacv1.ClusterRole))
	case *rbacv1.ClusterRoleBinding:
		typeSpecificResult = ClusterRoleBindingNeedsUpdate(existingTyped, desired.(*rbacv1.ClusterRoleBinding))
	case *rbacv1.Role:
		typeSpecificResult = RoleNeedsUpdate(existingTyped, desired.(*rbacv1.Role))
	case *rbacv1.RoleBinding:
		typeSpecificResult = RoleBindingNeedsUpdate(existingTyped, desired.(*rbacv1.RoleBinding))
	case *storagev1.CSIDriver:
		typeSpecificResult = CSIDriverNeedsUpdate(existingTyped, desired.(*storagev1.CSIDriver))
	case *admissionregistrationv1.ValidatingWebhookConfiguration:
		typeSpecificResult = ValidatingWebhookConfigurationNeedsUpdate(existingTyped, desired.(*admissionregistrationv1.ValidatingWebhookConfiguration))
	default:
		// For unknown types, just compare labels and annotations (already done above)
		typeSpecificResult = false
	}
	return typeSpecificResult
}

// labelsMatch checks if all desired labels are present in existing with the same values
// We don't care about extra labels that Kubernetes might add
// Treats nil and empty maps as equivalent
func labelsMatch(existing, desired map[string]string) bool {
	// If desired is nil or empty, we're not enforcing any labels
	if desired == nil || len(desired) == 0 {
		return true
	}

	// Check all desired labels exist in existing with same values
	for key, desiredValue := range desired {
		existingValue, exists := existing[key]
		if !exists || existingValue != desiredValue {
			return false
		}
	}
	return true
}

// annotationsMatch checks if all desired annotations are present in existing with the same values
// We don't care about extra annotations that Kubernetes might add
// Treats nil and empty maps as equivalent
func annotationsMatch(existing, desired map[string]string) bool {
	// If desired is nil or empty, we're not enforcing any annotations
	if desired == nil || len(desired) == 0 {
		return true
	}

	// Check all desired annotations exist in existing with same values
	for key, desiredValue := range desired {
		existingValue, exists := existing[key]
		if !exists || existingValue != desiredValue {
			return false
		}
	}
	return true
}

// ServiceNeedsUpdate checks if a Service needs updating
func ServiceNeedsUpdate(existing, desired *corev1.Service) bool {
	if existing.Spec.Type != desired.Spec.Type ||
		!reflect.DeepEqual(existing.Spec.Ports, desired.Spec.Ports) ||
		!reflect.DeepEqual(existing.Spec.Selector, desired.Spec.Selector) {
		return true
	}
	return false
}

// mapsEqual compares two string maps
// Only checks that all desired keys exist in existing with same values
// Allows Kubernetes to add extra keys
func mapsEqual(existing, desired map[string]string) bool {
	for key, desiredValue := range desired {
		existingValue, exists := existing[key]
		if !exists || existingValue != desiredValue {
			return false
		}
	}

	return true
}

// ServiceAccountNeedsUpdate checks if a ServiceAccount needs updating
func ServiceAccountNeedsUpdate(existing, desired *corev1.ServiceAccount) bool {
	// Compare automount service account token setting only if desired has it set
	if desired.AutomountServiceAccountToken != nil {
		if !boolPtrsEqual(existing.AutomountServiceAccountToken, desired.AutomountServiceAccountToken) {
			return true
		}
	}

	// Compare image pull secrets only if desired has them
	if len(desired.ImagePullSecrets) > 0 {
		if !localObjectReferencesEqual(existing.ImagePullSecrets, desired.ImagePullSecrets) {
			return true
		}
	}

	return false
}

// boolPtrsEqual compares two boolean pointers
func boolPtrsEqual(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// localObjectReferencesEqual compares two slices of LocalObjectReference
func localObjectReferencesEqual(existing, desired []corev1.LocalObjectReference) bool {
	if len(existing) != len(desired) {
		return false
	}

	existingMap := make(map[string]bool)
	for _, ref := range existing {
		existingMap[ref.Name] = true
	}

	for _, ref := range desired {
		if !existingMap[ref.Name] {
			return false
		}
	}

	return true
}

// ClusterRoleNeedsUpdate checks if a ClusterRole needs updating
func ClusterRoleNeedsUpdate(existing, desired *rbacv1.ClusterRole) bool {
	// Compare rules
	if !policyRulesEqual(existing.Rules, desired.Rules) {
		return true
	}
	// Compare aggregation rule
	if !aggregationRulesEqual(existing.AggregationRule, desired.AggregationRule) {
		return true
	}
	return false
}

// policyRulesEqual compares two slices of PolicyRule
func policyRulesEqual(existing, desired []rbacv1.PolicyRule) bool {
	if len(existing) != len(desired) {
		return false
	}
	return reflect.DeepEqual(existing, desired)
}

// aggregationRulesEqual compares two AggregationRule pointers
func aggregationRulesEqual(existing, desired *rbacv1.AggregationRule) bool {
	if existing == nil && desired == nil {
		return true
	}
	if existing == nil || desired == nil {
		return false
	}
	return reflect.DeepEqual(existing, desired)
}

// ClusterRoleBindingNeedsUpdate checks if a ClusterRoleBinding needs updating
func ClusterRoleBindingNeedsUpdate(existing, desired *rbacv1.ClusterRoleBinding) bool {
	// Compare subjects
	if !subjectsEqual(existing.Subjects, desired.Subjects) {
		return true
	}
	return false
}

// subjectsEqual compares two slices of Subject
func subjectsEqual(existing, desired []rbacv1.Subject) bool {
	if len(existing) != len(desired) {
		return false
	}
	return reflect.DeepEqual(existing, desired)
}

// RoleNeedsUpdate checks if a Role needs updating
func RoleNeedsUpdate(existing, desired *rbacv1.Role) bool {
	// Compare rules
	return !policyRulesEqual(existing.Rules, desired.Rules)
}

// RoleBindingNeedsUpdate checks if a RoleBinding needs updating
func RoleBindingNeedsUpdate(existing, desired *rbacv1.RoleBinding) bool {
	if !subjectsEqual(existing.Subjects, desired.Subjects) {
		return true
	}
	return false
}

// CSIDriverNeedsUpdate checks if a CSIDriver needs updating
func CSIDriverNeedsUpdate(existing, desired *storagev1.CSIDriver) bool {
	//if existing.Spec.AttachRequired != desired.Spec.AttachRequired {
	//	return true
	//}
	//if existing.Spec.PodInfoOnMount != desired.Spec.PodInfoOnMount {
	//	return true
	//}
	//if existing.Spec.FSGroupPolicy != desired.Spec.FSGroupPolicy {
	//	return true
	//}
	//if !reflect.DeepEqual(existing.Spec.VolumeLifecycleModes, desired.Spec.VolumeLifecycleModes) {
	//	return true
	//}
	//return false
	return false
}

// ValidatingWebhookConfigurationNeedsUpdate checks if a ValidatingWebhookConfiguration needs updating
func ValidatingWebhookConfigurationNeedsUpdate(existing, desired *admissionregistrationv1.ValidatingWebhookConfiguration) bool {
	// TODO: Add logic
	return false
}
