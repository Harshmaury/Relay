# SERVICE-CONTRACT.md — Relay

**Role:** control  
**Version:** v0.1.0  
**ADR:** ADR-041  
**Ports:** 9090 (tunnel listener), 9091 (HTTP router)  
**Module:** `github.com/Harshmaury/Relay`

---

## Tunnel protocol (port 9090)

engxa connects with a JSON handshake line:

```json
{"token":"<relay-token>","owner":"harsh","name":"api"}
```

Relay responds:

```json
{"ok":true,"tunnel_id":"tun_abc123","subdomain":"api.harsh","public_url":"https://api.harsh.engx.dev"}
```

On error:

```json
{"ok":false,"error":"invalid relay token"}
```

Connection stays open. Relay writes forwarded HTTP requests; engxa reads and
proxies to the local service. Response travels back over the same connection.

## HTTP router (port 9091)

All inbound requests to `*.engx.dev` land here via Cloudflare.

Relay extracts the subdomain from the `Host` header, looks up the tunnel
in the registry, and forwards the request byte-for-byte over the tunnel connection.

Headers added by Relay on every forwarded request:

| Header | Value |
|--------|-------|
| `X-Engx-Subdomain` | full subdomain prefix, e.g. `api.harsh` |
| `X-Engx-Owner` | tunnel owner, e.g. `harsh` |

Headers are Canon constants — never hardcoded.

## ADR-044 mode probe

When `RELAY_REQUIRE_IDENTITY=true`, Relay calls `GET /system/mode` on Nexus
before forwarding any request. If `mode == "insecure"`, returns `503`.

## Invariants

- `RELAY_TOKEN` empty → all tunnel connections denied (no silent acceptance)
- Relay never modifies request/response bodies, methods, or status codes
- Relay never calls write endpoints on Nexus, Forge, Atlas, or any observer (ADR-020)
- Token comparison always uses `crypto/subtle.ConstantTimeCompare` (no timing attacks)
- Reconnecting engxa for the same subdomain closes the previous connection
