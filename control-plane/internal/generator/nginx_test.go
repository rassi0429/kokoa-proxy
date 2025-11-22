package generator

import (
	"strings"
	"testing"

	"github.com/neo/kokoa-proxy/control-plane/internal/db"
)

func TestBuildConfigDeterministic(t *testing.T) {
	input := []db.RouteWithOrigin{
		{Hostname: "b.example.com", TargetPort: 8080, WireguardIP: "10.0.0.2"},
		{Hostname: "a.example.com", TargetPort: 8081, WireguardIP: "10.0.0.3"},
	}

	first := BuildConfig(input)
	second := BuildConfig(input)
	if first.ConfigHash != second.ConfigHash {
		t.Fatalf("expected deterministic hash, got %s and %s", first.ConfigHash, second.ConfigHash)
	}
	if !strings.Contains(first.Map, "a.example.com 10.0.0.3:8081;") {
		t.Fatalf("missing expected upstream mapping: %s", first.Map)
	}
	if len(first.Hostnames) != 2 || first.Hostnames[0] != "a.example.com" {
		t.Fatalf("hostnames not sorted: %v", first.Hostnames)
	}
}
