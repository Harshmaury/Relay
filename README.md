# Relay

**Public Endpoint Tunneling Service**  
`role: control` | `version: v0.1.0` | ADR-041

---

## What it does

Relay makes locally running services reachable at stable public HTTPS URLs.

```bash
engx expose atlas
# ✓ atlas.harsh.engx.dev → 127.0.0.1:8081
```

URL format: `https://<name>.<owner>.engx.dev`

---

## Ports

| Port | Purpose |
|------|---------|
| `9090` | Tunnel listener — engxa connects here (outbound TLS from dev machine) |
| `9091` | HTTP router — inbound public requests (behind Cloudflare) |

---

## Connection flow

```
engx expose api
  → engxd instructs engxa to open tunnel to relay.engx.dev:9090
  → engxa presents X-Relay-Token + owner
  → Relay validates token, assigns subdomain api.harsh.engx.dev
  → Inbound: GET https://api.harsh.engx.dev/users
  → Relay :9091 → tunnel conn → engxa → 127.0.0.1:<port>
```

---

## Environment variables

| Variable | Default | Required |
|----------|---------|----------|
| `RELAY_TOKEN` | — | **Yes** — shared secret for tunnel auth |
| `RELAY_TUNNEL_ADDR` | `0.0.0.0:9090` | No |
| `RELAY_HTTP_ADDR` | `0.0.0.0:9091` | No |
| `RELAY_DOMAIN` | `engx.dev` | No |
| `RELAY_SERVICE_TOKEN` | — | Production |
| `NEXUS_ADDR` | `http://127.0.0.1:8080` | No |
| `GATE_ADDR` | `http://127.0.0.1:8088` | No |
| `RELAY_REQUIRE_IDENTITY` | `false` | Phase 2 flips to `true` |

---

## Build

```bash
go build -o relay ./cmd/relay/
RELAY_TOKEN=dev-secret ./relay
```

---

## ADR

[ADR-041 — engx expose: Public Endpoint Tunneling](https://github.com/Harshmaury/engx-governance/blob/main/architecture/decisions/ADR-041-engx-expose-public-endpoint-tunneling.md)
