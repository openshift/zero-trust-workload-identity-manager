package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"sort"
	"strings"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/featuregate"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/pointer"
)

var (
	scheme = runtime.NewScheme()
	codecs serializer.CodecFactory
)

func init() {
	// Register core, storage and rbac schemes
	_ = corev1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = admissionregistrationv1.AddToScheme(scheme)
	_ = securityv1.AddToScheme(scheme)

	// Create a codec factory for this scheme
	codecs = serializer.NewCodecFactory(scheme)
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

// IsAutoReconcileDisabled returns true if the DISABLE_AUTO_RECONCILE feature gate is enabled.
func IsAutoReconcileDisabled() bool {
	return featuregate.IsAutoReconcileDisabled()
}

func StringToBool(s string) bool {
	if s == "true" {
		return true
	}
	return false
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

func StatefulSetSpecModified(desired, fetched *appsv1.StatefulSet) bool {
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

	if !reflect.DeepEqual(ds.Selector, fs.Selector) {
		return true
	}

	if !reflect.DeepEqual(ds.Template.Labels, fs.Template.Labels) {
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
	if !pointer.BoolEqual(dPod.ShareProcessNamespace, fPod.ShareProcessNamespace) {
		return true
	}
	if desired.Spec.Template.Spec.NodeSelector != nil && len(desired.Spec.Template.Spec.NodeSelector) != 0 && !reflect.DeepEqual(desired.Spec.Template.Spec.NodeSelector, fetched.Spec.Template.Spec.NodeSelector) {
		return true
	}
	if desired.Spec.Template.Spec.Affinity != nil && !reflect.DeepEqual(desired.Spec.Template.Spec.Affinity, fetched.Spec.Template.Spec.Affinity) {
		return true
	}
	if desired.Spec.Template.Spec.Tolerations != nil && len(desired.Spec.Template.Spec.NodeSelector) != 0 && !reflect.DeepEqual(desired.Spec.Template.Spec.Tolerations, fetched.Spec.Template.Spec.Tolerations) {
		return true
	}
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
		if dCont.Image != fCont.Image {
			return true
		}
		if dCont.ImagePullPolicy != fCont.ImagePullPolicy {
			return true
		}
		if !reflect.DeepEqual(dCont.Args, fCont.Args) {
			return true
		}
		if !reflect.DeepEqual(dCont.Env, fCont.Env) {
			return true
		}
		if !reflect.DeepEqual(dCont.Resources, fCont.Resources) {
			return true
		}
		if !reflect.DeepEqual(dCont.VolumeMounts, fCont.VolumeMounts) {
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
		if !reflect.DeepEqual(dvc.Spec.AccessModes, fvc.Spec.AccessModes) {
			return true
		}
		if !reflect.DeepEqual(dvc.Spec.Resources.Requests, fvc.Spec.Resources.Requests) {
			return true
		}
	}
	return false
}

func DeploymentSpecModified(desired, fetched *appsv1.Deployment) bool {
	if desired == nil || fetched == nil {
		return true
	}
	ds := desired.Spec
	fs := fetched.Spec
	if ds.Replicas != nil && fs.Replicas != nil && *ds.Replicas != *fs.Replicas {
		return true
	}
	if !reflect.DeepEqual(ds.Selector, fs.Selector) {
		return true
	}
	if !reflect.DeepEqual(ds.Template.Labels, fs.Template.Labels) {
		return true
	}
	dPod := ds.Template.Spec
	fPod := fs.Template.Spec
	if dPod.ServiceAccountName != fPod.ServiceAccountName {
		return true
	}
	if !pointer.BoolEqual(dPod.ShareProcessNamespace, fPod.ShareProcessNamespace) {
		return true
	}
	if desired.Spec.Template.Spec.NodeSelector != nil && len(desired.Spec.Template.Spec.NodeSelector) != 0 && !reflect.DeepEqual(desired.Spec.Template.Spec.NodeSelector, fetched.Spec.Template.Spec.NodeSelector) {
		return true
	}
	if desired.Spec.Template.Spec.Affinity != nil && !reflect.DeepEqual(desired.Spec.Template.Spec.Affinity, fetched.Spec.Template.Spec.Affinity) {
		return true
	}
	if desired.Spec.Template.Spec.Tolerations != nil && len(desired.Spec.Template.Spec.NodeSelector) != 0 && !reflect.DeepEqual(desired.Spec.Template.Spec.Tolerations, fetched.Spec.Template.Spec.Tolerations) {
		return true
	}
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
		if dCont.Image != fCont.Image {
			return true
		}
		if dCont.ImagePullPolicy != fCont.ImagePullPolicy {
			return true
		}
		if !reflect.DeepEqual(dCont.Args, fCont.Args) {
			return true
		}
		if !reflect.DeepEqual(dCont.Env, fCont.Env) {
			return true
		}
		if !reflect.DeepEqual(dCont.Resources, fCont.Resources) {
			return true
		}
		if !reflect.DeepEqual(dCont.VolumeMounts, fCont.VolumeMounts) {
			return true
		}
	}
	return false
}

func DaemonSetSpecModified(desired, fetched *appsv1.DaemonSet) bool {
	if desired == nil || fetched == nil {
		return true
	}
	ds := desired.Spec
	fs := fetched.Spec
	if !reflect.DeepEqual(ds.Selector, fs.Selector) {
		return true
	}
	if !reflect.DeepEqual(ds.Template.Labels, fs.Template.Labels) {
		return true
	}
	dPod := ds.Template.Spec
	fPod := fs.Template.Spec
	if dPod.ServiceAccountName != fPod.ServiceAccountName {
		return true
	}
	if !pointer.BoolEqual(dPod.ShareProcessNamespace, fPod.ShareProcessNamespace) {
		return true
	}
	if desired.Spec.Template.Spec.NodeSelector != nil && len(desired.Spec.Template.Spec.NodeSelector) != 0 && !reflect.DeepEqual(desired.Spec.Template.Spec.NodeSelector, fetched.Spec.Template.Spec.NodeSelector) {
		return true
	}
	if desired.Spec.Template.Spec.Affinity != nil && !reflect.DeepEqual(desired.Spec.Template.Spec.Affinity, fetched.Spec.Template.Spec.Affinity) {
		return true
	}
	if desired.Spec.Template.Spec.Tolerations != nil && len(desired.Spec.Template.Spec.NodeSelector) != 0 && !reflect.DeepEqual(desired.Spec.Template.Spec.Tolerations, fetched.Spec.Template.Spec.Tolerations) {
		return true
	}
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
		if dCont.Image != fCont.Image {
			return true
		}
		if dCont.ImagePullPolicy != fCont.ImagePullPolicy {
			return true
		}
		if !reflect.DeepEqual(dCont.Args, fCont.Args) {
			return true
		}
		if !reflect.DeepEqual(dCont.Resources, fCont.Resources) {
			return true
		}
		if !reflect.DeepEqual(dCont.VolumeMounts, fCont.VolumeMounts) {
			return true
		}
	}
	return false
}
