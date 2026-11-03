package spiffe_csi_driver

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSpiffeCSIDriver_ExcludesNetworkPolicyReconciliation(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	pkgDir := filepath.Dir(filename)

	matches, err := filepath.Glob(filepath.Join(pkgDir, "*.go"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, path := range matches {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		if strings.Contains(filepath.Base(path), "network_policies") {
			t.Fatalf("unexpected NetworkPolicy reconcile file: %s", path)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		body := string(content)
		if strings.Contains(body, "networkingv1") || strings.Contains(body, "reconcileNetworkPolicies") {
			t.Fatalf("%s must not reference NetworkPolicy reconciliation", filepath.Base(path))
		}
	}
}
