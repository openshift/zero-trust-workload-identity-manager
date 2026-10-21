package utils

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type kubernetesServiceGetter interface {
	Get(ctx context.Context, key client.ObjectKey, obj client.Object) error
}

// DatabaseEgressTCPPorts returns TCP ports for database egress NetworkPolicy rules.
// It always includes the dialed port from connectionString. For in-cluster Services,
// it also includes the Service targetPort when it differs (OVN enforces on pod port after NAT).
func normalizeConnectionString(raw string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
}

func DatabaseEgressTCPPorts(ctx context.Context, c kubernetesServiceGetter, operatorNamespace, databaseType, connectionString string) ([]int32, bool) {
	if !databaseTypeRequiresEgress(databaseType) {
		return nil, false
	}
	connectionString = normalizeConnectionString(connectionString)
	dialed := explicitPortFromConnectionString(databaseType, connectionString)
	if dialed == 0 {
		dialed = databaseEgressDefaultPort(databaseType)
	}
	if dialed == 0 {
		return nil, false
	}

	ports := []int32{dialed}
	host := hostFromConnectionString(connectionString)
	if host == "" || c == nil {
		return uniqueInt32s(ports), true
	}

	svcName, svcNS, isSvcHost := parseKubernetesServiceHost(host, operatorNamespace)
	if !isSvcHost {
		return uniqueInt32s(ports), true
	}

	svc := &corev1.Service{}
	if err := c.Get(ctx, client.ObjectKey{Name: svcName, Namespace: svcNS}, svc); err != nil {
		// OVN matches Service targetPort; if lookup fails, allow default DB port for in-cluster Service DNS.
		appendDefaultDBTargetPort(&ports, databaseType, dialed)
		return uniqueInt32s(ports), true
	}

	target := serviceTargetTCPPort(svc, dialed)
	if target > 0 && target != dialed {
		ports = append(ports, target)
	} else if target == 0 {
		appendDefaultDBTargetPort(&ports, databaseType, dialed)
	}
	return uniqueInt32s(ports), true
}

func appendDefaultDBTargetPort(ports *[]int32, databaseType string, dialed int32) {
	def := databaseEgressDefaultPort(databaseType)
	if def > 0 && def != dialed {
		*ports = append(*ports, def)
	}
}

func hostFromConnectionString(connectionString string) string {
	raw := strings.TrimSpace(connectionString)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		host, _, err := ParseHostPort(raw)
		if err != nil {
			return ""
		}
		return host
	}
	if host := hostFromMySQLDSN(raw); host != "" {
		return host
	}
	return hostFromLibpqConnectionString(raw)
}

func hostFromLibpqConnectionString(raw string) string {
	for _, field := range strings.Fields(raw) {
		key, val, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		if strings.EqualFold(key, "host") {
			return val
		}
	}
	return ""
}

func hostFromMySQLDSN(raw string) string {
	const tcpPrefix = "tcp("
	idx := strings.Index(raw, tcpPrefix)
	if idx < 0 {
		return ""
	}
	start := idx + len(tcpPrefix)
	end := strings.Index(raw[start:], ")")
	if end < 0 {
		return ""
	}
	hostPort := raw[start : start+end]
	if i := strings.LastIndex(hostPort, ":"); i >= 0 {
		return hostPort[:i]
	}
	return hostPort
}

// parseKubernetesServiceHost returns a Service name and namespace when host looks like
// an in-cluster Service DNS name or a short name in the operator namespace.
func parseKubernetesServiceHost(host, defaultNamespace string) (name, namespace string, ok bool) {
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return "", "", false
	}

	if strings.HasSuffix(host, ".svc.cluster.local") {
		host = strings.TrimSuffix(host, ".svc.cluster.local")
		parts := strings.Split(host, ".")
		if len(parts) < 2 {
			return "", "", false
		}
		return parts[0], parts[1], true
	}

	parts := strings.Split(host, ".")
	if len(parts) >= 3 && parts[2] == "svc" {
		return parts[0], parts[1], true
	}

	if !strings.Contains(host, ".") {
		return host, defaultNamespace, true
	}

	return "", "", false
}

func serviceTargetTCPPort(svc *corev1.Service, dialedPort int32) int32 {
	if svc == nil {
		return 0
	}
	var match *corev1.ServicePort
	for i := range svc.Spec.Ports {
		p := &svc.Spec.Ports[i]
		if p.Port == dialedPort {
			match = p
			break
		}
	}
	if match == nil && len(svc.Spec.Ports) == 1 {
		match = &svc.Spec.Ports[0]
	}
	if match == nil {
		return 0
	}
	return intstrTargetTCPPort(match.TargetPort)
}

func intstrTargetTCPPort(target intstr.IntOrString) int32 {
	if target.Type == intstr.Int || (target.Type == 0 && target.StrVal == "" && target.IntVal > 0) {
		return target.IntVal
	}
	if target.Type == intstr.String || target.StrVal != "" {
		if p := parsePositivePort(target.StrVal); p > 0 {
			return p
		}
	}
	// Named targetPort without Pod context: cannot resolve; caller keeps dialed port only.
	return 0
}

func uniqueInt32s(in []int32) []int32 {
	seen := make(map[int32]struct{}, len(in))
	out := make([]int32, 0, len(in))
	for _, p := range in {
		if p <= 0 {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}
