// @relay-project: relay
// @relay-path: internal/router/http.go
// HTTP router — receives inbound public requests and forwards through tunnel.
// ADR-041: requests are forwarded byte-for-byte; headers/methods/bodies unchanged.
// ADR-044: Relay probes GET /system/mode before forwarding when RELAY_REQUIRE_IDENTITY=true.
package router

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/Harshmaury/Relay/internal/config"
	"github.com/Harshmaury/Relay/internal/mode"
	"github.com/Harshmaury/Relay/internal/tunnel"
)

// Handler routes inbound public HTTP requests to tunnel connections.
type Handler struct {
	registry        *tunnel.Registry
	domain          string
	nexusAddr       string
	serviceToken    string
	requireIdentity bool
}

// NewHandler creates an HTTP routing handler.
func NewHandler(reg *tunnel.Registry, cfg *config.Config) *Handler {
	return &Handler{
		registry:        reg,
		domain:          cfg.PlatformDomain,
		nexusAddr:       cfg.NexusAddr,
		serviceToken:    cfg.ServiceToken,
		requireIdentity: cfg.RequireIdentity,
	}
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.requireIdentity {
		if blocked, reason := h.checkMode(); blocked {
			http.Error(w, "503 "+reason, http.StatusServiceUnavailable)
			return
		}
	}

	sub := ExtractSubdomain(r, h.domain)
	if sub == "" {
		http.Error(w, "400 missing or invalid subdomain", http.StatusBadRequest)
		return
	}

	entry := h.registry.Lookup(sub)
	if entry == nil {
		http.Error(w, fmt.Sprintf("404 tunnel not found: %s", sub), http.StatusNotFound)
		return
	}

	// Attach tunnel metadata headers (ADR-041, Canon constants).
	r.Header.Set(config.SubdomainHeader, sub)
	r.Header.Set(config.OwnerHeader, entry.Owner)

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = "tunnel"
		},
		Transport: &tunnelTransport{entry: entry},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, fmt.Sprintf("502 tunnel error: %v", err), http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r)
}

// tunnelTransport routes HTTP requests through the tunnel net.Conn.
type tunnelTransport struct {
	entry *tunnel.Entry
}

func (t *tunnelTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	conn := t.entry.Conn
	if err := req.Write(conn); err != nil {
		return nil, fmt.Errorf("tunnel write: %w", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return nil, fmt.Errorf("tunnel read: %w", err)
	}
	return resp, nil
}

// checkMode probes GET /system/mode on Nexus (ADR-044).
// Returns (true, reason) if the request should be blocked.
func (h *Handler) checkMode() (bool, string) {
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest(http.MethodGet, h.nexusAddr+"/system/mode", nil)
	if err != nil {
		return false, "" // fail-open
	}
	if h.serviceToken != "" {
		req.Header.Set(config.ServiceTokenHeader, h.serviceToken)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, "" // fail-open if Nexus unreachable
	}
	defer resp.Body.Close()

	var body struct {
		Data struct {
			Mode string `json:"mode"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, ""
	}
	if body.Data.Mode == mode.ModeInsecure {
		return true, "identity capability disabled — start Gate before exposing services"
	}
	return false, ""
}
