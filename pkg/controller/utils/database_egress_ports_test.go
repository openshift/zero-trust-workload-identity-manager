package utils

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type stubServiceGetter struct {
	svc *corev1.Service
}

func (s *stubServiceGetter) Get(_ context.Context, key client.ObjectKey, obj client.Object) error {
	if s.svc == nil || key.Name != s.svc.Name || key.Namespace != s.svc.Namespace {
		return errServiceNotFound
	}
	*(obj.(*corev1.Service)) = *s.svc
	return nil
}

var errServiceNotFound = &serviceNotFoundError{}

type serviceNotFoundError struct{}

func (e *serviceNotFoundError) Error() string { return "service not found" }

func TestParseKubernetesServiceHost(t *testing.T) {
	cases := []struct {
		host, defNS string
		name, ns    string
		ok          bool
	}{
		{"e2e-postgres.ztwim.svc", "ztwim", "e2e-postgres", "ztwim", true},
		{"e2e-postgres.ztwim.svc.cluster.local", "ztwim", "e2e-postgres", "ztwim", true},
		{"postgres", "ztwim", "postgres", "ztwim", true},
		{"db.example.com", "ztwim", "", "", false},
	}
	for _, c := range cases {
		name, ns, ok := parseKubernetesServiceHost(c.host, c.defNS)
		if name != c.name || ns != c.ns || ok != c.ok {
			t.Fatalf("parseKubernetesServiceHost(%q): got %q %q %v want %q %q %v", c.host, name, ns, ok, c.name, c.ns, c.ok)
		}
	}
}

func TestServiceTargetTCPPort(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-postgres"},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Port:       5433,
				TargetPort: intstr.FromInt32(5432),
			}},
		},
	}
	if got := serviceTargetTCPPort(svc, 5433); got != 5432 {
		t.Fatalf("expected target 5432, got %d", got)
	}
}

func TestDatabaseEgressTCPPorts_MultilineLibpqHost(t *testing.T) {
	ns := "zero-trust-workload-identity-manager"
	conn := "dbname=spire user=spire host=e2e-postgres." + ns + ".svc\n      port=5433 sslmode=disable password=spire-e2e"
	stub := &stubServiceGetter{svc: &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-postgres", Namespace: ns},
		Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{
			Port: 5433, TargetPort: intstr.FromInt32(5432),
		}}},
	}}
	ports, ok := DatabaseEgressTCPPorts(context.Background(), stub, ns, "postgres", conn)
	if !ok || len(ports) != 2 || ports[0] != 5433 || ports[1] != 5432 {
		t.Fatalf("expected 5433 and 5432, got %v %v", ports, ok)
	}
}

func TestDatabaseEgressTCPPorts_ExternalHost(t *testing.T) {
	conn := "dbname=spire host=db.example.com port=5433"
	ports, ok := DatabaseEgressTCPPorts(nil, nil, "ns", "postgres", conn)
	if !ok || len(ports) != 1 || ports[0] != 5433 {
		t.Fatalf("expected only dialed port 5433, got %v %v", ports, ok)
	}
}
