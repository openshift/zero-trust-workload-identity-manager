package utils

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestIsCreateOnlyMode(t *testing.T) {
	tests := []struct {
		name        string
		obj         client.Object
		expected    bool
		description string
	}{
		{
			name:        "nil object",
			obj:         nil,
			expected:    false,
			description: "should return false when object is nil",
		},
		{
			name: "object with nil annotations",
			obj: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: nil,
				},
			},
			expected:    false,
			description: "should return false when annotations map is nil",
		},
		{
			name: "object with empty annotations",
			obj: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			expected:    false,
			description: "should return false when annotations map is empty",
		},
		{
			name: "object with create-only annotation set to true",
			obj: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						CreateOnlyAnnotation: "true",
					},
				},
			},
			expected:    true,
			description: "should return true when create-only annotation is set to 'true'",
		},
		{
			name: "object with create-only annotation set to false",
			obj: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						CreateOnlyAnnotation: "false",
					},
				},
			},
			expected:    false,
			description: "should return false when create-only annotation is set to 'false'",
		},
		{
			name: "object with create-only annotation set to empty string",
			obj: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						CreateOnlyAnnotation: "",
					},
				},
			},
			expected:    false,
			description: "should return false when create-only annotation is empty string",
		},
		{
			name: "object with create-only annotation set to invalid value",
			obj: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						CreateOnlyAnnotation: "invalid",
					},
				},
			},
			expected:    false,
			description: "should return false when create-only annotation has invalid value",
		},
		{
			name: "object with other annotations but no create-only",
			obj: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"other.annotation": "value",
						"another.one":      "true",
					},
				},
			},
			expected:    false,
			description: "should return false when create-only annotation is not present",
		},
		{
			name: "object with create-only annotation and other annotations",
			obj: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						CreateOnlyAnnotation: "true",
						"other.annotation":   "value",
					},
				},
			},
			expected:    true,
			description: "should return true when create-only annotation is 'true' even with other annotations",
		},
		{
			name: "SpireServer object with create-only annotation",
			obj: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						CreateOnlyAnnotation: "true",
					},
				},
			},
			expected:    true,
			description: "should return true for SpireServer with create-only annotation set to 'true'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCreateOnlyAnnotationEnabled(tt.obj)
			if result != tt.expected {
				t.Errorf("IsCreateOnlyAnnotationEnabled() = %v, expected %v - %s", result, tt.expected, tt.description)
			}
		})
	}
}

func TestIsInCreateOnlyMode(t *testing.T) {
	tests := []struct {
		name              string
		obj               client.Object
		createOnlyFlag    bool
		expectedResult    bool
		expectedFlagAfter bool
		description       string
	}{
		{
			name:              "nil object with flag false",
			obj:               nil,
			createOnlyFlag:    false,
			expectedResult:    false,
			expectedFlagAfter: false,
			description:       "should return false when object is nil and flag is false",
		},
		{
			name:              "nil object with flag true",
			obj:               nil,
			createOnlyFlag:    true,
			expectedResult:    true,
			expectedFlagAfter: true,
			description:       "should return true when object is nil but flag is true",
		},
		{
			name: "object without annotation, flag false",
			obj: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			createOnlyFlag:    false,
			expectedResult:    false,
			expectedFlagAfter: false,
			description:       "should return false when object has no annotation and flag is false",
		},
		{
			name: "object without annotation, flag true",
			obj: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			createOnlyFlag:    true,
			expectedResult:    true,
			expectedFlagAfter: true,
			description:       "should return true when object has no annotation but flag is true",
		},
		{
			name: "object with annotation true, flag false",
			obj: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						CreateOnlyAnnotation: "true",
					},
				},
			},
			createOnlyFlag:    false,
			expectedResult:    true,
			expectedFlagAfter: true,
			description:       "should return true and set flag to true when object has annotation set to true",
		},
		{
			name: "object with annotation true, flag true",
			obj: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						CreateOnlyAnnotation: "true",
					},
				},
			},
			createOnlyFlag:    true,
			expectedResult:    true,
			expectedFlagAfter: true,
			description:       "should return true and keep flag true when both object annotation and flag are true",
		},
		{
			name: "object with annotation false, flag false",
			obj: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						CreateOnlyAnnotation: "false",
					},
				},
			},
			createOnlyFlag:    false,
			expectedResult:    false,
			expectedFlagAfter: false,
			description:       "should return false when both object annotation and flag are false",
		},
		{
			name: "object with annotation false, flag true",
			obj: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						CreateOnlyAnnotation: "false",
					},
				},
			},
			createOnlyFlag:    true,
			expectedResult:    true,
			expectedFlagAfter: true,
			description:       "should return true when object annotation is false but flag is true",
		},
		{
			name: "SpireServer object with annotation true, flag false",
			obj: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						CreateOnlyAnnotation: "true",
					},
				},
			},
			createOnlyFlag:    false,
			expectedResult:    true,
			expectedFlagAfter: true,
			description:       "should return true and set flag to true for SpireServer with annotation set to true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of the flag since it's passed by reference
			flagCopy := tt.createOnlyFlag
			result := IsInCreateOnlyMode(tt.obj, &flagCopy)

			if result != tt.expectedResult {
				t.Errorf("IsInCreateOnlyMode() = %v, expected %v - %s", result, tt.expectedResult, tt.description)
			}

			if flagCopy != tt.expectedFlagAfter {
				t.Errorf("createOnlyFlag after call = %v, expected %v - %s", flagCopy, tt.expectedFlagAfter, tt.description)
			}
		})
	}
}

func TestIsInCreateOnlyMode_WithUnstructuredObject(t *testing.T) {
	// Test with a real Kubernetes object type to ensure compatibility
	obj := &unstructured.Unstructured{}
	obj.SetAnnotations(map[string]string{
		CreateOnlyAnnotation: "true",
	})

	flag := false
	result := IsInCreateOnlyMode(obj, &flag)

	if !result {
		t.Errorf("IsInCreateOnlyMode() with unstructured object = %v, expected true", result)
	}

	if !flag {
		t.Errorf("createOnlyFlag after call with unstructured object = %v, expected true", flag)
	}
}

func TestIsInCreateOnlyMode_WithDifferentAPITypes(t *testing.T) {
	// Test with SpireAgent
	spireAgent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				CreateOnlyAnnotation: "true",
			},
		},
	}

	flag := false
	result := IsInCreateOnlyMode(spireAgent, &flag)

	if !result {
		t.Errorf("IsInCreateOnlyMode() with SpireAgent = %v, expected true", result)
	}

	if !flag {
		t.Errorf("createOnlyFlag after call with SpireAgent = %v, expected true", flag)
	}

	// Test with SpireOIDCDiscoveryProvider
	spireOIDC := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				CreateOnlyAnnotation: "false",
			},
		},
	}

	flag2 := true
	result2 := IsInCreateOnlyMode(spireOIDC, &flag2)

	if !result2 {
		t.Errorf("IsInCreateOnlyMode() with SpireOIDCDiscoveryProvider = %v, expected true", result2)
	}

	if !flag2 {
		t.Errorf("createOnlyFlag after call with SpireOIDCDiscoveryProvider = %v, expected true", flag2)
	}
}
