// @relay-project: relay
// @relay-path: internal/tunnel/mux.go
// Mux provides per-request isolation over a single tunnel net.Conn.
//
// Problem (ADR-047 audit): the original design shared one net.Conn per
// tunnel. Concurrent inbound requests wrote interleaved bytes to the same
// connection, corrupting the HTTP stream for all requests.
//
// Fix: a framing protocol with per-request IDs.
//
// Frame format (all big-endian):
//   [4 bytes: requestID] [4 bytes: bodyLen] [bodyLen bytes: body]
//
// Relay writes request frames. engxa reads them, processes each independently,
// and writes response frames with the matching requestID.
// Relay matches response frames back to waiting goroutines via a channel map.
//
// This makes Relay safe for concurrent inbound HTTP requests on any subdomain.
package tunnel

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	frameHeaderSize = 8 // 4-byte requestID + 4-byte bodyLen
	maxFrameBody    = 32 * 1024 * 1024 // 32 MB max request/response
)

// Mux multiplexes concurrent HTTP requests over one net.Conn.
// It is the replacement for direct conn usage in tunnelTransport.
type Mux struct {
	conn     net.Conn
	mu       sync.Mutex           // serialises frame writes
	pending  map[uint32]chan []byte // requestID → response channel
	pendingMu sync.RWMutex
	closed   atomic.Bool
	closeOnce sync.Once
	done     chan struct{}
	counter  atomic.Uint32 // CW-6: monotonic per-Mux request ID counter
}

// NewMux creates a Mux wrapping conn and starts the read loop.
func NewMux(conn net.Conn) *Mux {
	m := &Mux{
		conn:    conn,
		pending: make(map[uint32]chan []byte),
		done:    make(chan struct{}),
	}
	go m.readLoop()
	return m
}

// RoundTrip sends an HTTP request body through the tunnel and returns
// the response body. Safe for concurrent callers.
func (m *Mux) RoundTrip(reqBytes []byte) ([]byte, error) {
	if m.closed.Load() {
		return nil, fmt.Errorf("tunnel closed")
	}

	reqID := m.nextID()
	ch := make(chan []byte, 1)

	m.pendingMu.Lock()
	m.pending[reqID] = ch
	m.pendingMu.Unlock()

	defer func() {
		m.pendingMu.Lock()
		delete(m.pending, reqID)
		m.pendingMu.Unlock()
	}()

	if err := m.writeFrame(reqID, reqBytes); err != nil {
		return nil, fmt.Errorf("tunnel write: %w", err)
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("tunnel closed while waiting for response")
		}
		return resp, nil
	case <-m.done:
		return nil, fmt.Errorf("tunnel closed")
	case <-time.After(60 * time.Second):
		return nil, fmt.Errorf("tunnel response timeout")
	}
}

// Close closes the underlying connection and unblocks all pending requests.
func (m *Mux) Close() {
	m.closeOnce.Do(func() {
		m.closed.Store(true)
		m.conn.Close()
		close(m.done)
		// Drain pending channels so goroutines unblock.
		m.pendingMu.Lock()
		for id, ch := range m.pending {
			close(ch)
			delete(m.pending, id)
		}
		m.pendingMu.Unlock()
	})
}

// readLoop reads response frames from the tunnel connection and
// delivers each to the waiting RoundTrip goroutine.
func (m *Mux) readLoop() {
	defer m.Close()
	header := make([]byte, frameHeaderSize)
	for {
		if _, err := io.ReadFull(m.conn, header); err != nil {
			return // connection closed or broken
		}
		reqID := binary.BigEndian.Uint32(header[0:4])
		bodyLen := binary.BigEndian.Uint32(header[4:8])
		if bodyLen > maxFrameBody {
			return // protocol violation
		}
		body := make([]byte, bodyLen)
		if _, err := io.ReadFull(m.conn, body); err != nil {
			return
		}
		m.pendingMu.RLock()
		ch, ok := m.pending[reqID]
		m.pendingMu.RUnlock()
		if ok {
			select {
			case ch <- body:
			default: // already timed out
			}
		}
	}
}

// writeFrame writes one framed message to the tunnel connection.
// Mutex-protected — safe for concurrent callers.
func (m *Mux) writeFrame(reqID uint32, body []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	header := make([]byte, frameHeaderSize)
	binary.BigEndian.PutUint32(header[0:4], reqID)
	binary.BigEndian.PutUint32(header[4:8], uint32(len(body)))
	if _, err := m.conn.Write(header); err != nil {
		return err
	}
	_, err := m.conn.Write(body)
	return err
}

// nextID returns a monotonically increasing, per-Mux request ID.
// CW-6: replaced math/rand.Uint32() — rand had birthday-paradox collision
// risk under concurrent load and was not per-Mux isolated.
// Atomic counter guarantees uniqueness within a Mux lifetime.
// Wraps at uint32 max (~4B requests) — safe for any realistic tunnel.
// Non-zero guaranteed: Add(1) starts from 1.
func (m *Mux) nextID() uint32 {
	return m.counter.Add(1)
}
