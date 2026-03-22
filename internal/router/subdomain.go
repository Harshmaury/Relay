// @relay-project: relay
// @relay-path: internal/router/subdomain.go
// Package router maps incoming HTTP requests to tunnel entries.
// ADR-041: subdomain routing — api.harsh.engx.dev → tunnel for "api.harsh".
package router

import (
	"net/http"
	"strings"
)

// ExtractSubdomain returns the subdomain prefix from the request Host header.
// For host "api.harsh.engx.dev" with domain "engx.dev" → returns "api.harsh".
// Returns "" if the host does not match the expected domain suffix.
func ExtractSubdomain(r *http.Request, domain string) string {
	host := r.Host
	// Strip port if present.
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		host = host[:idx]
	}
	suffix := "." + domain
	if !strings.HasSuffix(host, suffix) {
		return ""
	}
	return strings.TrimSuffix(host, suffix)
}
