// @relay-project: relay
// @relay-path: internal/mode/mode.go
// Package mode mirrors ADR-044 RuntimeMode constants for use in Relay.
// Relay probes GET /system/mode to decide whether to forward inbound requests.
// ADR-041 §Compliance: if identity capability is disabled and
// RELAY_REQUIRE_IDENTITY=true, Relay rejects the connection.
package mode

// RuntimeMode values — mirror ADR-044 Nexus mode package exactly.
const (
	ModeFull      = "full"      // identity enforced, all observers healthy
	ModeDegraded  = "degraded"  // core operational, optional capabilities absent
	ModeInsecure  = "insecure"  // identity capability absent — Gate unreachable
)
