package spiffe_helper

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func newAdmissionRequest(pod *corev1.Pod) admission.Request {
	raw, _ := json.Marshal(pod)
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: raw,
			},
		},
	}
}

func TestHandle_NoAnnotation(t *testing.T) {
	injector := NewSpiffeHelperInjector()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "nginx"},
			},
		},
	}

	resp := injector.Handle(context.Background(), newAdmissionRequest(pod))
	if !resp.Allowed {
		t.Error("Expected pod without annotation to be allowed")
	}
	if len(resp.Patches) != 0 {
		t.Error("Expected no patches for pod without annotation")
	}
}

func TestHandle_WithAnnotation(t *testing.T) {
	t.Setenv("RELATED_IMAGE_SPIFFE_HELPER", "ghcr.io/spiffe/spiffe-helper:0.11.0")

	injector := NewSpiffeHelperInjector()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				AnnotationInjectHelper: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "nginx"},
			},
		},
	}

	resp := injector.Handle(context.Background(), newAdmissionRequest(pod))
	if !resp.Allowed {
		t.Error("Expected pod with annotation to be allowed")
	}
	if len(resp.Patches) == 0 {
		t.Error("Expected patches for pod with injection annotation")
	}

	// Apply the patches by re-marshaling
	mutatedPod := applyResponse(t, pod, resp)

	// Verify init container
	foundInit := false
	for _, c := range mutatedPod.Spec.InitContainers {
		if c.Name == InitContainerName {
			foundInit = true
			if c.Image != "ghcr.io/spiffe/spiffe-helper:0.11.0" {
				t.Errorf("Expected init container image 'ghcr.io/spiffe/spiffe-helper:0.11.0', got '%s'", c.Image)
			}
		}
	}
	if !foundInit {
		t.Error("Expected spiffe-helper-init init container to be injected")
	}

	// Verify sidecar container
	foundSidecar := false
	for _, c := range mutatedPod.Spec.Containers {
		if c.Name == SidecarContainerName {
			foundSidecar = true
			if c.Image != "ghcr.io/spiffe/spiffe-helper:0.11.0" {
				t.Errorf("Expected sidecar image 'ghcr.io/spiffe/spiffe-helper:0.11.0', got '%s'", c.Image)
			}
		}
	}
	if !foundSidecar {
		t.Error("Expected spiffe-helper sidecar container to be injected")
	}

	// Verify volumes
	volumeNames := map[string]bool{}
	for _, v := range mutatedPod.Spec.Volumes {
		volumeNames[v.Name] = true
	}
	for _, expected := range []string{VolumeNameWorkloadAPI, VolumeNameCerts, VolumeNameHelperConfig} {
		if !volumeNames[expected] {
			t.Errorf("Expected volume '%s' to be injected", expected)
		}
	}

	// Verify cert volume is mounted read-only in the app container
	for _, c := range mutatedPod.Spec.Containers {
		if c.Name == "app" {
			foundMount := false
			for _, vm := range c.VolumeMounts {
				if vm.Name == VolumeNameCerts {
					foundMount = true
					if !vm.ReadOnly {
						t.Error("Expected cert volume mount in app container to be read-only")
					}
					if vm.MountPath != DefaultCertDir {
						t.Errorf("Expected cert mount path '%s', got '%s'", DefaultCertDir, vm.MountPath)
					}
				}
			}
			if !foundMount {
				t.Error("Expected cert volume to be mounted in app container")
			}
		}
	}
}

func TestHandle_AlreadyInjected(t *testing.T) {
	t.Setenv("RELATED_IMAGE_SPIFFE_HELPER", "ghcr.io/spiffe/spiffe-helper:0.11.0")

	injector := NewSpiffeHelperInjector()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				AnnotationInjectHelper: "true",
			},
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: InitContainerName, Image: "ghcr.io/spiffe/spiffe-helper:0.11.0"},
			},
			Containers: []corev1.Container{
				{Name: "app", Image: "nginx"},
				{Name: SidecarContainerName, Image: "ghcr.io/spiffe/spiffe-helper:0.11.0"},
			},
		},
	}

	resp := injector.Handle(context.Background(), newAdmissionRequest(pod))
	if !resp.Allowed {
		t.Error("Expected already-injected pod to be allowed")
	}
	if len(resp.Patches) != 0 {
		t.Error("Expected no patches for already-injected pod")
	}
}

func TestHandle_CustomCertDir(t *testing.T) {
	t.Setenv("RELATED_IMAGE_SPIFFE_HELPER", "ghcr.io/spiffe/spiffe-helper:0.11.0")

	injector := NewSpiffeHelperInjector()
	customDir := "/custom/certs"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				AnnotationInjectHelper: "true",
				AnnotationCertDir:      customDir,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "nginx"},
			},
		},
	}

	resp := injector.Handle(context.Background(), newAdmissionRequest(pod))
	if !resp.Allowed {
		t.Error("Expected pod to be allowed")
	}

	mutatedPod := applyResponse(t, pod, resp)

	// Check cert mount path in app container
	for _, c := range mutatedPod.Spec.Containers {
		if c.Name == "app" {
			for _, vm := range c.VolumeMounts {
				if vm.Name == VolumeNameCerts {
					if vm.MountPath != customDir {
						t.Errorf("Expected custom cert dir '%s', got '%s'", customDir, vm.MountPath)
					}
				}
			}
		}
	}
}

func TestHandle_CustomConfigMap(t *testing.T) {
	t.Setenv("RELATED_IMAGE_SPIFFE_HELPER", "ghcr.io/spiffe/spiffe-helper:0.11.0")

	injector := NewSpiffeHelperInjector()
	customCM := "my-helper-config"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				AnnotationInjectHelper: "true",
				AnnotationHelperConfig: customCM,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "nginx"},
			},
		},
	}

	resp := injector.Handle(context.Background(), newAdmissionRequest(pod))
	if !resp.Allowed {
		t.Error("Expected pod to be allowed")
	}

	mutatedPod := applyResponse(t, pod, resp)

	// Check configmap name in volume
	for _, v := range mutatedPod.Spec.Volumes {
		if v.Name == VolumeNameHelperConfig {
			if v.ConfigMap == nil {
				t.Fatal("Expected ConfigMap volume source")
			}
			if v.ConfigMap.Name != customCM {
				t.Errorf("Expected configmap name '%s', got '%s'", customCM, v.ConfigMap.Name)
			}
		}
	}
}

func TestHandle_MissingImage(t *testing.T) {
	t.Setenv("RELATED_IMAGE_SPIFFE_HELPER", "")

	injector := NewSpiffeHelperInjector()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				AnnotationInjectHelper: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "nginx"},
			},
		},
	}

	resp := injector.Handle(context.Background(), newAdmissionRequest(pod))
	if resp.Allowed {
		t.Error("Expected pod to be rejected when image env var is missing")
	}
	if resp.Result.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, resp.Result.Code)
	}
}

func TestHandle_IncompatibleVolume(t *testing.T) {
	t.Setenv("RELATED_IMAGE_SPIFFE_HELPER", "ghcr.io/spiffe/spiffe-helper:0.11.0")

	injector := NewSpiffeHelperInjector()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				AnnotationInjectHelper: "true",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "nginx"},
			},
			Volumes: []corev1.Volume{
				{
					Name: VolumeNameCerts,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{Path: "/some/path"},
					},
				},
			},
		},
	}

	resp := injector.Handle(context.Background(), newAdmissionRequest(pod))
	if resp.Allowed {
		t.Error("Expected pod with incompatible volume to be rejected")
	}
	if resp.Result.Code != http.StatusUnprocessableEntity {
		t.Errorf("Expected status code %d, got %d", http.StatusUnprocessableEntity, resp.Result.Code)
	}
}

func TestHandle_InvalidPod(t *testing.T) {
	injector := NewSpiffeHelperInjector()
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: []byte("invalid json"),
			},
		},
	}

	resp := injector.Handle(context.Background(), req)
	if resp.Allowed {
		t.Error("Expected invalid pod to be rejected")
	}
	if resp.Result.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, resp.Result.Code)
	}
}

// applyResponse applies the actual JSON patches from the admission response to the original pod.
// It re-invokes Handle to get the mutated raw bytes from PatchResponseFromRaw,
// then unmarshals the result to verify the actual webhook output.
func applyResponse(t *testing.T, original *corev1.Pod, resp admission.Response) *corev1.Pod {
	t.Helper()

	if len(resp.Patches) == 0 {
		t.Fatal("expected patches but got none")
	}

	// Marshal the original pod and apply patches via jsonpatch
	originalJSON, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal original pod: %v", err)
	}

	// Build a generic document, apply patches manually
	var doc interface{}
	if err := json.Unmarshal(originalJSON, &doc); err != nil {
		t.Fatalf("failed to unmarshal original to generic: %v", err)
	}

	for _, patch := range resp.Patches {
		doc = applyPatch(t, doc, patch.Operation, patch.Path, patch.Value)
	}

	patchedJSON, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("failed to marshal patched doc: %v", err)
	}

	result := &corev1.Pod{}
	if err := json.Unmarshal(patchedJSON, result); err != nil {
		t.Fatalf("failed to unmarshal patched pod: %v", err)
	}
	return result
}

// applyPatch applies a single JSON patch operation to a document
func applyPatch(t *testing.T, doc interface{}, op, path string, value interface{}) interface{} {
	t.Helper()

	parts := splitPath(path)
	if len(parts) == 0 {
		return value
	}

	return applyPatchRecursive(t, doc, op, parts, value)
}

func splitPath(path string) []string {
	if path == "" || path == "/" {
		return nil
	}
	// Remove leading /
	if path[0] == '/' {
		path = path[1:]
	}
	result := []string{}
	for _, p := range splitOnSlash(path) {
		// Unescape JSON pointer
		p = replaceAll(p, "~1", "/")
		p = replaceAll(p, "~0", "~")
		result = append(result, p)
	}
	return result
}

func splitOnSlash(s string) []string {
	result := []string{}
	current := ""
	for _, c := range s {
		if c == '/' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	result = append(result, current)
	return result
}

func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old) - 1
		} else {
			result += string(s[i])
		}
	}
	return result
}

func applyPatchRecursive(t *testing.T, doc interface{}, op string, parts []string, value interface{}) interface{} {
	t.Helper()

	if len(parts) == 1 {
		key := parts[0]
		switch d := doc.(type) {
		case map[string]interface{}:
			if op == "add" || op == "replace" {
				d[key] = value
			}
			return d
		case []interface{}:
			if key == "-" {
				return append(d, value)
			}
			// Numeric index
			idx := 0
			for _, c := range key {
				idx = idx*10 + int(c-'0')
			}
			if op == "add" {
				// Insert at index
				result := make([]interface{}, len(d)+1)
				copy(result, d[:idx])
				result[idx] = value
				copy(result[idx+1:], d[idx:])
				return result
			}
			d[idx] = value
			return d
		default:
			t.Fatalf("unexpected type at path: %T", doc)
			return nil
		}
	}

	key := parts[0]
	rest := parts[1:]

	switch d := doc.(type) {
	case map[string]interface{}:
		child, exists := d[key]
		if !exists {
			child = map[string]interface{}{}
		}
		d[key] = applyPatchRecursive(t, child, op, rest, value)
		return d
	case []interface{}:
		idx := 0
		for _, c := range key {
			idx = idx*10 + int(c-'0')
		}
		d[idx] = applyPatchRecursive(t, d[idx], op, rest, value)
		return d
	default:
		t.Fatalf("unexpected type at path %s: %T", key, doc)
		return nil
	}
}
