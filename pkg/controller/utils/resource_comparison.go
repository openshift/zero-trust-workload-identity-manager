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
	// Compare selector
	if !mapsEqual(existing.Spec.Selector, desired.Spec.Selector) {
		return true
	}

	// Compare ports
	if !portsEqual(existing.Spec.Ports, desired.Spec.Ports) {
		return true
	}

	// Compare service type
	if existing.Spec.Type != desired.Spec.Type {
		return true
	}

	// Compare session affinity only if desired sets it
	if desired.Spec.SessionAffinity != "" && existing.Spec.SessionAffinity != desired.Spec.SessionAffinity {
		return true
	}

	return false
}

// portsEqual compares two slices of ServicePort
func portsEqual(existing, desired []corev1.ServicePort) bool {
	if len(existing) != len(desired) {
		return false
	}

	// Create maps for easier comparison
	existingMap := make(map[string]corev1.ServicePort)
	for _, port := range existing {
		existingMap[port.Name] = port
	}

	for _, desiredPort := range desired {
		existingPort, exists := existingMap[desiredPort.Name]
		if !exists {
			return false
		}

		// Compare key fields (ignore NodePort as it's assigned by Kubernetes)
		if existingPort.Protocol != desiredPort.Protocol ||
			existingPort.Port != desiredPort.Port ||
			existingPort.TargetPort != desiredPort.TargetPort {
			return false
		}

		// Only compare AppProtocol if desired has it set
		if desiredPort.AppProtocol != nil && existingPort.AppProtocol != nil {
			if *existingPort.AppProtocol != *desiredPort.AppProtocol {
				return false
			}
		}
	}

	return true
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

	// Don't compare Secrets field as it's auto-populated by Kubernetes

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

	// For simplicity, we compare the entire rule slice using DeepEqual
	// This is safe because PolicyRules don't have Kubernetes-added fields
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
		return false // Note: RoleRef is immutable, we don't update if only subjects differ
	}

	// RoleRef is immutable - if it differs, the resource needs to be recreated, not updated
	// We return false here because update won't work anyway

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
	// Compare subjects
	if !subjectsEqual(existing.Subjects, desired.Subjects) {
		return false // Note: RoleRef is immutable
	}

	// RoleRef is immutable
	return false
}

// CSIDriverNeedsUpdate checks if a CSIDriver needs updating
func CSIDriverNeedsUpdate(existing, desired *storagev1.CSIDriver) bool {
	if existing.Spec.AttachRequired != desired.Spec.AttachRequired {
		return true
	}
	if existing.Spec.PodInfoOnMount != desired.Spec.PodInfoOnMount {
		return true
	}
	if existing.Spec.FSGroupPolicy != desired.Spec.FSGroupPolicy {
		return true
	}
	if !reflect.DeepEqual(existing.Spec.VolumeLifecycleModes, desired.Spec.VolumeLifecycleModes) {
		return true
	}
	return false

}

// ValidatingWebhookConfigurationNeedsUpdate checks if a ValidatingWebhookConfiguration needs updating
func ValidatingWebhookConfigurationNeedsUpdate(existing, desired *admissionregistrationv1.ValidatingWebhookConfiguration) bool {
	// Compare webhooks array
	if len(existing.Webhooks) != len(desired.Webhooks) {
		return true
	}

	// Create a map for easier comparison by name
	existingMap := make(map[string]admissionregistrationv1.ValidatingWebhook)
	for _, webhook := range existing.Webhooks {
		existingMap[webhook.Name] = webhook
	}

	for _, desiredWebhook := range desired.Webhooks {
		existingWebhook, exists := existingMap[desiredWebhook.Name]
		if !exists {
			return true
		}

		// Compare ClientConfig carefully - Kubernetes adds CABundle automatically
		if !webhookClientConfigEqual(existingWebhook.ClientConfig, desiredWebhook.ClientConfig) {
			return true
		}

		if !reflect.DeepEqual(existingWebhook.Rules, desiredWebhook.Rules) {
			return true
		}
		if !reflect.DeepEqual(existingWebhook.NamespaceSelector, desiredWebhook.NamespaceSelector) {
			return true
		}
		if !reflect.DeepEqual(existingWebhook.ObjectSelector, desiredWebhook.ObjectSelector) {
			return true
		}
		if !reflect.DeepEqual(existingWebhook.AdmissionReviewVersions, desiredWebhook.AdmissionReviewVersions) {
			return true
		}
		if !reflect.DeepEqual(existingWebhook.SideEffects, desiredWebhook.SideEffects) {
			return true
		}
	}

	return false
}

// webhookClientConfigEqual compares webhook ClientConfig
// Ignores CABundle as it's injected by Kubernetes
// Returns true if configs are equal, false if they differ
func webhookClientConfigEqual(existing, desired admissionregistrationv1.WebhookClientConfig) bool {
	// Compare Service reference
	if !reflect.DeepEqual(existing.Service, desired.Service) {
		return false // Different
	}

	// Compare URL if set
	if desired.URL != nil && existing.URL != nil {
		if *existing.URL != *desired.URL {
			return false // Different
		}
	} else if (desired.URL == nil) != (existing.URL == nil) {
		// One is nil, other is not
		return false // Different
	}

	// Don't compare CABundle - it's injected by cert-manager or Kubernetes

	return true // Equal
}

// PreserveServiceImmutableFields preserves immutable and Kubernetes-managed fields on Service
// This should be called BEFORE comparison to avoid false positives
func PreserveServiceImmutableFields(existing, desired *corev1.Service) {
	// Preserve ClusterIP and ClusterIPs as they are immutable
	desired.Spec.ClusterIP = existing.Spec.ClusterIP
	desired.Spec.ClusterIPs = existing.Spec.ClusterIPs

	// Preserve HealthCheckNodePort for LoadBalancer/NodePort services
	if desired.Spec.Type == corev1.ServiceTypeLoadBalancer || desired.Spec.Type == corev1.ServiceTypeNodePort {
		desired.Spec.HealthCheckNodePort = existing.Spec.HealthCheckNodePort
	}

	// Preserve IPFamilies and IPFamilyPolicy if they were set
	if len(existing.Spec.IPFamilies) > 0 {
		desired.Spec.IPFamilies = existing.Spec.IPFamilies
	}
	if existing.Spec.IPFamilyPolicy != nil {
		desired.Spec.IPFamilyPolicy = existing.Spec.IPFamilyPolicy
	}

	// Preserve InternalTrafficPolicy - Kubernetes sets this to default value
	if desired.Spec.InternalTrafficPolicy == nil && existing.Spec.InternalTrafficPolicy != nil {
		desired.Spec.InternalTrafficPolicy = existing.Spec.InternalTrafficPolicy
	}

	// Preserve ExternalTrafficPolicy for LoadBalancer/NodePort
	if desired.Spec.Type == corev1.ServiceTypeLoadBalancer || desired.Spec.Type == corev1.ServiceTypeNodePort {
		if desired.Spec.ExternalTrafficPolicy == "" && existing.Spec.ExternalTrafficPolicy != "" {
			desired.Spec.ExternalTrafficPolicy = existing.Spec.ExternalTrafficPolicy
		}
	}

	// Preserve SessionAffinity if not set in desired
	if desired.Spec.SessionAffinity == "" && existing.Spec.SessionAffinity != "" {
		desired.Spec.SessionAffinity = existing.Spec.SessionAffinity
	}
}

// PreserveValidatingWebhookImmutableFields preserves Kubernetes-managed fields on ValidatingWebhookConfiguration
// This should be called BEFORE comparison to avoid false positives
func PreserveValidatingWebhookImmutableFields(existing, desired *admissionregistrationv1.ValidatingWebhookConfiguration) {
	// Preserve CABundle that's injected by cert-manager or Kubernetes
	if len(existing.Webhooks) == len(desired.Webhooks) {
		for i := range desired.Webhooks {
			// Match by name
			for j := range existing.Webhooks {
				if existing.Webhooks[j].Name == desired.Webhooks[i].Name {
					// Preserve CABundle
					if len(existing.Webhooks[j].ClientConfig.CABundle) > 0 {
						desired.Webhooks[i].ClientConfig.CABundle = existing.Webhooks[j].ClientConfig.CABundle
					}
					break
				}
			}
		}
	}
}
