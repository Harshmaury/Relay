// @relay-project: relay
// @relay-path: internal/tunnel/conn.go
// Conn manages the handshake lifecycle for one engxa tunnel connection.
// After handshake succeeds, a *Mux is created and registered.
// The Mux handles all subsequent concurrent request forwarding.
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
	Token string `json:"token"`  // X-Relay-Token value
	Owner string `json:"owner"`  // Gate subject or OS username
	Name  string `json:"name"`   // requested subdomain prefix
}

// HandshakeResponse is sent back after successful handshake.
type HandshakeResponse struct {
	OK        bool   `json:"ok"`
	TunnelID  string `json:"tunnel_id,omitempty"`
	Subdomain string `json:"subdomain,omitempty"`
	PublicURL string `json:"public_url,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Handler processes incoming tunnel connections from engxa.
type Handler struct {
	registry   *Registry
	relayToken string
	domain     string
}

// NewHandler creates a tunnel Handler.
func NewHandler(reg *Registry, relayToken, domain string) *Handler {
	return &Handler{registry: reg, relayToken: relayToken, domain: domain}
}

// Handle runs the tunnel handshake and registers the Mux.
// Blocks until the tunnel closes. Safe to call in a goroutine.
func (h *Handler) Handle(conn net.Conn) {
	// Read handshake (10s deadline).
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		sendHandshakeError(conn, "handshake read failed")
		conn.Close()
		return
	}
	conn.SetDeadline(time.Time{}) // clear — tunnel is long-lived

	var req HandshakeRequest
	if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
		sendHandshakeError(conn, "handshake parse error")
		conn.Close()
		return
	}

	if !tokenValid(req.Token, h.relayToken) {
		sendHandshakeError(conn, "invalid relay token")
		conn.Close()
		return
	}
	if req.Owner == "" || req.Name == "" {
		sendHandshakeError(conn, "owner and name are required")
		conn.Close()
		return
	}

	subdomain := fmt.Sprintf("%s.%s", req.Name, req.Owner)
	id := newTunnelID()
	mux := NewMux(conn) // create mux — starts readLoop goroutine

	entry := &Entry{
		ID:           id,
		Owner:        req.Owner,
		Subdomain:    subdomain,
		Mux:          mux,
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

	// Block until the mux closes (engxa disconnects).
	<-mux.done
}

func sendHandshakeError(conn net.Conn, reason string) {
	resp := HandshakeResponse{OK: false, Error: reason}
	b, _ := json.Marshal(resp)
	fmt.Fprintf(conn, "%s\n", b)
}

func tokenValid(presented, expected string) bool {
	if expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(presented), []byte(expected)) == 1
}

func newTunnelID() string {
	return fmt.Sprintf("tun_%d", time.Now().UnixNano()%0xFFFFFF)
}

var _ = config.ServiceName // ensure config package used
