package goport

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
)

func TestOpenForwardsHTTPOverYamux(t *testing.T) {
	local := newLocalHTTPServer(t)
	fake := newFakeGoPortServer(t, "abc123", "https://abc123.goport.uz")

	tunnel, err := New("token", WithServer(fake.addr)).Open(local.port)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer tunnel.Close()

	if got := tunnel.URL(); got != "https://abc123.goport.uz" {
		t.Fatalf("URL() = %q, want %q", got, "https://abc123.goport.uz")
	}

	conn := fake.nextConn(t)
	if conn.req.Token != "token" {
		t.Fatalf("handshake token = %q, want token", conn.req.Token)
	}
	if conn.req.Port != strconv.Itoa(local.port) {
		t.Fatalf("handshake port = %q, want %d", conn.req.Port, local.port)
	}

	stream, err := conn.session.Open()
	if err != nil {
		t.Fatalf("session.Open() error = %v", err)
	}
	defer stream.Close()

	req, err := http.NewRequest(http.MethodGet, "http://abc123.goport.uz/ping", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Host = "abc123.goport.uz"

	if err := req.Write(stream); err != nil {
		t.Fatalf("request write error = %v", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(stream), req)
	if err != nil {
		t.Fatalf("ReadResponse() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if got := resp.Header.Get("X-Goport-Test"); got != "ok" {
		t.Fatalf("X-Goport-Test = %q, want ok", got)
	}
}

func TestReconnectUsesAssignedSubdomainWithoutReset(t *testing.T) {
	fake := newFakeGoPortServer(t, "abc123", "https://abc123.goport.uz")

	client := New("token", WithServer(fake.addr)).Reset()
	client.reconnectMin = 10 * time.Millisecond
	client.reconnectMax = 20 * time.Millisecond

	tunnel, err := client.Open(12345)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer tunnel.Close()

	first := fake.nextConn(t)
	if !first.req.Reset {
		t.Fatalf("first handshake reset = false, want true")
	}
	if first.req.Subdomain != "" {
		t.Fatalf("first handshake subdomain = %q, want empty", first.req.Subdomain)
	}

	_ = first.session.Close()
	_ = first.conn.Close()

	second := fake.nextConn(t)
	if second.req.Reset {
		t.Fatalf("reconnect reset = true, want false")
	}
	if second.req.Subdomain != "abc123" {
		t.Fatalf("reconnect subdomain = %q, want abc123", second.req.Subdomain)
	}
}

func TestOpenRejectsInvalidInput(t *testing.T) {
	if _, err := New("").Open(3000); err == nil {
		t.Fatal("Open() with empty token error = nil, want error")
	}
	if _, err := New("token").Open(0); err == nil {
		t.Fatal("Open() with invalid port error = nil, want error")
	}
	if _, err := New("token", WithServer("")).Open(3000); err == nil {
		t.Fatal("Open() with empty server error = nil, want error")
	}
}

type localHTTPServer struct {
	port int
}

func newLocalHTTPServer(t *testing.T) *localHTTPServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("local listen error = %v", err)
	}

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Host != "abc123.goport.uz" {
				t.Errorf("local request host = %q, want abc123.goport.uz", r.Host)
			}
			w.Header().Set("X-Goport-Test", "ok")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("created"))
		}),
	}

	go func() {
		_ = srv.Serve(ln)
	}()
	t.Cleanup(func() {
		_ = srv.Close()
	})

	return &localHTTPServer{
		port: ln.Addr().(*net.TCPAddr).Port,
	}
}

type fakeGoPortServer struct {
	addr   string
	ln     net.Listener
	events chan fakeConn
}

type fakeConn struct {
	req     registrationRequest
	conn    net.Conn
	session *yamux.Session
}

func newFakeGoPortServer(t *testing.T, subdomain, publicURL string) *fakeGoPortServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("fake listen error = %v", err)
	}

	f := &fakeGoPortServer{
		addr:   ln.Addr().String(),
		ln:     ln,
		events: make(chan fakeConn, 8),
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}

			var req registrationRequest
			if err := json.NewDecoder(conn).Decode(&req); err != nil {
				_ = conn.Close()
				continue
			}

			resp := registrationResponse{
				Subdomain: subdomain,
				URL:       publicURL,
			}
			if err := json.NewEncoder(conn).Encode(resp); err != nil {
				_ = conn.Close()
				continue
			}

			session, err := yamux.Server(conn, nil)
			if err != nil {
				_ = conn.Close()
				continue
			}

			f.events <- fakeConn{req: req, conn: conn, session: session}
		}
	}()

	t.Cleanup(func() {
		_ = ln.Close()
	})

	return f
}

func (f *fakeGoPortServer) nextConn(t *testing.T) fakeConn {
	t.Helper()

	select {
	case conn := <-f.events:
		return conn
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for fake GoPort connection")
		return fakeConn{}
	}
}
