package spiffe_csi_driver

import (
	"reflect"
	"testing"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestGenerateSpiffeCSIDriverSCC(t *testing.T) {
	t.Run("without custom labels", func(t *testing.T) {
		scc := generateSpiffeCSIDriverSCC(nil)

		// Test that function returns non-nil SCC
		if scc == nil {
			t.Fatal("Expected non-nil SecurityContextConstraints, got nil")
		}

		// Test ObjectMeta
		testObjectMeta(t, scc.ObjectMeta, nil)

		// Test ReadOnlyRootFilesystem
		if !scc.ReadOnlyRootFilesystem {
			t.Error("Expected ReadOnlyRootFilesystem to be true")
		}

		// Test RunAsUser strategy
		testRunAsUserStrategy(t, scc.RunAsUser)

		// Test SELinuxContext strategy
		testSELinuxContextStrategy(t, scc.SELinuxContext)

		// Test SupplementalGroups strategy
		testSupplementalGroupsStrategy(t, scc.SupplementalGroups)

		// Test FSGroup strategy
		testFSGroupStrategy(t, scc.FSGroup)

		// Test Users
		testUsers(t, scc.Users)

		// Test Volumes
		testSCCVolumes(t, scc.Volumes)

		// Test host-related permissions
		testHostPermissions(t, scc)

		// Test privilege settings
		testPrivilegeSettings(t, scc)

		// Test capabilities
		testCapabilities(t, scc)
	})

	t.Run("with custom labels", func(t *testing.T) {
		customLabels := map[string]string{
			"custom-label-1": "value-1",
			"environment":    "production",
		}
		scc := generateSpiffeCSIDriverSCC(customLabels)

		if scc == nil {
			t.Fatal("Expected non-nil SecurityContextConstraints, got nil")
		}

		// Test ObjectMeta with custom labels
		testObjectMeta(t, scc.ObjectMeta, customLabels)

		// Verify custom labels are present
		if val, ok := scc.Labels["custom-label-1"]; !ok || val != "value-1" {
			t.Errorf("Expected custom label 'custom-label-1=value-1', got '%s'", val)
		}

		if val, ok := scc.Labels["environment"]; !ok || val != "production" {
			t.Errorf("Expected custom label 'environment=production', got '%s'", val)
		}
	})

	t.Run("preserves all labels", func(t *testing.T) {
		// Get labels without custom labels
		sccWithoutCustom := generateSpiffeCSIDriverSCC(nil)
		baseLabels := make(map[string]string)
		for k, v := range sccWithoutCustom.Labels {
			baseLabels[k] = v
		}

		// Get labels with custom labels
		customLabels := map[string]string{
			"test-label": "test-value",
		}
		sccWithCustom := generateSpiffeCSIDriverSCC(customLabels)

		// All base labels should still be present
		for k, v := range baseLabels {
			if sccWithCustom.Labels[k] != v {
				t.Errorf("Base label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, sccWithCustom.Labels[k])
			}
		}

		// Custom label should also be present
		if val, ok := sccWithCustom.Labels["test-label"]; !ok || val != "test-value" {
			t.Errorf("Custom label was not added")
		}
	})
}

func testObjectMeta(t *testing.T, meta metav1.ObjectMeta, customLabels map[string]string) {
	expectedName := "spire-spiffe-csi-driver"
	if meta.Name != expectedName {
		t.Errorf("Expected name '%s', got '%s'", expectedName, meta.Name)
	}

	// Verify other ObjectMeta fields are not set (as per the function)
	if meta.Namespace != "" {
		t.Errorf("Expected empty namespace, got '%s'", meta.Namespace)
	}

	expectedLabels := utils.SpiffeCSIDriverLabels(customLabels)
	if !reflect.DeepEqual(meta.Labels, expectedLabels) {
		t.Errorf("Expected labels %v, got %v", expectedLabels, meta.Labels)
	}

	// Verify standard labels are always present
	if val, ok := meta.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
		t.Errorf("Expected standard label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
	}

	if val, ok := meta.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentCSI {
		t.Errorf("Expected standard label app.kubernetes.io/component=%s", utils.ComponentCSI)
	}

	// If custom labels were provided, verify they are present
	if customLabels != nil {
		for k, v := range customLabels {
			if meta.Labels[k] != v {
				t.Errorf("Expected custom label '%s=%s', got '%s'", k, v, meta.Labels[k])
			}
		}
	}

	if len(meta.Annotations) > 0 {
		t.Errorf("Expected no annotations, got %v", meta.Annotations)
	}
}

func testRunAsUserStrategy(t *testing.T, strategy securityv1.RunAsUserStrategyOptions) {
	expectedType := securityv1.RunAsUserStrategyMustRunAsRange
	if strategy.Type != expectedType {
		t.Errorf("Expected RunAsUser strategy type '%s', got '%s'", expectedType, strategy.Type)
	}

	// Verify other fields are not set for MustRunAsRange strategy
	if strategy.UID != nil {
		t.Errorf("Expected UID to be nil for MustRunAsRange strategy, got %v", strategy.UID)
	}

	if strategy.UIDRangeMin != nil {
		t.Errorf("Expected UIDRangeMin to be nil for MustRunAsRange strategy, got %v", strategy.UIDRangeMin)
	}

	if strategy.UIDRangeMax != nil {
		t.Errorf("Expected UIDRangeMax to be nil for MustRunAsRange strategy, got %v", strategy.UIDRangeMax)
	}
}

func testSELinuxContextStrategy(t *testing.T, strategy securityv1.SELinuxContextStrategyOptions) {
	expectedType := securityv1.SELinuxStrategyMustRunAs
	if strategy.Type != expectedType {
		t.Errorf("Expected SELinuxContext strategy type '%s', got '%s'", expectedType, strategy.Type)
	}

	// Verify SELinuxOptions is not set for MustRunAs strategy
	if strategy.SELinuxOptions != nil {
		t.Errorf("Expected SELinuxOptions to be nil for MustRunAs strategy, got %v", strategy.SELinuxOptions)
	}
}

func testSupplementalGroupsStrategy(t *testing.T, strategy securityv1.SupplementalGroupsStrategyOptions) {
	expectedType := securityv1.SupplementalGroupsStrategyMustRunAs
	if strategy.Type != expectedType {
		t.Errorf("Expected SupplementalGroups strategy type '%s', got '%s'", expectedType, strategy.Type)
	}

	// Verify ranges are not set for MustRunAs strategy
	if len(strategy.Ranges) > 0 {
		t.Errorf("Expected no ranges for MustRunAs strategy, got %v", strategy.Ranges)
	}
}

func testFSGroupStrategy(t *testing.T, strategy securityv1.FSGroupStrategyOptions) {
	expectedType := securityv1.FSGroupStrategyMustRunAs
	if strategy.Type != expectedType {
		t.Errorf("Expected FSGroup strategy type '%s', got '%s'", expectedType, strategy.Type)
	}

	// Verify ranges are not set for MustRunAs strategy
	if len(strategy.Ranges) > 0 {
		t.Errorf("Expected no ranges for MustRunAs strategy, got %v", strategy.Ranges)
	}
}

func testUsers(t *testing.T, users []string) {
	expectedUsers := []string{
		"system:serviceaccount:zero-trust-workload-identity-manager:spire-spiffe-csi-driver",
	}

	if len(users) != len(expectedUsers) {
		t.Errorf("Expected %d users, got %d", len(expectedUsers), len(users))
		return
	}

	if !reflect.DeepEqual(users, expectedUsers) {
		t.Errorf("Expected users %v, got %v", expectedUsers, users)
	}
}

func testSCCVolumes(t *testing.T, volumes []securityv1.FSType) {
	expectedVolumes := []securityv1.FSType{
		securityv1.FSTypeConfigMap,
		securityv1.FSTypeHostPath,
		securityv1.FSTypeSecret,
	}

	if len(volumes) != len(expectedVolumes) {
		t.Errorf("Expected %d volume types, got %d", len(expectedVolumes), len(volumes))
		return
	}

	// Check that all expected volumes are present (order doesn't matter for this test)
	volumeMap := make(map[securityv1.FSType]bool)
	for _, vol := range volumes {
		volumeMap[vol] = true
	}

	for _, expectedVol := range expectedVolumes {
		if !volumeMap[expectedVol] {
			t.Errorf("Expected volume type '%s' not found in volumes %v", expectedVol, volumes)
		}
	}

	// Alternatively, if order matters, use this instead:
	if !reflect.DeepEqual(volumes, expectedVolumes) {
		t.Errorf("Expected volumes %v, got %v", expectedVolumes, volumes)
	}
}

func testHostPermissions(t *testing.T, scc *securityv1.SecurityContextConstraints) {
	// Test AllowHostDirVolumePlugin
	if !scc.AllowHostDirVolumePlugin {
		t.Error("Expected AllowHostDirVolumePlugin to be true")
	}

	// Test AllowHostIPC
	if scc.AllowHostIPC {
		t.Error("Expected AllowHostIPC to be false")
	}

	// Test AllowHostNetwork
	if scc.AllowHostNetwork {
		t.Error("Expected AllowHostNetwork to be false")
	}

	// Test AllowHostPID
	if scc.AllowHostPID {
		t.Error("Expected AllowHostPID to be false")
	}

	// Test AllowHostPorts
	if scc.AllowHostPorts {
		t.Error("Expected AllowHostPorts to be false")
	}
}

func testPrivilegeSettings(t *testing.T, scc *securityv1.SecurityContextConstraints) {
	// Test AllowPrivilegeEscalation
	if scc.AllowPrivilegeEscalation == nil {
		t.Error("Expected AllowPrivilegeEscalation to be non-nil")
	} else if !*scc.AllowPrivilegeEscalation {
		t.Error("Expected AllowPrivilegeEscalation to be true")
	}

	// Test AllowPrivilegedContainer
	if !scc.AllowPrivilegedContainer {
		t.Error("Expected AllowPrivilegedContainer to be true")
	}
}

func testCapabilities(t *testing.T, scc *securityv1.SecurityContextConstraints) {
	// Test DefaultAddCapabilities - should be empty slice
	if scc.DefaultAddCapabilities == nil {
		t.Error("Expected DefaultAddCapabilities to be non-nil empty slice, got nil")
	} else if len(scc.DefaultAddCapabilities) != 0 {
		t.Errorf("Expected DefaultAddCapabilities to be empty, got %v", scc.DefaultAddCapabilities)
	}

	// Test RequiredDropCapabilities - should contain "ALL"
	expectedDropCapabilities := []corev1.Capability{"ALL"}
	if scc.RequiredDropCapabilities == nil {
		t.Error("Expected RequiredDropCapabilities to be non-nil, got nil")
	} else if !reflect.DeepEqual(scc.RequiredDropCapabilities, expectedDropCapabilities) {
		t.Errorf("Expected RequiredDropCapabilities to be %v, got %v", expectedDropCapabilities, scc.RequiredDropCapabilities)
	}
}

// Test for custom labels preservation
func TestSCCCustomLabels(t *testing.T) {
	t.Run("empty custom labels", func(t *testing.T) {
		scc := generateSpiffeCSIDriverSCC(map[string]string{})

		// Standard labels should still be present
		if val, ok := scc.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected standard label even with empty custom labels map")
		}
	})

	t.Run("multiple custom labels", func(t *testing.T) {
		customLabels := map[string]string{
			"label1": "value1",
			"label2": "value2",
			"label3": "value3",
		}
		scc := generateSpiffeCSIDriverSCC(customLabels)

		for k, v := range customLabels {
			if scc.Labels[k] != v {
				t.Errorf("Custom label '%s=%s' not found or has wrong value: '%s'", k, v, scc.Labels[k])
			}
		}

		// Standard labels should also be present
		if val, ok := scc.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected standard label with custom labels")
		}
	})
}

// Test table-driven approach for different SCC field validations
func TestSCCFieldValidation(t *testing.T) {
	scc := generateSpiffeCSIDriverSCC(nil)

	tests := []struct {
		name     string
		field    string
		expected interface{}
		actual   interface{}
	}{
		{
			name:     "ReadOnlyRootFilesystem",
			field:    "ReadOnlyRootFilesystem",
			expected: true,
			actual:   scc.ReadOnlyRootFilesystem,
		},
		{
			name:     "AllowHostDirVolumePlugin",
			field:    "AllowHostDirVolumePlugin",
			expected: true,
			actual:   scc.AllowHostDirVolumePlugin,
		},
		{
			name:     "AllowHostIPC",
			field:    "AllowHostIPC",
			expected: false,
			actual:   scc.AllowHostIPC,
		},
		{
			name:     "AllowHostNetwork",
			field:    "AllowHostNetwork",
			expected: false,
			actual:   scc.AllowHostNetwork,
		},
		{
			name:     "AllowHostPID",
			field:    "AllowHostPID",
			expected: false,
			actual:   scc.AllowHostPID,
		},
		{
			name:     "AllowHostPorts",
			field:    "AllowHostPorts",
			expected: false,
			actual:   scc.AllowHostPorts,
		},
		{
			name:     "AllowPrivilegedContainer",
			field:    "AllowPrivilegedContainer",
			expected: true,
			actual:   scc.AllowPrivilegedContainer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.actual, tt.expected) {
				t.Errorf("Field %s: expected %v, got %v", tt.field, tt.expected, tt.actual)
			}
		})
	}
}

// Test for AllowPrivilegeEscalation pointer validation
func TestAllowPrivilegeEscalationPointer(t *testing.T) {
	scc := generateSpiffeCSIDriverSCC(nil)

	if scc.AllowPrivilegeEscalation == nil {
		t.Fatal("Expected AllowPrivilegeEscalation to be non-nil")
	}

	expectedValue := true
	if *scc.AllowPrivilegeEscalation != expectedValue {
		t.Errorf("Expected AllowPrivilegeEscalation value to be %v, got %v",
			expectedValue, *scc.AllowPrivilegeEscalation)
	}

	// Test that it's using ptr.To(true) by comparing with manual pointer creation
	manualPtr := ptr.To(true)
	if *scc.AllowPrivilegeEscalation != *manualPtr {
		t.Error("AllowPrivilegeEscalation doesn't match expected ptr.To(true) value")
	}
}

// Test strategy types enumeration
func TestStrategyTypes(t *testing.T) {
	scc := generateSpiffeCSIDriverSCC(nil)

	strategyTests := []struct {
		name     string
		actual   interface{}
		expected interface{}
	}{
		{
			name:     "RunAsUser strategy",
			actual:   scc.RunAsUser.Type,
			expected: securityv1.RunAsUserStrategyMustRunAsRange,
		},
		{
			name:     "SELinuxContext strategy",
			actual:   scc.SELinuxContext.Type,
			expected: securityv1.SELinuxStrategyMustRunAs,
		},
		{
			name:     "SupplementalGroups strategy",
			actual:   scc.SupplementalGroups.Type,
			expected: securityv1.SupplementalGroupsStrategyMustRunAs,
		},
		{
			name:     "FSGroup strategy",
			actual:   scc.FSGroup.Type,
			expected: securityv1.FSGroupStrategyMustRunAs,
		},
	}

	for _, tt := range strategyTests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.actual != tt.expected {
				t.Errorf("%s: expected %v, got %v", tt.name, tt.expected, tt.actual)
			}
		})
	}
}

// Benchmark test for performance validation
func BenchmarkGenerateSpiffeCSIDriverSCC(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateSpiffeCSIDriverSCC(nil)
	}
}

// Test for immutability - ensure function returns new instance each time
func TestSCCImmutability(t *testing.T) {
	scc1 := generateSpiffeCSIDriverSCC(nil)
	scc2 := generateSpiffeCSIDriverSCC(nil)

	// They should be equal in content
	if !reflect.DeepEqual(scc1, scc2) {
		t.Error("Expected SCCs to have identical content")
	}

	// But they should be different instances
	if scc1 == scc2 {
		t.Error("Expected SCCs to be different instances")
	}

	// Modifying one shouldn't affect the other
	scc1.Name = "modified"
	if scc2.Name == "modified" {
		t.Error("Modifying one SCC affected the other - instances are not independent")
	}
}
