// @relay-project: relay
// @relay-path: internal/router/http.go
// HTTP router — receives inbound public requests and forwards through tunnel Mux.
// Each request is independently framed — safe for concurrent callers.
package router

import (
	canonid "github.com/Harshmaury/Canon/identity"
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

	r.Header.Set(canonid.SubdomainHeader, sub)
	r.Header.Set(canonid.OwnerHeader, entry.Owner)

	// DEF-009 fix: propagate inbound trace ID through the tunnel (ADR-045).
	// req.Write() in muxTransport.RoundTrip() serializes all request headers,
	// so setting the header here guarantees it arrives at the local service.
	if traceID := r.Header.Get(canonid.TraceIDHeader); traceID != "" {
		r.Header.Set(canonid.TraceIDHeader, traceID)
	}

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
	// All headers set on req (including X-Trace-ID) are included by req.Write().
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

// modeResponse mirrors the Nexus /system/mode Accord envelope shape.
// Using a named type (not anonymous struct) makes decode failures detectable
// rather than silently producing an empty Mode string.
type modeResponse struct {
	OK   bool `json:"ok"`
	Data struct {
		Mode string `json:"mode"`
	} `json:"data"`
}

// checkMode probes GET /system/mode on Nexus (ADR-044).
//
// Returns (blocked=true, reason) when the platform is in insecure mode.
// Returns (blocked=true, reason) when the mode cannot be determined — fail-safe:
// an unreadable mode response is treated as unknown, not safe.
// Returns (blocked=false, "") when the platform is in a valid mode.
//
// DEF-005 fix: decode failure now returns blocked=true instead of false.
// A misconfigured or changed envelope must not silently permit forwarding.
func (h *Handler) checkMode() (bool, string) {
	req, err := http.NewRequest(http.MethodGet, h.nexusAddr+"/system/mode", nil)
	if err != nil {
		// Request construction failure — treat as unknown mode (fail-safe).
		return true, "mode check failed: could not build request"
	}
	if h.serviceToken != "" {
		req.Header.Set(canonid.ServiceTokenHeader, h.serviceToken)
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Nexus unreachable — do NOT block: Relay should not gate on Nexus availability.
		// Nexus downtime should not take down public tunnel routing.
		return false, ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Non-200 means auth failure (401) or Nexus error (5xx).
		// We can't determine the mode — fail-safe: block until we can.
		return true, fmt.Sprintf("mode check failed: Nexus returned HTTP %d", resp.StatusCode)
	}

	var body modeResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || !body.OK {
		// Decode failure means the envelope changed or is malformed.
		// Fail-safe: block rather than silently permit forwarding in unknown mode.
		return true, "mode check failed: could not decode Nexus response"
	}

	if body.Data.Mode == mode.ModeInsecure {
		return true, "identity capability disabled — start Gate before exposing services"
	}
	return false, ""
}
