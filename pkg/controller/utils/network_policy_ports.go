package utils

import (
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// TCPPortFromURL returns the TCP port for egress NetworkPolicy rules derived from a URL.
// Uses explicit port when present; otherwise https→443, http→80, unknown scheme→0.
func TCPPortFromURL(raw string) int32 {
	if raw == "" {
		return 0
	}
	u, err := url.Parse(raw)
	if err != nil {
		return 0
	}
	if u.Port() != "" {
		if p, err := strconv.ParseInt(u.Port(), 10, 32); err == nil && p > 0 {
			return int32(p)
		}
		return 0
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return 443
	case "http":
		return 80
	default:
		return 0
	}
}

func databaseEgressDefaultPort(databaseType string) int32 {
	switch strings.ToLower(databaseType) {
	case "postgres", "aws_postgresql", "sql":
		return 5432
	case "mysql", "aws_mysql":
		return 3306
	default:
		return 0
	}
}

func databaseTypeRequiresEgress(databaseType string) bool {
	switch strings.ToLower(databaseType) {
	case "", "sqlite3":
		return false
	case "postgres", "aws_postgresql", "sql", "mysql", "aws_mysql":
		return true
	default:
		return false
	}
}

func parsePositivePort(portStr string) int32 {
	if portStr == "" {
		return 0
	}
	p, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil || p <= 0 {
		return 0
	}
	return int32(p)
}

// portFromPostgresURL extracts an explicit TCP port from a postgres/postgresql URL.
func portFromPostgresURL(raw string) int32 {
	u, err := url.Parse(raw)
	if err != nil {
		return 0
	}
	switch strings.ToLower(u.Scheme) {
	case "postgres", "postgresql":
	default:
		return 0
	}
	if u.Port() != "" {
		return parsePositivePort(u.Port())
	}
	return 0
}

// portFromMySQLDSN extracts TCP port from DSN forms like user:pass@tcp(host:3307)/db.
func portFromMySQLDSN(raw string) int32 {
	const tcpPrefix = "tcp("
	idx := strings.Index(raw, tcpPrefix)
	if idx < 0 {
		return 0
	}
	start := idx + len(tcpPrefix)
	end := strings.Index(raw[start:], ")")
	if end < 0 {
		return 0
	}
	hostPort := raw[start : start+end]
	if !strings.Contains(hostPort, ":") {
		return 0
	}
	_, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		return 0
	}
	return parsePositivePort(portStr)
}

// portFromLibpqConnectionString reads port=N from space-separated key=value pairs.
func portFromLibpqConnectionString(raw string) int32 {
	for _, field := range strings.Fields(raw) {
		key, val, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		if strings.EqualFold(key, "port") {
			return parsePositivePort(val)
		}
	}
	return 0
}

// explicitPortFromConnectionString returns a positive port when present in connectionString.
func explicitPortFromConnectionString(databaseType, connectionString string) int32 {
	raw := strings.TrimSpace(connectionString)
	if raw == "" {
		return 0
	}
	if strings.Contains(raw, "://") {
		if p := portFromPostgresURL(raw); p > 0 {
			return p
		}
		if p := TCPPortFromURL(raw); p > 0 {
			return p
		}
	}
	if p := portFromMySQLDSN(raw); p > 0 {
		return p
	}
	return portFromLibpqConnectionString(raw)
}

// DatabaseEgressPort returns the TCP port for database egress NetworkPolicy rules.
// It parses connectionString for an explicit port; otherwise uses the default for databaseType.
func DatabaseEgressPort(databaseType, connectionString string) (int32, bool) {
	if !databaseTypeRequiresEgress(databaseType) {
		return 0, false
	}
	if p := explicitPortFromConnectionString(databaseType, connectionString); p > 0 {
		return p, true
	}
	def := databaseEgressDefaultPort(databaseType)
	if def == 0 {
		return 0, false
	}
	return def, true
}

// VaultEgressPort returns the TCP port for Vault upstream authority egress.
func VaultEgressPort(vaultAddr string) int32 {
	u, err := url.Parse(vaultAddr)
	if err != nil {
		return 443
	}
	if u.Port() != "" {
		return TCPPortFromURL(vaultAddr)
	}
	if strings.EqualFold(u.Scheme, "http") {
		return 8200
	}
	return 443
}

// ProxyEgressTCPPorts collects unique proxy ports from operator HTTP(S)_PROXY env vars.
func ProxyEgressTCPPorts() []int32 {
	seen := make(map[int32]struct{})
	var ports []int32
	for _, raw := range []string{os.Getenv(HTTPProxyEnvVar), os.Getenv(HTTPSProxyEnvVar)} {
		if raw == "" {
			continue
		}
		p := TCPPortFromURL(raw)
		if p == 0 {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		ports = append(ports, p)
	}
	return ports
}

// ParseHostPort is used by tests to validate URL parsing helpers.
func ParseHostPort(raw string) (string, int32, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", 0, err
	}
	host := u.Hostname()
	port := TCPPortFromURL(raw)
	if u.Port() != "" {
		return host, port, nil
	}
	_, _, err = net.SplitHostPort(u.Host)
	if err == nil {
		return host, port, nil
	}
	return host, port, nil
}
