// @relay-project: relay
// @relay-path: internal/tunnel/registry.go
// Registry is the in-memory store of active tunnel connections.
// Each entry now holds a *Mux instead of a raw net.Conn,
// enabling safe concurrent HTTP request forwarding (Fix 1 — audit).
package tunnel

import (
	"fmt"
	"sync"
	"time"
)

// Entry is one active tunnel in the registry.
type Entry struct {
	ID           string
	Owner        string // Gate subject or OS username
	Subdomain    string // full subdomain prefix, e.g. "api.harsh"
	Mux          *Mux  // request multiplexer — replaces raw net.Conn
	RegisteredAt time.Time
}

// PublicURL returns the full public HTTPS URL for this tunnel.
func (e *Entry) PublicURL(domain string) string {
	return fmt.Sprintf("https://%s.%s", e.Subdomain, domain)
}

// Registry is a thread-safe in-memory map of active tunnels.
type Registry struct {
	mu    sync.RWMutex
	byID  map[string]*Entry
	bySub map[string]*Entry
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		byID:  make(map[string]*Entry),
		bySub: make(map[string]*Entry),
	}
}

// Register adds a tunnel entry. Replaces any existing entry for the same subdomain.
func (r *Registry) Register(e *Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if old, ok := r.bySub[e.Subdomain]; ok {
		old.Mux.Close()
		delete(r.byID, old.ID)
	}
	r.byID[e.ID] = e
	r.bySub[e.Subdomain] = e
}

// Lookup returns the tunnel entry for the given subdomain, or nil.
func (r *Registry) Lookup(subdomain string) *Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.bySub[subdomain]
}

// Remove removes a tunnel entry by ID and closes its Mux.
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok {
		return
	}
	e.Mux.Close()
	delete(r.byID, e.ID)
	delete(r.bySub, e.Subdomain)
}

// List returns a snapshot of all active tunnel entries.
func (r *Registry) List() []*Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Entry, 0, len(r.byID))
	for _, e := range r.byID {
		out = append(out, e)
	}
	return out
}

// Count returns the number of active tunnels.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byID)
}
