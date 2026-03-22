// @relay-project: relay
// @relay-path: internal/router/http.go
// HTTP router — receives inbound public requests and forwards through tunnel Mux.
// Each request is independently framed — safe for concurrent callers (Fix 1).
package router

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/Harshmaury/Relay/internal/config"
	"github.com/Harshmaury/Relay/internal/mode"
	"github.com/Harshmaury/Relay/internal/tunnel"
)

// Handler routes inbound public HTTP requests to tunnel Mux entries.
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

	r.Header.Set(config.SubdomainHeader, sub)
	r.Header.Set(config.OwnerHeader, entry.Owner)

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = "tunnel"
		},
		Transport: &muxTransport{entry: entry},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, fmt.Sprintf("502 tunnel error: %v", err), http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r)
}

// muxTransport implements http.RoundTripper using the tunnel Mux.
// Each RoundTrip call is independently framed — fully concurrent-safe.
type muxTransport struct {
	entry *tunnel.Entry
}

func (t *muxTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Serialise the HTTP request to bytes.
	var buf bytes.Buffer
	if err := req.Write(&buf); err != nil {
		return nil, fmt.Errorf("serialise request: %w", err)
	}

	// Send through mux — blocks until response frame received.
	respBytes, err := t.entry.Mux.RoundTrip(buf.Bytes())
	if err != nil {
		return nil, err
	}

	// Deserialise the HTTP response.
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(respBytes)), req)
	if err != nil {
		return nil, fmt.Errorf("deserialise response: %w", err)
	}
	return resp, nil
}

// checkMode probes GET /system/mode on Nexus (ADR-044).
func (h *Handler) checkMode() (bool, string) {
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest(http.MethodGet, h.nexusAddr+"/system/mode", nil)
	if err != nil {
		return false, ""
	}
	if h.serviceToken != "" {
		req.Header.Set(config.ServiceTokenHeader, h.serviceToken)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()
	var body struct {
		Data struct{ Mode string `json:"mode"` } `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, ""
	}
	if body.Data.Mode == mode.ModeInsecure {
		return true, "identity capability disabled — start Gate before exposing services"
	}
	return false, ""
}
