// Package goport opens public GoPort tunnels to local TCP ports.
package goport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	neturl "net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/yamux"
)

const (
	// DefaultServer is the public GoPort TCP tunnel endpoint.
	DefaultServer = "goport.uz:7000"

	defaultDialTimeout  = 10 * time.Second
	defaultReconnectMin = time.Second
	defaultReconnectMax = 30 * time.Second
)

// Tunnel is an active public tunnel to a local port.
type Tunnel interface {
	URL() string
	Close() error
}

// Option configures a Client returned by New.
type Option func(*Client)

// WithServer overrides the default GoPort TCP tunnel server address.
func WithServer(addr string) Option {
	return func(c *Client) {
		c.serverAddr = strings.TrimSpace(addr)
	}
}

// Client is a small builder for opening GoPort tunnels.
type Client struct {
	token        string
	serverAddr   string
	subdomain    string
	reset        bool
	dialTimeout  time.Duration
	reconnectMin time.Duration
	reconnectMax time.Duration
}

// New returns a tunnel client builder authenticated with token.
func New(token string, opts ...Option) *Client {
	c := &Client{
		token:        strings.TrimSpace(token),
		serverAddr:   DefaultServer,
		dialTimeout:  defaultDialTimeout,
		reconnectMin: defaultReconnectMin,
		reconnectMax: defaultReconnectMax,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

// WithServer overrides the default GoPort TCP tunnel server address.
func (c *Client) WithServer(addr string) *Client {
	c.serverAddr = strings.TrimSpace(addr)
	return c
}

// Subdomain requests a custom public subdomain for the tunnel.
func (c *Client) Subdomain(name string) *Client {
	c.subdomain = strings.TrimSpace(name)
	c.reset = false
	return c
}

// Reset requests a fresh random subdomain for the tunnel.
func (c *Client) Reset() *Client {
	c.subdomain = ""
	c.reset = true
	return c
}

// Open establishes a tunnel to localhost:port.
func (c *Client) Open(port int) (Tunnel, error) {
	cfg := c.config()
	if cfg.token == "" {
		return nil, errors.New("goport: token is required")
	}
	if cfg.serverAddr == "" {
		return nil, errors.New("goport: server address is required")
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("goport: invalid port %d", port)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t := &clientTunnel{
		cfg:       cfg,
		port:      port,
		localAddr: net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
		ctx:       ctx,
		cancel:    cancel,
	}
	if err := t.connect(true); err != nil {
		cancel()
		return nil, err
	}

	t.runWG.Add(1)
	go t.run()

	return t, nil
}

func (c *Client) config() clientConfig {
	return clientConfig{
		token:        c.token,
		serverAddr:   c.serverAddr,
		subdomain:    c.subdomain,
		reset:        c.reset,
		dialTimeout:  c.dialTimeout,
		reconnectMin: c.reconnectMin,
		reconnectMax: c.reconnectMax,
	}
}

type clientConfig struct {
	token        string
	serverAddr   string
	subdomain    string
	reset        bool
	dialTimeout  time.Duration
	reconnectMin time.Duration
	reconnectMax time.Duration
}

type registrationRequest struct {
	Type      string `json:"type"`
	Port      string `json:"port"`
	Subdomain string `json:"subdomain,omitempty"`
	Reset     bool   `json:"reset,omitempty"`
	Token     string `json:"token,omitempty"`
}

type registrationResponse struct {
	Subdomain string `json:"subdomain"`
	URL       string `json:"url"`
	Error     string `json:"error,omitempty"`
}

type clientTunnel struct {
	cfg       clientConfig
	port      int
	localAddr string

	ctx    context.Context
	cancel context.CancelFunc

	runWG      sync.WaitGroup
	handlersWG sync.WaitGroup
	closeOnce  sync.Once

	mu                sync.RWMutex
	conn              net.Conn
	session           *yamux.Session
	url               string
	assignedSubdomain string
}

func (t *clientTunnel) URL() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.url
}

func (t *clientTunnel) Close() error {
	t.closeOnce.Do(func() {
		t.cancel()
		t.closeCurrent()
		t.runWG.Wait()
		t.handlersWG.Wait()
	})
	return nil
}

func (t *clientTunnel) run() {
	defer t.runWG.Done()

	backoff := t.cfg.reconnectMin
	if backoff <= 0 {
		backoff = defaultReconnectMin
	}

	for t.ctx.Err() == nil {
		_ = t.serve()
		if t.ctx.Err() != nil {
			return
		}
		t.closeCurrent()

		if !t.wait(backoff) {
			return
		}

		for t.ctx.Err() == nil {
			if err := t.connect(false); err == nil {
				backoff = t.cfg.reconnectMin
				if backoff <= 0 {
					backoff = defaultReconnectMin
				}
				break
			}
			if !t.wait(backoff) {
				return
			}
			backoff = nextBackoff(backoff, t.cfg.reconnectMax)
		}
	}
}

func (t *clientTunnel) serve() error {
	session := t.currentSession()
	if session == nil {
		return errors.New("goport: tunnel session is not connected")
	}

	for {
		stream, err := session.Accept()
		if err != nil {
			return err
		}

		t.handlersWG.Add(1)
		go func() {
			defer t.handlersWG.Done()
			t.handleStream(stream)
		}()
	}
}

func (t *clientTunnel) currentSession() *yamux.Session {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.session
}

func (t *clientTunnel) connect(initial bool) error {
	dialer := net.Dialer{
		Timeout:   t.cfg.dialTimeout,
		KeepAlive: 30 * time.Second,
	}

	conn, err := dialer.DialContext(t.ctx, "tcp", t.cfg.serverAddr)
	if err != nil {
		return fmt.Errorf("goport: connect to %s: %w", t.cfg.serverAddr, err)
	}

	if t.cfg.dialTimeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(t.cfg.dialTimeout))
	}

	req := registrationRequest{
		Type:  "http",
		Port:  strconv.Itoa(t.port),
		Token: t.cfg.token,
	}
	if initial {
		req.Subdomain = t.cfg.subdomain
		req.Reset = t.cfg.reset
	} else {
		req.Subdomain = t.reconnectSubdomain()
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		_ = conn.Close()
		return fmt.Errorf("goport: send handshake: %w", err)
	}

	var resp registrationResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		_ = conn.Close()
		return fmt.Errorf("goport: read handshake: %w", err)
	}
	if resp.Error != "" {
		_ = conn.Close()
		return fmt.Errorf("goport: %s", resp.Error)
	}

	publicURL := responseURL(resp)
	if publicURL == "" {
		_ = conn.Close()
		return errors.New("goport: server returned empty tunnel url")
	}

	_ = conn.SetDeadline(time.Time{})

	session, err := yamux.Client(conn, nil)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("goport: start yamux client: %w", err)
	}

	t.mu.Lock()
	if t.ctx.Err() != nil {
		t.mu.Unlock()
		_ = session.Close()
		_ = conn.Close()
		return t.ctx.Err()
	}
	t.conn = conn
	t.session = session
	t.url = publicURL
	if subdomain := responseSubdomain(resp); subdomain != "" {
		t.assignedSubdomain = subdomain
	}
	t.mu.Unlock()

	return nil
}

func (t *clientTunnel) reconnectSubdomain() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.assignedSubdomain != "" {
		return t.assignedSubdomain
	}
	return t.cfg.subdomain
}

func (t *clientTunnel) closeCurrent() {
	t.mu.Lock()
	session := t.session
	conn := t.conn
	t.session = nil
	t.conn = nil
	t.mu.Unlock()

	if session != nil {
		_ = session.Close()
	}
	if conn != nil {
		_ = conn.Close()
	}
}

func (t *clientTunnel) wait(d time.Duration) bool {
	if d <= 0 {
		d = defaultReconnectMin
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-t.ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (t *clientTunnel) handleStream(stream net.Conn) {
	defer stream.Close()

	dialer := net.Dialer{Timeout: t.cfg.dialTimeout}
	localConn, err := dialer.DialContext(t.ctx, "tcp", t.localAddr)
	if err != nil {
		writeBadGateway(stream, t.localAddr, err)
		return
	}
	defer localConn.Close()

	proxy(stream, localConn)
}

func proxy(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go copyAndClose(a, b, done)
	go copyAndClose(b, a, done)

	<-done
	_ = a.Close()
	_ = b.Close()
	<-done
}

func copyAndClose(dst, src net.Conn, done chan<- struct{}) {
	_, _ = io.Copy(dst, src)
	closeWrite(dst)
	done <- struct{}{}
}

func closeWrite(c net.Conn) {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
		return
	}
	_ = c.Close()
}

func writeBadGateway(w io.Writer, localAddr string, err error) {
	body := fmt.Sprintf("failed to connect to %s: %v\n", localAddr, err)
	_, _ = fmt.Fprintf(w, "HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
}

func responseURL(resp registrationResponse) string {
	if url := strings.TrimSpace(resp.URL); url != "" {
		return url
	}

	subdomain := strings.TrimSpace(resp.Subdomain)
	if strings.HasPrefix(subdomain, "https://") || strings.HasPrefix(subdomain, "http://") {
		return subdomain
	}
	return ""
}

func responseSubdomain(resp registrationResponse) string {
	subdomain := strings.TrimSpace(resp.Subdomain)
	if subdomain == "" {
		return subdomainFromURL(resp.URL)
	}
	if strings.HasPrefix(subdomain, "https://") || strings.HasPrefix(subdomain, "http://") {
		return subdomainFromURL(subdomain)
	}
	return subdomain
}

func subdomainFromURL(raw string) string {
	parsed, err := neturl.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	host := parsed.Hostname()
	if i := strings.IndexByte(host, '.'); i > 0 {
		return host[:i]
	}
	return host
}

func nextBackoff(current, max time.Duration) time.Duration {
	if current <= 0 {
		current = defaultReconnectMin
	}
	if max <= 0 {
		max = defaultReconnectMax
	}

	next := current * 2
	if next > max {
		return max
	}
	return next
}
