package utils

import (
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"

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
	case *appsv1.StatefulSet:
		typeSpecificResult = StatefulSetNeedsUpdate(existingTyped, desired.(*appsv1.StatefulSet))
	case *appsv1.Deployment:
		typeSpecificResult = DeploymentNeedsUpdate(existingTyped, desired.(*appsv1.Deployment))
	case *appsv1.DaemonSet:
		typeSpecificResult = DaemonSetNeedsUpdate(existingTyped, desired.(*appsv1.DaemonSet))
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

// stringSlicesEqual compares two string slices (order-independent)
func stringSlicesEqual(existing, desired []string) bool {
	if len(existing) != len(desired) {
		return false
	}

	// Create a set of existing strings for order-independent comparison
	existingSet := make(map[string]bool)
	for _, str := range existing {
		existingSet[str] = true
	}

	// Check all desired strings exist in existing
	for _, str := range desired {
		if !existingSet[str] {
			return false
		}
	}

	return true
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

// volumesEqual compares two volume slices for equality
func volumesEqual(fetched, desired []corev1.Volume) bool {
	if len(desired) == 0 && len(fetched) == 0 {
		return true
	}
	if len(desired) != len(fetched) {
		return false
	}

	// Create a map of fetched volumes by name for easier lookup
	fetchedMap := make(map[string]corev1.Volume)
	for _, v := range fetched {
		fetchedMap[v.Name] = v
	}

	// Check each desired volume exists and matches in fetched
	for _, desiredVol := range desired {
		fetchedVol, exists := fetchedMap[desiredVol.Name]
		if !exists {
			return false
		}

		// Compare volume sources
		// Check ConfigMap volume
		if desiredVol.ConfigMap != nil {
			if fetchedVol.ConfigMap == nil {
				return false
			}
			if desiredVol.ConfigMap.Name != fetchedVol.ConfigMap.Name {
				return false
			}
		}

		// Check Secret volume
		if desiredVol.Secret != nil {
			if fetchedVol.Secret == nil {
				return false
			}
			if desiredVol.Secret.SecretName != fetchedVol.Secret.SecretName {
				return false
			}
			if !equality.Semantic.DeepEqual(desiredVol.Secret.Items, fetchedVol.Secret.Items) {
				return false
			}
		}

		// Check EmptyDir volume
		if desiredVol.EmptyDir != nil {
			if fetchedVol.EmptyDir == nil {
				return false
			}
		}

		// Check HostPath volume
		if desiredVol.HostPath != nil {
			if fetchedVol.HostPath == nil {
				return false
			}
			if desiredVol.HostPath.Path != fetchedVol.HostPath.Path {
				return false
			}
		}

		// Check PersistentVolumeClaim volume
		if desiredVol.PersistentVolumeClaim != nil {
			if fetchedVol.PersistentVolumeClaim == nil {
				return false
			}
			if desiredVol.PersistentVolumeClaim.ClaimName != fetchedVol.PersistentVolumeClaim.ClaimName {
				return false
			}
		}

		// Check Projected volume
		if desiredVol.Projected != nil {
			if fetchedVol.Projected == nil {
				return false
			}
			if !equality.Semantic.DeepEqual(desiredVol.Projected, fetchedVol.Projected) {
				return false
			}
		}

		// Check CSI volume
		if desiredVol.CSI != nil {
			if fetchedVol.CSI == nil {
				return false
			}
			if !equality.Semantic.DeepEqual(desiredVol.CSI, fetchedVol.CSI) {
				return false
			}
		}
	}

	return true
}

// containerSpecModified checks if a container spec has been modified
func containerSpecModified(fetched, desired *corev1.Container) bool {
	// Check basic container properties
	if desired.Name != fetched.Name ||
		desired.Image != fetched.Image ||
		desired.ImagePullPolicy != fetched.ImagePullPolicy {
		return true
	}

	// Check args
	if !equality.Semantic.DeepEqual(desired.Args, fetched.Args) {
		return true
	}

	// Check environment variables
	if !equality.Semantic.DeepEqual(desired.Env, fetched.Env) {
		return true
	}

	// Check ports
	if len(desired.Ports) != len(fetched.Ports) {
		return true
	}
	fetchedByKey := make(map[string]corev1.ContainerPort, len(fetched.Ports))
	for _, port := range fetched.Ports {
		key := fmt.Sprintf("%d/%s", port.ContainerPort, port.Protocol)
		fetchedByKey[key] = port
	}
	for _, desiredPort := range desired.Ports {
		key := fmt.Sprintf("%d/%s", desiredPort.ContainerPort, desiredPort.Protocol)
		fetchedPort, ok := fetchedByKey[key]
		if !ok || !equality.Semantic.DeepEqual(desiredPort, fetchedPort) {
			return true
		}
	}

	// ReadinessProbe nil checks
	if (desired.ReadinessProbe == nil) != (fetched.ReadinessProbe == nil) {
		return true
	}
	if desired.ReadinessProbe != nil && fetched.ReadinessProbe != nil &&
		!equality.Semantic.DeepEqual(desired.ReadinessProbe.HTTPGet, fetched.ReadinessProbe.HTTPGet) {
		return true
	}

	// LivenessProbe nil checks
	if (desired.LivenessProbe == nil) != (fetched.LivenessProbe == nil) {
		return true
	}
	if desired.LivenessProbe != nil && fetched.LivenessProbe != nil &&
		!equality.Semantic.DeepEqual(desired.LivenessProbe.HTTPGet, fetched.LivenessProbe.HTTPGet) {
		return true
	}

	// SecurityContext checks
	if (desired.SecurityContext == nil) != (fetched.SecurityContext == nil) {
		return true
	}
	if desired.SecurityContext != nil && !equality.Semantic.DeepEqual(desired.SecurityContext, fetched.SecurityContext) {
		return true
	}

	// Check volume mounts
	if !equality.Semantic.DeepEqual(desired.VolumeMounts, fetched.VolumeMounts) {
		return true
	}

	// Check resources
	if !equality.Semantic.DeepEqual(desired.Resources, fetched.Resources) {
		return true
	}

	return false
}

// StatefulSetNeedsUpdate checks if a StatefulSet needs updating
func StatefulSetNeedsUpdate(fetched, desired *appsv1.StatefulSet) bool {
	if desired == nil || fetched == nil {
		return true
	}
	ds := desired.Spec
	fs := fetched.Spec
	if ds.Replicas != nil && fs.Replicas != nil && *ds.Replicas != *fs.Replicas {
		return true
	}
	if ds.ServiceName != fs.ServiceName {
		return true
	}

	if !equality.Semantic.DeepEqual(ds.Selector, fs.Selector) {
		return true
	}

	if !equality.Semantic.DeepEqual(ds.Template.Labels, fs.Template.Labels) {
		return true
	}

	for _, key := range []string{
		"kubectl.kubernetes.io/default-container",
		"ztwim.openshift.io/spire-server-config-hash",
		"ztwim.openshift.io/spire-controller-manager-config-hash",
	} {
		if ds.Template.Annotations[key] != fs.Template.Annotations[key] {
			return true
		}
	}
	dPod := ds.Template.Spec
	fPod := fs.Template.Spec
	if dPod.ServiceAccountName != fPod.ServiceAccountName {
		return true
	}
	if !ptr.Equal(dPod.ShareProcessNamespace, fPod.ShareProcessNamespace) {
		return true
	}
	// Check DNSPolicy
	if dPod.DNSPolicy != "" && dPod.DNSPolicy != fPod.DNSPolicy {
		return true
	}
	if len(dPod.NodeSelector) != len(fPod.NodeSelector) {
		return true
	}
	if len(dPod.NodeSelector) > 0 && !equality.Semantic.DeepEqual(dPod.NodeSelector, fPod.NodeSelector) {
		return true
	}
	if !equality.Semantic.DeepEqual(dPod.Affinity, fPod.Affinity) {
		return true
	}
	if len(dPod.Tolerations) != len(fPod.Tolerations) {
		return true
	}
	if len(dPod.Tolerations) > 0 && !equality.Semantic.DeepEqual(dPod.Tolerations, fPod.Tolerations) {
		return true
	}
	// Check volumes
	if !volumesEqual(fPod.Volumes, dPod.Volumes) {
		return true
	}
	// Check regular containers
	if len(dPod.Containers) != len(fPod.Containers) {
		return true
	}
	dMap := map[string]corev1.Container{}
	fMap := map[string]corev1.Container{}
	for _, c := range dPod.Containers {
		dMap[c.Name] = c
	}
	for _, c := range fPod.Containers {
		fMap[c.Name] = c
	}

	for name, dCont := range dMap {
		fCont, ok := fMap[name]
		if !ok {
			return true
		}
		if containerSpecModified(&fCont, &dCont) {
			return true
		}
	}

	// Check init containers
	if len(dPod.InitContainers) != len(fPod.InitContainers) {
		return true
	}
	dInitMap := map[string]corev1.Container{}
	fInitMap := map[string]corev1.Container{}
	for _, c := range dPod.InitContainers {
		dInitMap[c.Name] = c
	}
	for _, c := range fPod.InitContainers {
		fInitMap[c.Name] = c
	}
	for name, dInitCont := range dInitMap {
		fInitCont, ok := fInitMap[name]
		if !ok {
			return true
		}
		if containerSpecModified(&fInitCont, &dInitCont) {
			return true
		}
	}
	if len(ds.VolumeClaimTemplates) != len(fs.VolumeClaimTemplates) {
		return true
	}
	for i := range ds.VolumeClaimTemplates {
		dvc := ds.VolumeClaimTemplates[i]
		fvc := fs.VolumeClaimTemplates[i]
		if dvc.Name != fvc.Name {
			return true
		}
		if !equality.Semantic.DeepEqual(dvc.Spec.AccessModes, fvc.Spec.AccessModes) {
			return true
		}
		if !equality.Semantic.DeepEqual(dvc.Spec.Resources.Requests, fvc.Spec.Resources.Requests) {
			return true
		}
	}
	return false
}

// DeploymentNeedsUpdate checks if a Deployment needs updating
func DeploymentNeedsUpdate(fetched, desired *appsv1.Deployment) bool {
	if desired == nil || fetched == nil {
		return true
	}
	ds := desired.Spec
	fs := fetched.Spec
	if ds.Replicas != nil && fs.Replicas != nil && *ds.Replicas != *fs.Replicas {
		return true
	}
	if !equality.Semantic.DeepEqual(ds.Selector, fs.Selector) {
		return true
	}
	if !equality.Semantic.DeepEqual(ds.Template.Labels, fs.Template.Labels) {
		return true
	}
	dPod := ds.Template.Spec
	fPod := fs.Template.Spec
	if dPod.ServiceAccountName != fPod.ServiceAccountName {
		return true
	}
	if !ptr.Equal(dPod.ShareProcessNamespace, fPod.ShareProcessNamespace) {
		return true
	}
	// Check DNSPolicy
	if dPod.DNSPolicy != "" && dPod.DNSPolicy != fPod.DNSPolicy {
		return true
	}
	if len(dPod.NodeSelector) != len(fPod.NodeSelector) {
		return true
	}
	if len(dPod.NodeSelector) > 0 && !equality.Semantic.DeepEqual(dPod.NodeSelector, fPod.NodeSelector) {
		return true
	}
	if !equality.Semantic.DeepEqual(dPod.Affinity, fPod.Affinity) {
		return true
	}
	if len(dPod.Tolerations) != len(fPod.Tolerations) {
		return true
	}
	if len(dPod.Tolerations) > 0 && !equality.Semantic.DeepEqual(dPod.Tolerations, fPod.Tolerations) {
		return true
	}
	// Check volumes
	if !volumesEqual(fPod.Volumes, dPod.Volumes) {
		return true
	}
	// Check regular containers
	if len(dPod.Containers) != len(fPod.Containers) {
		return true
	}
	dMap := map[string]corev1.Container{}
	fMap := map[string]corev1.Container{}
	for _, c := range dPod.Containers {
		dMap[c.Name] = c
	}
	for _, c := range fPod.Containers {
		fMap[c.Name] = c
	}
	for name, dCont := range dMap {
		fCont, ok := fMap[name]
		if !ok {
			return true
		}
		if containerSpecModified(&fCont, &dCont) {
			return true
		}
	}
	// Check init containers
	if len(dPod.InitContainers) != len(fPod.InitContainers) {
		return true
	}
	dInitMap := map[string]corev1.Container{}
	fInitMap := map[string]corev1.Container{}
	for _, c := range dPod.InitContainers {
		dInitMap[c.Name] = c
	}
	for _, c := range fPod.InitContainers {
		fInitMap[c.Name] = c
	}
	for name, dInitCont := range dInitMap {
		fInitCont, ok := fInitMap[name]
		if !ok {
			return true
		}
		if containerSpecModified(&fInitCont, &dInitCont) {
			return true
		}
	}
	return false
}

// DaemonSetNeedsUpdate checks if a DaemonSet needs updating
func DaemonSetNeedsUpdate(fetched, desired *appsv1.DaemonSet) bool {
	if desired == nil || fetched == nil {
		return true
	}
	ds := desired.Spec
	fs := fetched.Spec
	if !equality.Semantic.DeepEqual(ds.Selector, fs.Selector) {
		return true
	}
	if !equality.Semantic.DeepEqual(ds.Template.Labels, fs.Template.Labels) {
		return true
	}
	dPod := ds.Template.Spec
	fPod := fs.Template.Spec
	if dPod.ServiceAccountName != fPod.ServiceAccountName {
		return true
	}
	if !ptr.Equal(dPod.ShareProcessNamespace, fPod.ShareProcessNamespace) {
		return true
	}
	// Check DNSPolicy
	if dPod.DNSPolicy != "" && dPod.DNSPolicy != fPod.DNSPolicy {
		return true
	}
	if len(dPod.NodeSelector) != len(fPod.NodeSelector) {
		return true
	}
	if len(dPod.NodeSelector) > 0 && !equality.Semantic.DeepEqual(dPod.NodeSelector, fPod.NodeSelector) {
		return true
	}
	if !equality.Semantic.DeepEqual(dPod.Affinity, fPod.Affinity) {
		return true
	}
	if len(dPod.Tolerations) != len(fPod.Tolerations) {
		return true
	}
	if len(dPod.Tolerations) > 0 && !equality.Semantic.DeepEqual(dPod.Tolerations, fPod.Tolerations) {
		return true
	}
	// Check volumes
	if !volumesEqual(fPod.Volumes, dPod.Volumes) {
		return true
	}
	// Check regular containers
	if len(dPod.Containers) != len(fPod.Containers) {
		return true
	}
	dMap := map[string]corev1.Container{}
	fMap := map[string]corev1.Container{}
	for _, c := range dPod.Containers {
		dMap[c.Name] = c
	}
	for _, c := range fPod.Containers {
		fMap[c.Name] = c
	}
	for name, dCont := range dMap {
		fCont, ok := fMap[name]
		if !ok {
			return true
		}
		if containerSpecModified(&fCont, &dCont) {
			return true
		}
	}
	// Check init containers
	if len(dPod.InitContainers) != len(fPod.InitContainers) {
		return true
	}
	dInitMap := map[string]corev1.Container{}
	fInitMap := map[string]corev1.Container{}
	for _, c := range dPod.InitContainers {
		dInitMap[c.Name] = c
	}
	for _, c := range fPod.InitContainers {
		fInitMap[c.Name] = c
	}
	for name, dInitCont := range dInitMap {
		fInitCont, ok := fInitMap[name]
		if !ok {
			return true
		}
		if containerSpecModified(&fInitCont, &dInitCont) {
			return true
		}
	}
	return false
}
