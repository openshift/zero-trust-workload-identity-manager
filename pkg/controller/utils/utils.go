package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"

	routev1 "github.com/openshift/api/route/v1"
	securityv1 "github.com/openshift/api/security/v1"
	"k8s.io/utils/ptr"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
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

func init() {
	// Register core, storage and rbac schemes
	_ = corev1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	_ = admissionregistrationv1.AddToScheme(scheme)
	_ = securityv1.AddToScheme(scheme)
	_ = routev1.AddToScheme(scheme)

	// Create a codec factory for this scheme
	codecs = serializer.NewCodecFactory(scheme)
}

const (
	LogLevelInfo  = "info"
	LogFormatText = "text"
)

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

// volumesEqual compares two volume slices for equality
func volumesEqual(desired, fetched []corev1.Volume) bool {
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
			if !reflect.DeepEqual(desiredVol.Secret.Items, fetchedVol.Secret.Items) {
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
			if !reflect.DeepEqual(desiredVol.Projected, fetchedVol.Projected) {
				return false
			}
		}

		// Check CSI volume
		if desiredVol.CSI != nil {
			if fetchedVol.CSI == nil {
				return false
			}
			if !reflect.DeepEqual(desiredVol.CSI, fetchedVol.CSI) {
				return false
			}
		}
	}

	return true
}

// containerSpecModified checks if a container spec has been modified
func containerSpecModified(desired, fetched *corev1.Container) bool {
	// Check basic container properties
	if desired.Name != fetched.Name ||
		desired.Image != fetched.Image ||
		desired.ImagePullPolicy != fetched.ImagePullPolicy {
		return true
	}

	// Check args
	if !reflect.DeepEqual(desired.Args, fetched.Args) {
		return true
	}

	// Check environment variables
	if !reflect.DeepEqual(desired.Env, fetched.Env) {
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
		if !ok || !reflect.DeepEqual(desiredPort, fetchedPort) {
			return true
		}
	}

	// ReadinessProbe nil checks
	if (desired.ReadinessProbe == nil) != (fetched.ReadinessProbe == nil) {
		return true
	}
	if desired.ReadinessProbe != nil && fetched.ReadinessProbe != nil &&
		!reflect.DeepEqual(desired.ReadinessProbe.HTTPGet, fetched.ReadinessProbe.HTTPGet) {
		return true
	}

	// LivenessProbe nil checks
	if (desired.LivenessProbe == nil) != (fetched.LivenessProbe == nil) {
		return true
	}
	if desired.LivenessProbe != nil && fetched.LivenessProbe != nil &&
		!reflect.DeepEqual(desired.LivenessProbe.HTTPGet, fetched.LivenessProbe.HTTPGet) {
		return true
	}

	// SecurityContext checks
	if (desired.SecurityContext == nil) != (fetched.SecurityContext == nil) {
		return true
	}
	if desired.SecurityContext != nil && !reflect.DeepEqual(desired.SecurityContext, fetched.SecurityContext) {
		return true
	}

	// Check volume mounts
	if !reflect.DeepEqual(desired.VolumeMounts, fetched.VolumeMounts) {
		return true
	}

	// Check resources
	if !reflect.DeepEqual(desired.Resources, fetched.Resources) {
		return true
	}

	return false
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
	if len(dPod.NodeSelector) > 0 && !reflect.DeepEqual(dPod.NodeSelector, fPod.NodeSelector) {
		return true
	}
	if !reflect.DeepEqual(dPod.Affinity, fPod.Affinity) {
		return true
	}
	if len(dPod.Tolerations) != len(fPod.Tolerations) {
		return true
	}
	if len(dPod.Tolerations) > 0 && !reflect.DeepEqual(dPod.Tolerations, fPod.Tolerations) {
		return true
	}
	// Check volumes
	if !volumesEqual(dPod.Volumes, fPod.Volumes) {
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
		if containerSpecModified(&dCont, &fCont) {
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
		if containerSpecModified(&dInitCont, &fInitCont) {
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
	if len(dPod.NodeSelector) > 0 && !reflect.DeepEqual(dPod.NodeSelector, fPod.NodeSelector) {
		return true
	}
	if !reflect.DeepEqual(dPod.Affinity, fPod.Affinity) {
		return true
	}
	if len(dPod.Tolerations) != len(fPod.Tolerations) {
		return true
	}
	if len(dPod.Tolerations) > 0 && !reflect.DeepEqual(dPod.Tolerations, fPod.Tolerations) {
		return true
	}
	// Check volumes
	if !volumesEqual(dPod.Volumes, fPod.Volumes) {
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
		if containerSpecModified(&dCont, &fCont) {
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
		if containerSpecModified(&dInitCont, &fInitCont) {
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
	if len(dPod.NodeSelector) > 0 && !reflect.DeepEqual(dPod.NodeSelector, fPod.NodeSelector) {
		return true
	}
	if !reflect.DeepEqual(dPod.Affinity, fPod.Affinity) {
		return true
	}
	if len(dPod.Tolerations) != len(fPod.Tolerations) {
		return true
	}
	if len(dPod.Tolerations) > 0 && !reflect.DeepEqual(dPod.Tolerations, fPod.Tolerations) {
		return true
	}
	// Check volumes
	if !volumesEqual(dPod.Volumes, fPod.Volumes) {
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
		if containerSpecModified(&dCont, &fCont) {
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
		if containerSpecModified(&dInitCont, &fInitCont) {
			return true
		}
	}
	return false
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
