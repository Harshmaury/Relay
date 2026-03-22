// @relay-project: relay
// @relay-path: internal/tunnel/registry_test.go
package tunnel

import (
	"net"
	"testing"
	"time"
)

func makeMux(t *testing.T) (*Mux, net.Conn) {
	t.Helper()
	server, client := net.Pipe()
	m := NewMux(client)
	return m, server
}

func makeEntry(id, owner, sub string, m *Mux) *Entry {
	return &Entry{ID: id, Owner: owner, Subdomain: sub, Mux: m, RegisteredAt: time.Now()}
}

func TestRegistry_RegisterAndLookup(t *testing.T) {
	r := NewRegistry()
	m, server := makeMux(t)
	defer server.Close()
	r.Register(makeEntry("tun_001", "harsh", "api.harsh", m))
	got := r.Lookup("api.harsh")
	if got == nil || got.ID != "tun_001" {
		t.Fatalf("expected entry tun_001, got %v", got)
	}
}

func TestRegistry_LookupMiss(t *testing.T) {
	r := NewRegistry()
	if r.Lookup("nonexistent") != nil {
		t.Error("expected nil")
	}
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry()
	m, server := makeMux(t)
	defer server.Close()
	r.Register(makeEntry("tun_002", "harsh", "forge.harsh", m))
	r.Remove("tun_002")
	if r.Lookup("forge.harsh") != nil {
		t.Error("expected nil after remove")
	}
}

func TestRegistry_ReRegisterClosesOld(t *testing.T) {
	r := NewRegistry()
	m1, s1 := makeMux(t)
	m2, s2 := makeMux(t)
	defer s1.Close()
	defer s2.Close()
	r.Register(makeEntry("tun_a", "harsh", "api.harsh", m1))
	r.Register(makeEntry("tun_b", "harsh", "api.harsh", m2))
	got := r.Lookup("api.harsh")
	if got == nil || got.ID != "tun_b" {
		t.Errorf("expected tun_b, got %v", got)
	}
}

func TestRegistry_Count(t *testing.T) {
	r := NewRegistry()
	m1, s1 := makeMux(t)
	m2, s2 := makeMux(t)
	defer s1.Close(); defer s2.Close()
	r.Register(makeEntry("t1", "a", "a.a", m1))
	r.Register(makeEntry("t2", "b", "b.b", m2))
	if r.Count() != 2 {
		t.Errorf("expected 2, got %d", r.Count())
	}
}

func TestEntry_PublicURL(t *testing.T) {
	e := &Entry{Subdomain: "api.harsh"}
	if got := e.PublicURL("engx.dev"); got != "https://api.harsh.engx.dev" {
		t.Errorf("got %s", got)
	}
}
