# WORKFLOW-SESSION.md — Relay

**Role:** control — public endpoint tunneling  
**ADR:** ADR-041  
**Version:** v0.1.0  
**Repo:** github.com/Harshmaury/Relay  
**Local path:** ~/workspace/projects/engx/services/relay

---

## Start of session checklist

```bash
cd ~/workspace/projects/engx/services/relay
git pull
go build ./...
go test ./...
go vet ./...
```

---

## Running locally (dev)

```bash
export RELAY_TOKEN=dev-secret-local
export RELAY_TUNNEL_ADDR=127.0.0.1:9090
export RELAY_HTTP_ADDR=127.0.0.1:9091
export RELAY_DOMAIN=localhost
go run ./cmd/relay/
```

Then from a second terminal, simulate engxa connecting:

```bash
# Handshake test (raw TCP)
echo '{"token":"dev-secret-local","owner":"harsh","name":"test"}' | nc localhost 9090
```

---

## go.mod setup (first time)

After cloning:

```bash
cd ~/workspace/projects/engx/services/relay
# Edit go.mod — add requires:
go get github.com/Harshmaury/Canon@v1.0.0

# Replace the canon.go shim imports with real Canon imports once go.mod is set:
# Replace: "github.com/Harshmaury/Relay/internal/config".ServiceTokenHeader
# With:    canon "github.com/Harshmaury/Canon/identity"  →  canon.ServiceTokenHeader
```

---

## Phase 2 checklist (RELAY_REQUIRE_IDENTITY)

- [ ] Gate is running and tagged v1.0.0
- [ ] `RELAY_REQUIRE_IDENTITY=true` in production env
- [ ] engxa attaches `X-Identity-Token` when opening tunnel (ADR-042)
- [ ] GateValidator wired into tunnel handshake
- [ ] Guardian G-010 fires when Relay is exposed but Gate is down

---

## Never

- Accept a tunnel connection when `RELAY_TOKEN` env is empty
- Modify request/response bodies, methods, or status codes
- Call write endpoints on any platform service (ADR-020)
- Hardcode header strings — always use Canon constants (ADR-016)
- Open an inbound port on the developer's machine (tunnel is outbound from engxa)
