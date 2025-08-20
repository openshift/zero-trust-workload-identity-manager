package spiffe_csi_driver

import (
	"reflect"
	"testing"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestGenerateSpiffeCSIDriverSCC(t *testing.T) {
	scc := generateSpiffeCSIDriverSCC()

	// Test that function returns non-nil SCC
	if scc == nil {
		t.Fatal("Expected non-nil SecurityContextConstraints, got nil")
	}

	// Test ObjectMeta
	testObjectMeta(t, scc.ObjectMeta)

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
}

func testObjectMeta(t *testing.T, meta metav1.ObjectMeta) {
	expectedName := "spire-spiffe-csi-driver"
	if meta.Name != expectedName {
		t.Errorf("Expected name '%s', got '%s'", expectedName, meta.Name)
	}

	// Verify other ObjectMeta fields are not set (as per the function)
	if meta.Namespace != "" {
		t.Errorf("Expected empty namespace, got '%s'", meta.Namespace)
	}

	expectedLabels := utils.SpiffeCSIDriverLabels(nil)
	if !reflect.DeepEqual(meta.Labels, expectedLabels) {
		t.Errorf("Expected labels %v, got %v", expectedLabels, meta.Labels)
	}

	if len(meta.Annotations) > 0 {
		t.Errorf("Expected no annotations, got %v", meta.Annotations)
	}
}

func testRunAsUserStrategy(t *testing.T, strategy securityv1.RunAsUserStrategyOptions) {
	expectedType := securityv1.RunAsUserStrategyRunAsAny
	if strategy.Type != expectedType {
		t.Errorf("Expected RunAsUser strategy type '%s', got '%s'", expectedType, strategy.Type)
	}

	// Verify other fields are not set for RunAsAny strategy
	if strategy.UID != nil {
		t.Errorf("Expected UID to be nil for RunAsAny strategy, got %v", strategy.UID)
	}

	if strategy.UIDRangeMin != nil {
		t.Errorf("Expected UIDRangeMin to be nil for RunAsAny strategy, got %v", strategy.UIDRangeMin)
	}

	if strategy.UIDRangeMax != nil {
		t.Errorf("Expected UIDRangeMax to be nil for RunAsAny strategy, got %v", strategy.UIDRangeMax)
	}
}

func testSELinuxContextStrategy(t *testing.T, strategy securityv1.SELinuxContextStrategyOptions) {
	expectedType := securityv1.SELinuxStrategyRunAsAny
	if strategy.Type != expectedType {
		t.Errorf("Expected SELinuxContext strategy type '%s', got '%s'", expectedType, strategy.Type)
	}

	// Verify SELinuxOptions is not set for RunAsAny strategy
	if strategy.SELinuxOptions != nil {
		t.Errorf("Expected SELinuxOptions to be nil for RunAsAny strategy, got %v", strategy.SELinuxOptions)
	}
}

func testSupplementalGroupsStrategy(t *testing.T, strategy securityv1.SupplementalGroupsStrategyOptions) {
	expectedType := securityv1.SupplementalGroupsStrategyRunAsAny
	if strategy.Type != expectedType {
		t.Errorf("Expected SupplementalGroups strategy type '%s', got '%s'", expectedType, strategy.Type)
	}

	// Verify ranges are not set for RunAsAny strategy
	if len(strategy.Ranges) > 0 {
		t.Errorf("Expected no ranges for RunAsAny strategy, got %v", strategy.Ranges)
	}
}

func testFSGroupStrategy(t *testing.T, strategy securityv1.FSGroupStrategyOptions) {
	expectedType := securityv1.FSGroupStrategyRunAsAny
	if strategy.Type != expectedType {
		t.Errorf("Expected FSGroup strategy type '%s', got '%s'", expectedType, strategy.Type)
	}

	// Verify ranges are not set for RunAsAny strategy
	if len(strategy.Ranges) > 0 {
		t.Errorf("Expected no ranges for RunAsAny strategy, got %v", strategy.Ranges)
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
	// Test DefaultAddCapabilities
	if scc.DefaultAddCapabilities != nil {
		t.Errorf("Expected DefaultAddCapabilities to be nil, got %v", scc.DefaultAddCapabilities)
	}

	// Test RequiredDropCapabilities
	if scc.RequiredDropCapabilities != nil {
		t.Errorf("Expected RequiredDropCapabilities to be nil, got %v", scc.RequiredDropCapabilities)
	}
}

// Test table-driven approach for different SCC field validations
func TestSCCFieldValidation(t *testing.T) {
	scc := generateSpiffeCSIDriverSCC()

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
	scc := generateSpiffeCSIDriverSCC()

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
	scc := generateSpiffeCSIDriverSCC()

	strategyTests := []struct {
		name     string
		actual   interface{}
		expected interface{}
	}{
		{
			name:     "RunAsUser strategy",
			actual:   scc.RunAsUser.Type,
			expected: securityv1.RunAsUserStrategyRunAsAny,
		},
		{
			name:     "SELinuxContext strategy",
			actual:   scc.SELinuxContext.Type,
			expected: securityv1.SELinuxStrategyRunAsAny,
		},
		{
			name:     "SupplementalGroups strategy",
			actual:   scc.SupplementalGroups.Type,
			expected: securityv1.SupplementalGroupsStrategyRunAsAny,
		},
		{
			name:     "FSGroup strategy",
			actual:   scc.FSGroup.Type,
			expected: securityv1.FSGroupStrategyRunAsAny,
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
		generateSpiffeCSIDriverSCC()
	}
}

// Test for immutability - ensure function returns new instance each time
func TestSCCImmutability(t *testing.T) {
	scc1 := generateSpiffeCSIDriverSCC()
	scc2 := generateSpiffeCSIDriverSCC()

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
