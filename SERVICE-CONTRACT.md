// @relay-project: relay
// @relay-path: SERVICE-CONTRACT.md
# SERVICE-CONTRACT.md — Relay
# @version: 0.1.0
# @updated: 2026-03-25

**Ports:** 9090 (tunnel listener) · 9091 (HTTP router) · **Domain:** Control

---

## Code

```
cmd/relay/main.go              startup, goroutine lifecycle
internal/tunnel/registry.go    in-memory tunnel registry (sync.RWMutex)
internal/tunnel/conn.go        TLS tunnel lifecycle, validateToken()
internal/tunnel/mux.go         TCP multiplexer — monotonic counter for request IDs
internal/router/subdomain.go   Host header → tunnel lookup
internal/router/http.go        HTTP reverse proxy through tunnel
internal/auth/token.go         constant-time token comparison
```

---

## Contract

### Tunnel handshake (port 9090)

engxa → Relay (JSON line):
```json
{"token":"<relay-token>","owner":"harsh","name":"api"}
```

Relay → engxa:
```json
{"ok":true,"tunnel_id":"tun_abc123","subdomain":"api.harsh","public_url":"https://api.harsh.engx.dev"}
```
or
```json
{"ok":false,"error":"invalid relay token"}
```

### Headers added to every forwarded request

| Header | Value |
|--------|-------|
| `X-Engx-Subdomain` | `Canon/identity.SubdomainHeader` — e.g. `api.harsh` |
| `X-Engx-Owner` | `Canon/identity.OwnerHeader` — e.g. `harsh` |

### Token validation modes

| Mode | Condition | Behavior |
|------|-----------|----------|
| Gate JWT | `RELAY_GATE_ADDR` set | `POST /gate/validate` — owner from `sub` claim |
| Legacy HMAC | `RELAY_GATE_ADDR` unset | compare against `RELAY_TOKEN` |

---

## Control

**Reconnect:** engxa reconnecting for same subdomain closes the previous connection immediately.

**Request ID:** `nextID()` uses `atomic.Uint32.Add(1)` — monotonically increasing, zero collision risk under concurrent load (CW-6 fix).

**Mode probe:** if `RELAY_REQUIRE_IDENTITY=true`, calls `GET /system/mode` on Nexus before forwarding. Returns `503` if `mode == "insecure"`.

---

## Context

- Never modifies request/response bodies, methods, or status codes
- Never calls write endpoints on any platform service (ADR-020)
- `RELAY_TOKEN` empty → all tunnel connections denied (no silent acceptance)
- Token comparison: `crypto/subtle.ConstantTimeCompare` always
