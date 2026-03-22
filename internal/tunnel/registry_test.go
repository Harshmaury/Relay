// @relay-project: relay
// @relay-path: internal/tunnel/registry_test.go
package tunnel

import (
	"net"
	"testing"
	"time"
)

// fakeConn is a minimal net.Conn for testing.
type fakeConn struct{ closed bool }

func (f *fakeConn) Close() error                       { f.closed = true; return nil }
func (f *fakeConn) Read(b []byte) (int, error)         { return 0, nil }
func (f *fakeConn) Write(b []byte) (int, error)        { return len(b), nil }
func (f *fakeConn) LocalAddr() net.Addr                { return nil }
func (f *fakeConn) RemoteAddr() net.Addr               { return nil }
func (f *fakeConn) SetDeadline(t time.Time) error      { return nil }
func (f *fakeConn) SetReadDeadline(t time.Time) error  { return nil }
func (f *fakeConn) SetWriteDeadline(t time.Time) error { return nil }

func makeEntry(id, owner, sub string) *Entry {
	return &Entry{
		ID:           id,
		Owner:        owner,
		Subdomain:    sub,
		Conn:         &fakeConn{},
		RegisteredAt: time.Now(),
	}
}

func TestRegistry_RegisterAndLookup(t *testing.T) {
	r := NewRegistry()
	e := makeEntry("tun_001", "harsh", "api.harsh")
	r.Register(e)

	got := r.Lookup("api.harsh")
	if got == nil {
		t.Fatal("expected entry, got nil")
	}
	if got.ID != "tun_001" {
		t.Errorf("wrong ID: %s", got.ID)
	}
}

func TestRegistry_LookupMiss(t *testing.T) {
	r := NewRegistry()
	if r.Lookup("nonexistent.sub") != nil {
		t.Error("expected nil for unknown subdomain")
	}
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry()
	conn := &fakeConn{}
	e := &Entry{ID: "tun_002", Owner: "harsh", Subdomain: "forge.harsh", Conn: conn, RegisteredAt: time.Now()}
	r.Register(e)
	r.Remove("tun_002")

	if r.Lookup("forge.harsh") != nil {
		t.Error("expected nil after remove")
	}
	if !conn.closed {
		t.Error("expected connection to be closed on remove")
	}
}

func TestRegistry_ReRegisterClosesOld(t *testing.T) {
	r := NewRegistry()
	old := &fakeConn{}
	r.Register(&Entry{ID: "tun_a", Owner: "harsh", Subdomain: "api.harsh", Conn: old, RegisteredAt: time.Now()})

	// Re-register same subdomain — old connection should be closed.
	newConn := &fakeConn{}
	r.Register(&Entry{ID: "tun_b", Owner: "harsh", Subdomain: "api.harsh", Conn: newConn, RegisteredAt: time.Now()})

	if !old.closed {
		t.Error("expected old connection to be closed on re-register")
	}
	got := r.Lookup("api.harsh")
	if got == nil || got.ID != "tun_b" {
		t.Errorf("expected new entry, got %v", got)
	}
}

func TestRegistry_Count(t *testing.T) {
	r := NewRegistry()
	if r.Count() != 0 {
		t.Error("expected empty registry")
	}
	r.Register(makeEntry("t1", "a", "a.a"))
	r.Register(makeEntry("t2", "b", "b.b"))
	if r.Count() != 2 {
		t.Errorf("expected 2, got %d", r.Count())
	}
}

func TestEntry_PublicURL(t *testing.T) {
	e := &Entry{Subdomain: "api.harsh"}
	got := e.PublicURL("engx.dev")
	want := "https://api.harsh.engx.dev"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
