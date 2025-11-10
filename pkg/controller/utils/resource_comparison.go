package utils

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"

	"k8s.io/apimachinery/pkg/api/equality"

	"sigs.k8s.io/controller-runtime/pkg/client"

	securityv1 "github.com/openshift/api/security/v1"
	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
)

// ResourceNeedsUpdate determines if a resource needs to be updated based on its type
// This checks labels, annotations, and type-specific fields
func ResourceNeedsUpdate(existing, desired client.Object) bool {
	// Compare labels - only check if desired labels are present and match
	existingLabels := existing.GetLabels()
	desiredLabels := desired.GetLabels()
	if !LabelsMatch(existingLabels, desiredLabels) {
		return true
	}

	// Compare annotations - only check if desired annotations are present and match
	existingAnnotations := existing.GetAnnotations()
	desiredAnnotations := desired.GetAnnotations()
	if !AnnotationsMatch(existingAnnotations, desiredAnnotations) {
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
	case *securityv1.SecurityContextConstraints:
		typeSpecificResult = SecurityContextConstraintsNeedsUpdate(existingTyped, desired.(*securityv1.SecurityContextConstraints))
	case *spiffev1alpha1.ClusterSPIFFEID:
		typeSpecificResult = ClusterSPIFFEIDNeedsUpdate(existingTyped, desired.(*spiffev1alpha1.ClusterSPIFFEID))
	default:
		// For unknown types, just compare labels and annotations (already done above)
		typeSpecificResult = false
	}
	return typeSpecificResult
}

// LabelsMatch checks if all desired labels are present in existing with the same values
// We don't care about extra labels that Kubernetes might add
// Treats nil and empty maps as equivalent
func LabelsMatch(existing, desired map[string]string) bool {
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

// AnnotationsMatch checks if all desired annotations are present in existing with the same values
// We don't care about extra annotations that Kubernetes might add
// Treats nil and empty maps as equivalent
func AnnotationsMatch(existing, desired map[string]string) bool {
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
	// Don't compare ClusterIP - it's immutable and set by K8s
	// Don't compare ClusterIPs - they're immutable and set by K8s
	// Don't compare healthCheckNodePort - it's set by K8s for LoadBalancer services

	if existing.Spec.Type != desired.Spec.Type {
		return true
	}
	if !equality.Semantic.DeepEqual(existing.Spec.Ports, desired.Spec.Ports) {
		return true
	}
	if !equality.Semantic.DeepEqual(existing.Spec.Selector, desired.Spec.Selector) {
		return true
	}
	return false
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
	return equality.Semantic.DeepEqual(existing, desired)
}

// aggregationRulesEqual compares two AggregationRule pointers
func aggregationRulesEqual(existing, desired *rbacv1.AggregationRule) bool {
	if existing == nil && desired == nil {
		return true
	}
	if existing == nil || desired == nil {
		return false
	}
	return equality.Semantic.DeepEqual(existing, desired)
}

// ClusterRoleBindingNeedsUpdate checks if a ClusterRoleBinding needs updating
func ClusterRoleBindingNeedsUpdate(existing, desired *rbacv1.ClusterRoleBinding) bool {
	// Compare subjects
	if !subjectsEqual(existing.Subjects, desired.Subjects) {
		return true
	}
	if !equality.Semantic.DeepEqual(existing.RoleRef, desired.RoleRef) {
		return true
	}
	return false
}

// subjectsEqual compares two slices of Subject
func subjectsEqual(existing, desired []rbacv1.Subject) bool {
	if len(existing) != len(desired) {
		return false
	}
	return equality.Semantic.DeepEqual(existing, desired)
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
	if !equality.Semantic.DeepEqual(existing.RoleRef, desired.RoleRef) {
		return true
	}
	return false
}

// CSIDriverNeedsUpdate checks if a CSIDriver needs updating
func CSIDriverNeedsUpdate(existing, desired *storagev1.CSIDriver) bool {
	// AttachRequired and PodInfoOnMount are pointers, need proper comparison
	if !boolPtrsEqual(existing.Spec.AttachRequired, desired.Spec.AttachRequired) {
		return true
	}
	if !boolPtrsEqual(existing.Spec.PodInfoOnMount, desired.Spec.PodInfoOnMount) {
		return true
	}
	// FSGroupPolicy is also a pointer
	if !fsGroupPolicyPtrsEqual(existing.Spec.FSGroupPolicy, desired.Spec.FSGroupPolicy) {
		return true
	}
	if !equality.Semantic.DeepEqual(existing.Spec.VolumeLifecycleModes, desired.Spec.VolumeLifecycleModes) {
		return true
	}
	// Compare TokenRequests if present
	if !equality.Semantic.DeepEqual(existing.Spec.TokenRequests, desired.Spec.TokenRequests) {
		return true
	}
	return false
}

// fsGroupPolicyPtrsEqual compares two FSGroupPolicy pointers
func fsGroupPolicyPtrsEqual(a, b *storagev1.FSGroupPolicy) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// ValidatingWebhookConfigurationNeedsUpdate checks if a ValidatingWebhookConfiguration needs updating
func ValidatingWebhookConfigurationNeedsUpdate(existing, desired *admissionregistrationv1.ValidatingWebhookConfiguration) bool {
	// Compare webhooks - the main content of the configuration
	if !equality.Semantic.DeepEqual(existing.Webhooks, desired.Webhooks) {
		return true
	}
	return false
}

// SecurityContextConstraintsNeedsUpdate checks if a SecurityContextConstraints needs updating
func SecurityContextConstraintsNeedsUpdate(existing, desired *securityv1.SecurityContextConstraints) bool {
	// Compare Users
	if !stringSlicesEqual(existing.Users, desired.Users) {
		return true
	}
	// Compare Groups
	if !stringSlicesEqual(existing.Groups, desired.Groups) {
		return true
	}
	// Compare Volumes
	if !fsTypeSlicesEqual(existing.Volumes, desired.Volumes) {
		return true
	}
	// Compare AllowHost* flags
	if existing.AllowHostDirVolumePlugin != desired.AllowHostDirVolumePlugin ||
		existing.AllowHostIPC != desired.AllowHostIPC ||
		existing.AllowHostNetwork != desired.AllowHostNetwork ||
		existing.AllowHostPID != desired.AllowHostPID ||
		existing.AllowHostPorts != desired.AllowHostPorts ||
		existing.AllowPrivilegedContainer != desired.AllowPrivilegedContainer ||
		existing.ReadOnlyRootFilesystem != desired.ReadOnlyRootFilesystem {
		return true
	}
	// Compare AllowPrivilegeEscalation
	if !boolPtrsEqual(existing.AllowPrivilegeEscalation, desired.AllowPrivilegeEscalation) {
		return true
	}
	// Compare strategy options
	if existing.RunAsUser.Type != desired.RunAsUser.Type ||
		existing.SELinuxContext.Type != desired.SELinuxContext.Type ||
		existing.SupplementalGroups.Type != desired.SupplementalGroups.Type ||
		existing.FSGroup.Type != desired.FSGroup.Type {
		return true
	}
	// Compare capabilities
	if !capabilitySlicesEqual(existing.AllowedCapabilities, desired.AllowedCapabilities) ||
		!capabilitySlicesEqual(existing.DefaultAddCapabilities, desired.DefaultAddCapabilities) ||
		!capabilitySlicesEqual(existing.RequiredDropCapabilities, desired.RequiredDropCapabilities) {
		return true
	}
	return false
}

// stringSlicesEqual compares two string slices
func stringSlicesEqual(existing, desired []string) bool {
	if len(existing) != len(desired) {
		return false
	}
	return equality.Semantic.DeepEqual(existing, desired)
}

// fsTypeSlicesEqual compares two FSType slices (order-independent)
func fsTypeSlicesEqual(existing, desired []securityv1.FSType) bool {
	if len(existing) != len(desired) {
		return false
	}

	// Create a set of existing volumes for order-independent comparison
	existingSet := make(map[securityv1.FSType]bool)
	for _, vol := range existing {
		existingSet[vol] = true
	}

	// Check all desired volumes exist in existing
	for _, vol := range desired {
		if !existingSet[vol] {
			return false
		}
	}

	return true
}

// capabilitySlicesEqual compares two Capability slices
func capabilitySlicesEqual(existing, desired []corev1.Capability) bool {
	if len(existing) != len(desired) {
		return false
	}
	return equality.Semantic.DeepEqual(existing, desired)
}

// ClusterSPIFFEIDNeedsUpdate checks if a ClusterSPIFFEID needs updating
func ClusterSPIFFEIDNeedsUpdate(existing, desired *spiffev1alpha1.ClusterSPIFFEID) bool {
	// Compare Spec fields
	if existing.Spec.ClassName != desired.Spec.ClassName ||
		existing.Spec.Hint != desired.Spec.Hint ||
		existing.Spec.SPIFFEIDTemplate != desired.Spec.SPIFFEIDTemplate ||
		existing.Spec.Fallback != desired.Spec.Fallback ||
		existing.Spec.AutoPopulateDNSNames != desired.Spec.AutoPopulateDNSNames {
		return true
	}
	// Compare DNS name templates
	if !stringSlicesEqual(existing.Spec.DNSNameTemplates, desired.Spec.DNSNameTemplates) {
		return true
	}
	// Compare selectors using Semantic.DeepEqual for Kubernetes types
	if !equality.Semantic.DeepEqual(existing.Spec.PodSelector, desired.Spec.PodSelector) ||
		!equality.Semantic.DeepEqual(existing.Spec.NamespaceSelector, desired.Spec.NamespaceSelector) ||
		!equality.Semantic.DeepEqual(existing.Spec.WorkloadSelectorTemplates, desired.Spec.WorkloadSelectorTemplates) {
		return true
	}
	return false
}
