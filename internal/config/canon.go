// DEPRECATED: replaced by github.com/Harshmaury/Canon v1.0.0 imports.
// This file will be deleted after go mod tidy confirms Canon is in go.sum.
// Do not add new constants here — use Canon directly.
// @relay-project: relay
// @relay-path: internal/config/canon.go
// Canonical header and address constants for Relay.
// These mirror github.com/Harshmaury/Canon/identity exactly.
// TODO: once `go get github.com/Harshmaury/Canon@v1.0.0` runs on this machine,
//       replace usages of this file with direct Canon imports and delete this file.
//       Canon source: Canon/identity/identity.go (ADR-016, ADR-041, ADR-042, ADR-045)
package config

// Header constants — ADR-041, ADR-016. Must match Canon exactly.
const (
	RelayTokenHeader    = "X-Relay-Token"     // authenticates engxa tunnel connection
	SubdomainHeader     = "X-Engx-Subdomain"  // subdomain prefix assigned to this tunnel
	OwnerHeader         = "X-Engx-Owner"      // owner identifier (e.g. "harsh")
	ServiceTokenHeader  = "X-Service-Token"   // inter-service mesh auth (ADR-008)
	TraceIDHeader       = "X-Trace-ID"        // distributed trace propagation (ADR-015)
	IdentityTokenHeader = "X-Identity-Token"  // Gate-issued actor identity (ADR-042)
)

// Service name constants — Canon/identity.ServiceRelay.
const ServiceName = "relay"

// Default addresses — Canon/identity.
const (
	DefaultTunnelListenAddr = "0.0.0.0:9090"   // tunnel listener — engxa connects here
	DefaultHTTPListenAddr   = "0.0.0.0:9091"   // HTTP router — inbound public requests
	DefaultNexusAddr        = "http://127.0.0.1:8080"
	DefaultGateAddr         = "http://127.0.0.1:8088"
)
