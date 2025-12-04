package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sort"
	"strings"

	routev1 "github.com/openshift/api/route/v1"
	securityv1 "github.com/openshift/api/security/v1"
	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	scheme = runtime.NewScheme()
	codecs serializer.CodecFactory
)

const (
	createOnlyEnvName        = "CREATE_ONLY_MODE"
	CreateOnlyModeStatusType = "CreateOnlyMode"
	CreateOnlyModeEnabled    = "CreateOnlyModeEnabled"
	CreateOnlyModeDisabled   = "CreateOnlyModeDisabled"
)

func init() {
	// Register core, storage and rbac schemes
	_ = corev1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = admissionregistrationv1.AddToScheme(scheme)
	_ = securityv1.AddToScheme(scheme)
	_ = routev1.AddToScheme(scheme)
	_ = spiffev1alpha1.AddToScheme(scheme)

	// Create a codec factory for this scheme
	codecs = serializer.NewCodecFactory(scheme)
}

const (
	LogLevelInfo  = "info"
	LogFormatText = "text"
)

// GetOperatorNamespace returns the namespace where the operator resources should be installed.
// It reads from the OPERATOR_NAMESPACE environment variable.
// Returns an empty string if the environment variable is not set.
func GetOperatorNamespace() string {
	return os.Getenv("OPERATOR_NAMESPACE")
}

func DecodeClusterRoleObjBytes(objBytes []byte) *rbacv1.ClusterRole {
	obj, err := runtime.Decode(codecs.UniversalDecoder(rbacv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*rbacv1.ClusterRole)
}

func DecodeClusterRoleBindingObjBytes(objBytes []byte) *rbacv1.ClusterRoleBinding {
	obj, err := runtime.Decode(codecs.UniversalDecoder(rbacv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*rbacv1.ClusterRoleBinding)
}

func DecodeRoleObjBytes(objBytes []byte) *rbacv1.Role {
	obj, err := runtime.Decode(codecs.UniversalDecoder(rbacv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*rbacv1.Role)
}

func DecodeRoleBindingObjBytes(objBytes []byte) *rbacv1.RoleBinding {
	obj, err := runtime.Decode(codecs.UniversalDecoder(rbacv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*rbacv1.RoleBinding)
}

func DecodeServiceObjBytes(objBytes []byte) *corev1.Service {
	obj, err := runtime.Decode(codecs.UniversalDecoder(corev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*corev1.Service)
}

func DecodeServiceAccountObjBytes(objBytes []byte) *corev1.ServiceAccount {
	obj, err := runtime.Decode(codecs.UniversalDecoder(corev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*corev1.ServiceAccount)
}

func DecodeCsiDriverObjBytes(objBytes []byte) *storagev1.CSIDriver {
	obj, err := runtime.Decode(codecs.UniversalDecoder(storagev1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*storagev1.CSIDriver)
}

func DecodeValidatingWebhookConfigurationByBytes(objBytes []byte) *admissionregistrationv1.ValidatingWebhookConfiguration {
	obj, err := runtime.Decode(codecs.UniversalDecoder(admissionregistrationv1.SchemeGroupVersion), objBytes)
	if err != nil {
		panic(err)
	}
	return obj.(*admissionregistrationv1.ValidatingWebhookConfiguration)
}

// SetLabel sets a label key/value on the given object metadata labels map.
// If the labels map is nil, it initializes it.
func SetLabel(labels map[string]string, key, value string) map[string]string {
	if labels == nil {
		labels = map[string]string{}
	}
	labels[key] = value
	return labels
}

// GenerateConfigHashFromString returns a SHA256 hex string of the trimmed input string
func GenerateConfigHashFromString(data string) string {
	normalized := strings.TrimSpace(data) // Removes leading/trailing whitespace and newlines
	return GenerateConfigHash([]byte(normalized))
}

// GenerateConfigHash returns a SHA256 hex string of the trimmed input bytes
func GenerateConfigHash(data []byte) string {
	normalized := strings.TrimSpace(string(data)) // Convert to string, trim, convert back to bytes
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])
}

// GenerateMapHash takes a map[string]string, sorts it by key, and returns a SHA256 hash.
func GenerateMapHash(m map[string]string) string {
	var builder strings.Builder

	// Extract and sort the keys
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Concatenate keys and values in sorted order
	for _, k := range keys {
		builder.WriteString(strings.TrimSpace(k))
		builder.WriteString("=")
		builder.WriteString(strings.TrimSpace(m[k]))
		builder.WriteString(";") // Separator (optional but recommended for clarity)
	}

	// Compute the hash
	hash := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(hash[:])
}

func StringToBool(s string) bool {
	return s == "true"
}

func DerefResourceRequirements(r *corev1.ResourceRequirements) corev1.ResourceRequirements {
	if r != nil {
		return *r
	}
	return corev1.ResourceRequirements{}
}

func DerefAffinity(a *corev1.Affinity) corev1.Affinity {
	if a != nil {
		return *a
	}
	return corev1.Affinity{}
}

func DerefTolerations(tolerations []*corev1.Toleration) []corev1.Toleration {
	result := []corev1.Toleration{}
	for _, t := range tolerations {
		if t != nil {
			result = append(result, *t)
		}
	}
	return result
}

func DerefNodeSelector(selector map[string]string) map[string]string {
	if selector == nil {
		return map[string]string{}
	}
	result := make(map[string]string, len(selector))
	for k, v := range selector {
		result[k] = v
	}
	return result
}

func GetLogLevelFromString(logLevel string) string {
	if logLevel == "" {
		return LogLevelInfo
	}
	return logLevel
}

func GetLogFormatFromString(logFormat string) string {
	if logFormat == "" {
		return LogFormatText
	}
	return logFormat
}

// IsInCreateOnlyMode checks if create-only mode is enabled
// If the environment variable is set to "true", it returns true
// Otherwise, it returns false
func IsInCreateOnlyMode() bool {
	createOnlyEnvValue := os.Getenv(createOnlyEnvName)
	return createOnlyEnvValue == "true"
}

// ZTWIMSpecChangedPredicate triggers reconciliation when ZTWIM spec is created
// while avoiding unnecessary reconciliations when only non-critical fields change
var ZTWIMSpecChangedPredicate = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		return true
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		return false
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return true
	},
	GenericFunc: func(e event.GenericEvent) bool {
		return false
	},
}

// OwnerReferenceChangedPredicate triggers reconciliation when owner references change
// This is useful for detecting when owner references are removed or modified
var OwnerReferenceChangedPredicate = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		return true
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldOwners := e.ObjectOld.GetOwnerReferences()
		newOwners := e.ObjectNew.GetOwnerReferences()

		// Check if owner references length changed
		if len(oldOwners) != len(newOwners) {
			return true
		}

		// Check if any owner reference was modified
		oldOwnerMap := make(map[string]string)
		for _, owner := range oldOwners {
			oldOwnerMap[string(owner.UID)] = owner.Name
		}

		for _, owner := range newOwners {
			oldName, exists := oldOwnerMap[string(owner.UID)]
			if !exists || oldName != owner.Name {
				return true
			}
		}

		// No owner reference changes detected
		return false
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return true
	},
	GenericFunc: func(e event.GenericEvent) bool {
		return false
	},
}

// NeedsOwnerReferenceUpdate checks if an object's owner reference needs to be updated
// This prevents unnecessary reconciliations by only updating when the owner reference
// is missing or different from what's expected
func NeedsOwnerReferenceUpdate(obj client.Object, expectedOwner client.Object) bool {
	owners := obj.GetOwnerReferences()
	expectedUID := expectedOwner.GetUID()
	expectedName := expectedOwner.GetName()
	expectedKind := expectedOwner.GetObjectKind().GroupVersionKind().Kind

	// If no owner references exist, update is needed
	if len(owners) == 0 {
		return true
	}

	// Check if expected owner exists and matches (by UID, name, and kind)
	for _, owner := range owners {
		if owner.UID == expectedUID && owner.Name == expectedName && owner.Kind == expectedKind {
			// Owner reference is correct, no update needed
			return false
		}
	}

	// Expected owner not found or mismatched, update is needed
	return true
}

// GenerationOrOwnerReferenceChangedPredicate triggers reconciliation when either:
// 1. The resource generation changes (spec/status changes)
// 2. Owner references change (removed/modified)
// This is the standard predicate for all operand controllers
var GenerationOrOwnerReferenceChangedPredicate = predicate.Or(
	predicate.GenerationChangedPredicate{},
	OwnerReferenceChangedPredicate,
)
