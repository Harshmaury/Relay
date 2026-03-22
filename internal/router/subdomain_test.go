// @relay-project: relay
// @relay-path: internal/router/subdomain_test.go
package router

import (
	"net/http"
	"testing"
)

func TestExtractSubdomain(t *testing.T) {
	tests := []struct {
		host   string
		domain string
		want   string
	}{
		{"api.harsh.engx.dev", "engx.dev", "api.harsh"},
		{"forge.harsh.engx.dev", "engx.dev", "forge.harsh"},
		{"api.harsh.engx.dev:9091", "engx.dev", "api.harsh"}, // with port
		{"other.site.com", "engx.dev", ""},                   // wrong domain
		{"engx.dev", "engx.dev", ""},                          // no subdomain
		{"api.harsh.engx.dev", "engx.io", ""},                 // wrong TLD
	}
	for _, tt := range tests {
		r := &http.Request{Host: tt.host}
		got := ExtractSubdomain(r, tt.domain)
		if got != tt.want {
			t.Errorf("host=%q domain=%q: got %q, want %q", tt.host, tt.domain, got, tt.want)
		}
	}
}
