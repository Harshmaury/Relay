// @relay-project: relay
// @relay-path: internal/tunnel/conn.go
// Conn manages the lifecycle of one engxa tunnel connection.
// ADR-041: engxa opens an outbound TLS connection to Relay :9090.
// Relay validates the relay token, assigns a subdomain, registers the tunnel.
package tunnel

import (
	"bufio"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/Harshmaury/Relay/internal/config"
)

// HandshakeRequest is the JSON message engxa sends on connection.
type HandshakeRequest struct {
	Token  string `json:"token"`   // X-Relay-Token value — validates tunnel ownership
	Owner  string `json:"owner"`   // Gate subject or OS username
	Name   string `json:"name"`    // requested subdomain prefix, e.g. "api"
}

// HandshakeResponse is the JSON message Relay sends back on success.
type HandshakeResponse struct {
	OK        bool   `json:"ok"`
	TunnelID  string `json:"tunnel_id,omitempty"`
	Subdomain string `json:"subdomain,omitempty"` // full: "api.harsh"
	PublicURL string `json:"public_url,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Handler processes a new tunnel connection from engxa.
type Handler struct {
	registry     *Registry
	relayToken   string
	domain       string
}

// NewHandler creates a tunnel Handler.
func NewHandler(reg *Registry, relayToken, domain string) *Handler {
	return &Handler{registry: reg, relayToken: relayToken, domain: domain}
}

// Handle runs the tunnel lifecycle for one engxa connection.
// Blocks until the tunnel closes. Safe to call in a goroutine.
func (h *Handler) Handle(conn net.Conn) {
	defer conn.Close()

	// Read handshake JSON (one line).
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		sendHandshakeError(conn, "handshake read failed")
		return
	}
	conn.SetDeadline(time.Time{}) // clear deadline — tunnel is long-lived

	var req HandshakeRequest
	if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
		sendHandshakeError(conn, "handshake parse error")
		return
	}

	// Validate relay token (constant-time compare — ADR-041).
	if !tokenValid(req.Token, h.relayToken) {
		sendHandshakeError(conn, "invalid relay token")
		return
	}

	if req.Owner == "" || req.Name == "" {
		sendHandshakeError(conn, "owner and name are required")
		return
	}

	// Assign subdomain: "<name>.<owner>"
	subdomain := fmt.Sprintf("%s.%s", req.Name, req.Owner)
	id := newTunnelID()

	entry := &Entry{
		ID:           id,
		Owner:        req.Owner,
		Subdomain:    subdomain,
		Conn:         conn,
		RegisteredAt: time.Now().UTC(),
	}
	h.registry.Register(entry)
	defer h.registry.Remove(id)

	resp := HandshakeResponse{
		OK:        true,
		TunnelID:  id,
		Subdomain: subdomain,
		PublicURL: entry.PublicURL(h.domain),
	}
	respBytes, _ := json.Marshal(resp)
	fmt.Fprintf(conn, "%s\n", respBytes)

	// Keep tunnel alive — wait for connection close.
	// The HTTP router uses the connection directly for request forwarding.
	// A zero-byte read unblocks when engxa closes the connection.
	buf := make([]byte, 1)
	conn.Read(buf) // blocks until engxa disconnects
}

func sendHandshakeError(conn net.Conn, reason string) {
	resp := HandshakeResponse{OK: false, Error: reason}
	b, _ := json.Marshal(resp)
	fmt.Fprintf(conn, "%s\n", b)
}

func tokenValid(presented, expected string) bool {
	if expected == "" {
		return false // relay token is required; empty = always deny
	}
	return subtle.ConstantTimeCompare([]byte(presented), []byte(expected)) == 1
}

// newTunnelID generates a short unique tunnel ID.
func newTunnelID() string {
	b := make([]byte, 6)
	for i := range b {
		b[i] = "abcdefghijklmnopqrstuvwxyz0123456789"[time.Now().UnixNano()%36]
		time.Sleep(1) // jitter — production replace with crypto/rand
	}
	return fmt.Sprintf("tun_%x", b)
}

// Ensure config package is used (for canon.go constants via config.RelayTokenHeader etc.)
var _ = config.ServiceName
