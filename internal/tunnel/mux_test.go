// @relay-project: relay
// @relay-path: internal/tunnel/mux_test.go
package tunnel

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
	"encoding/binary"
)

// echoMuxServer simulates engxa: reads framed requests, echoes them back as responses.
func echoMuxServer(conn net.Conn) {
	header := make([]byte, frameHeaderSize)
	for {
		if _, err := readFull(conn, header); err != nil {
			return
		}
		reqID := binary.BigEndian.Uint32(header[0:4])
		bodyLen := binary.BigEndian.Uint32(header[4:8])
		body := make([]byte, bodyLen)
		if _, err := readFull(conn, body); err != nil {
			return
		}
		// Echo back with same reqID
		resp := make([]byte, frameHeaderSize+len(body))
		binary.BigEndian.PutUint32(resp[0:4], reqID)
		binary.BigEndian.PutUint32(resp[4:8], uint32(len(body)))
		copy(resp[8:], body)
		conn.Write(resp)
	}
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		conn.SetDeadline(time.Now().Add(5 * time.Second))
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func TestMux_SingleRequest(t *testing.T) {
	server, client := net.Pipe()
	go echoMuxServer(server)
	m := NewMux(client)
	defer m.Close()

	resp, err := m.RoundTrip([]byte("hello"))
	if err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
	if string(resp) != "hello" {
		t.Errorf("got %q, want %q", resp, "hello")
	}
}

func TestMux_ConcurrentRequests(t *testing.T) {
	server, client := net.Pipe()
	go echoMuxServer(server)
	m := NewMux(client)
	defer m.Close()

	const N = 20
	var wg sync.WaitGroup
	errors := make([]error, N)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			payload := fmt.Sprintf("request-%d", n)
			resp, err := m.RoundTrip([]byte(payload))
			if err != nil {
				errors[n] = err
				return
			}
			if string(resp) != payload {
				errors[n] = fmt.Errorf("got %q, want %q", resp, payload)
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errors {
		if err != nil {
			t.Errorf("request %d: %v", i, err)
		}
	}
}

func TestMux_ClosedMux(t *testing.T) {
	server, client := net.Pipe()
	server.Close()
	m := NewMux(client)
	// Wait for readLoop to detect close
	time.Sleep(50 * time.Millisecond)

	_, err := m.RoundTrip([]byte("hello"))
	if err == nil {
		t.Error("expected error on closed mux")
	}
}
