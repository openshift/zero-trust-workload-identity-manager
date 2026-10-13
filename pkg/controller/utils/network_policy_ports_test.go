package utils

import "testing"

func TestTCPPortFromURL(t *testing.T) {
	cases := map[string]int32{
		"https://vault.example.com:8200": 8200,
		"https://proxy.example.com":      443,
		"http://proxy.example.com:8080":  8080,
		"http://vault.local":             80,
		"":                               0,
	}
	for raw, want := range cases {
		if got := TCPPortFromURL(raw); got != want {
			t.Fatalf("TCPPortFromURL(%q) = %d, want %d", raw, got, want)
		}
	}
}

func TestDatabaseEgressPort(t *testing.T) {
	if _, ok := DatabaseEgressPort("sqlite3", ""); ok {
		t.Fatal("sqlite3 should not require egress")
	}
	p, ok := DatabaseEgressPort("postgres", "dbname=spire host=postgres")
	if !ok || p != 5432 {
		t.Fatalf("postgres default: got %d %v", p, ok)
	}
	p, ok = DatabaseEgressPort("postgres", "dbname=spire host=postgres port=5433")
	if !ok || p != 5433 {
		t.Fatalf("postgres libpq port: got %d %v", p, ok)
	}
	p, ok = DatabaseEgressPort("postgres", "postgresql://user:pass@db.example.com:5433/spire")
	if !ok || p != 5433 {
		t.Fatalf("postgres url port: got %d %v", p, ok)
	}
	p, ok = DatabaseEgressPort("mysql", "user:pass@tcp(localhost:3306)/spire")
	if !ok || p != 3306 {
		t.Fatalf("mysql default in dsn: got %d %v", p, ok)
	}
	p, ok = DatabaseEgressPort("mysql", "user:pass@tcp(db.example.com:3307)/spire?parseTime=true")
	if !ok || p != 3307 {
		t.Fatalf("mysql custom port: got %d %v", p, ok)
	}
	p, ok = DatabaseEgressPort("aws_postgresql", "")
	if !ok || p != 5432 {
		t.Fatalf("aws_postgresql default: got %d %v", p, ok)
	}
}

func TestVaultEgressPort(t *testing.T) {
	if VaultEgressPort("http://vault.ztwim.svc") != 8200 {
		t.Fatal("expected vault http default 8200")
	}
	if VaultEgressPort("https://vault.example.com") != 443 {
		t.Fatal("expected https default 443")
	}
}

func TestProxyEgressTCPPorts(t *testing.T) {
	t.Setenv(HTTPProxyEnvVar, "http://proxy.example.com:3128")
	t.Setenv(HTTPSProxyEnvVar, "https://proxy.example.com:8443")
	ports := ProxyEgressTCPPorts()
	if len(ports) != 2 {
		t.Fatalf("expected 2 ports, got %v", ports)
	}
}
