// @relay-project: relay
// @relay-path: internal/config/env.go
// Package config loads all Relay configuration from environment variables.
// ADR-041: all config is from env; no config files.
package config

import (
	"os"

	canonid "github.com/Harshmaury/Canon/identity"
)

// Config holds all Relay runtime configuration.
type Config struct {
	// TunnelAddr is the address Relay listens on for engxa tunnel connections.
	// Environment: RELAY_TUNNEL_ADDR. Default: 0.0.0.0:9090
	TunnelAddr string

	// HTTPAddr is the address Relay listens on for inbound public HTTP requests.
	// Environment: RELAY_HTTP_ADDR. Default: 0.0.0.0:9091
	HTTPAddr string

	// NexusAddr is the Nexus HTTP API address for mode probing (ADR-044).
	// Environment: NEXUS_ADDR. Default: http://127.0.0.1:8080
	NexusAddr string

	// GateAddr is the Gate HTTP API address for token validation (ADR-042).
	// Environment: GATE_ADDR. Default: http://127.0.0.1:8088
	GateAddr string

	// ServiceToken is the platform service token (ADR-008).
	// Environment: RELAY_SERVICE_TOKEN. Required in production.
	ServiceToken string

	// RelayToken is the shared secret engxa presents when opening a tunnel.
	// Environment: RELAY_TOKEN. Required.
	RelayToken string

	// PlatformDomain is the base domain for public tunnel URLs.
	// Environment: RELAY_DOMAIN. Default: engx.dev
	PlatformDomain string

	// RequireIdentity gates inbound requests on Gate identity validation.
	// Environment: RELAY_REQUIRE_IDENTITY. Default: false (Phase 1).
	// Phase 2: this default flips to true.
	RequireIdentity bool
}

// Load reads all configuration from environment with defaults applied.
// DefaultTunnelListenAddr is the local address Relay listens on for tunnel connections.
// Override with RELAY_TUNNEL_ADDR environment variable.
const DefaultTunnelListenAddr = "0.0.0.0:9090"

// DefaultHTTPListenAddr is the local address Relay listens on for inbound HTTP traffic.
// Override with RELAY_HTTP_ADDR environment variable.
const DefaultHTTPListenAddr = "0.0.0.0:8090"

func Load() *Config {
	return &Config{
		TunnelAddr:      envOr("RELAY_TUNNEL_ADDR", DefaultTunnelListenAddr),
		HTTPAddr:        envOr("RELAY_HTTP_ADDR", DefaultHTTPListenAddr),
		NexusAddr:       envOr("NEXUS_ADDR", canonid.DefaultNexusAddr),
		GateAddr:        envOr("GATE_ADDR", canonid.DefaultGateAddr),
		ServiceToken:    os.Getenv("RELAY_SERVICE_TOKEN"),
		RelayToken:      os.Getenv("RELAY_TOKEN"),
		PlatformDomain:  envOr("RELAY_DOMAIN", "engx.dev"),
		RequireIdentity: os.Getenv("RELAY_REQUIRE_IDENTITY") == "true",
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
