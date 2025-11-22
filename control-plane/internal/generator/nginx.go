package generator

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/neo/kokoa-proxy/control-plane/internal/db"
)

type Route struct {
	Hostname string `json:"hostname"`
	Upstream string `json:"upstream"`
}

type Config struct {
	Map        string  `json:"map"`
	Hostnames  []string `json:"hostnames"`
	Routes     []Route `json:"routes"`
	ConfigHash string  `json:"config_hash"`
}

// BuildConfig creates an nginx map stanza and deterministic route list.
func BuildConfig(routes []db.RouteWithOrigin) Config {
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Hostname < routes[j].Hostname
	})

	var sb strings.Builder
	sb.WriteString("map $host $kokoa_backend {\n    default \"\";\n")

	outRoutes := make([]Route, 0, len(routes))
	hostnames := make([]string, 0, len(routes))
	for _, r := range routes {
		upstream := fmt.Sprintf("%s:%d", r.WireguardIP, r.TargetPort)
		sb.WriteString(fmt.Sprintf("    %s %s;\n", r.Hostname, upstream))
		outRoutes = append(outRoutes, Route{Hostname: r.Hostname, Upstream: upstream})
		hostnames = append(hostnames, r.Hostname)
	}
	sb.WriteString("}\n")

	hash := sha256.Sum256([]byte(sb.String()))
	return Config{
		Map:        sb.String(),
		Hostnames:  hostnames,
		Routes:     outRoutes,
		ConfigHash: fmt.Sprintf("%x", hash[:]),
	}
}
